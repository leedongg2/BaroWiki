package route

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"io/fs"
	"mime"
	"net/http"
	"opennamu/route/tool"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var asset_version_suffix = regexp.MustCompile(`\.cache_v[0-9]+$`)
var asset_zip_lock sync.Mutex
var asset_zip_cache = map[[32]byte][]byte{}
var asset_zip_size int

func asset_accepts_gzip(header string) bool {
	wildcard := false
	for _, entry := range strings.Split(header, ",") {
		parts := strings.Split(entry, ";")
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		quality := 1.0
		for _, param := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
			if ok && strings.EqualFold(key, "q") {
				quality, _ = strconv.ParseFloat(value, 64)
			}
		}
		if name == "gzip" {
			return quality > 0
		}
		if name == "*" {
			wildcard = quality > 0
		}
	}
	return wildcard
}

func asset_gzip(data []byte) []byte {
	hash := sha256.Sum256(data)
	asset_zip_lock.Lock()
	defer asset_zip_lock.Unlock()
	if cached, ok := asset_zip_cache[hash]; ok {
		return cached
	}
	var buffer bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&buffer, gzip.BestSpeed)
	_, _ = writer.Write(data)
	_ = writer.Close()
	result := buffer.Bytes()
	if len(result) <= 2<<20 {
		if asset_zip_size+len(result) > 16<<20 || len(asset_zip_cache) >= 256 {
			asset_zip_cache = map[[32]byte][]byte{}
			asset_zip_size = 0
		}
		asset_zip_cache[hash] = result
		asset_zip_size += len(result)
	}
	return result
}

func View_view_file(c *gin.Context) {
	raw_path := strings.TrimPrefix(c.Param("name"), "/")
	versioned := asset_version_suffix.MatchString(raw_path)
	name := tool.Clean_view_path(asset_version_suffix.ReplaceAllString(raw_path, ""))
	if name == "" {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := tool.Read_view_file(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			c.Status(http.StatusNotFound)
		} else {
			c.Status(http.StatusInternalServerError)
		}
		return
	}
	content_type := mime.TypeByExtension(path.Ext(name))
	if content_type == "" {
		content_type = http.DetectContentType(data)
	}
	c.Header("Content-Type", content_type)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=0, must-revalidate")
	if versioned {
		c.Header("Cache-Control", "public, max-age=86400")
	}
	compressible := strings.HasPrefix(content_type, "text/") || strings.Contains(content_type, "javascript") || strings.Contains(content_type, "json") || strings.Contains(content_type, "svg")
	if compressible && len(data) >= 1024 {
		c.Header("Vary", "Accept-Encoding")
		if c.GetHeader("Range") == "" && asset_accepts_gzip(c.GetHeader("Accept-Encoding")) {
			data = asset_gzip(data)
			c.Header("Content-Encoding", "gzip")
		}
	}
	c.Header("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(data)))
	// 조건부 요청, HEAD, 범위 다운로드는 표준 HTTP 구현으로 처리한다.
	http.ServeContent(c.Writer, c.Request, path.Base(name), time.Time{}, bytes.NewReader(data))
}
