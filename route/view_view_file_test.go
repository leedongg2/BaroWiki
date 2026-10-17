package route

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStaticAssetDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/views/*name", View_view_file)
	router.HEAD("/views/*name", View_view_file)
	fetch := func(method, path string, headers map[string]string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, nil)
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	path := "/views/main_css/css/main.css.cache_v295"
	plain := fetch("GET", path, nil)
	if plain.Code != 200 || !strings.Contains(plain.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("plain: %d %v", plain.Code, plain.Header())
	}
	zipped := fetch("GET", path, map[string]string{"Accept-Encoding": "gzip"})
	reader, err := gzip.NewReader(bytes.NewReader(zipped.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(decoded, plain.Body.Bytes()) {
		t.Fatal("compressed content changed")
	}
	if zipped.Body.Len() >= plain.Body.Len() || zipped.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatal("compression failed")
	}
	if zipped.Header().Get("ETag") == plain.Header().Get("ETag") {
		t.Fatal("encodings need distinct validators")
	}
	cached := fetch("GET", path, map[string]string{"If-None-Match": plain.Header().Get("ETag")})
	if cached.Code != 304 || cached.Body.Len() != 0 {
		t.Fatal("conditional GET failed")
	}
	head := fetch("HEAD", path, nil)
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("ETag") != plain.Header().Get("ETag") {
		t.Fatal("HEAD failed")
	}
	ranged := fetch("GET", path, map[string]string{"Range": "bytes=0-9", "Accept-Encoding": "gzip"})
	if ranged.Code != 206 || !bytes.Equal(ranged.Body.Bytes(), plain.Body.Bytes()[:10]) {
		t.Fatal("range failed")
	}
	for _, path := range []string{"/views/missing.css", "/views/views.go", "/views/../../go.mod"} {
		if got := fetch("GET", path, nil); got.Code != 404 {
			t.Errorf("%s: status %d", path, got.Code)
		}
	}
	for _, header := range []string{"gzip;q=0", "*;q=1, gzip;q=0", "br", "gzip;q=invalid"} {
		if asset_accepts_gzip(header) {
			t.Errorf("gzip must be rejected: %s", header)
		}
	}
}
