package tool

import (
	"crypto/sha256"
	"crypto/sha512"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/filesystem"
	"github.com/gin-gonic/gin"
)

const session_cookie_name = "opennamu_session"

// 로그인을 얼마나 오래 유지하는가 (30일).
//
// 0 으로 두면 브라우저를 닫는 순간 쿠키가 사라져 로그인이 풀린다.
const session_max_age = 60 * 60 * 24 * 30

// 세션 한 개의 최대 크기.
//
// 세션에는 사용자가 넣은 꾸미기 CSS(head) 가 통째로 들어갈 수 있다.
// 기본값(4KB)으로는 조금만 길어도 저장이 조용히 실패한다.
const session_max_length = 1 << 20

// 세션 파일을 두는 곳. data 폴더는 볼륨이라 서버를 다시 시작해도 남는다.
var session_dir = filepath.Join("data", "sessions")

func Session_middleware() gin.HandlerFunc {
	secret := os.Getenv("NAMU_SESSION_KEY")
	if secret == "" {
		db := DB_connect()
		QueryRow_DB(db, `select data from other where name = "session_key"`, []any{&secret})
		DB_close(db)
	}
	if secret == "" {
		secret = Get_random_key(128)
	}
	auth_key := sha512.Sum512([]byte(secret))
	encryption_key := sha256.Sum256([]byte("opennamu-session:" + secret))

	// 폴더를 못 만들면 빈 값을 넘긴다. 그때는 임시 폴더에 저장되어
	// 예전처럼 다시 시작할 때 로그인이 풀리지만, 위키는 그대로 돈다.
	path := session_dir
	if err := os.MkdirAll(path, 0o700); err != nil {
		path = ""
	}

	store := filesystem.NewStore(path, auth_key[:], encryption_key[:])
	if sized, ok := store.(interface{ MaxLength(int) }); ok {
		sized.MaxLength(session_max_length)
	}
	store.Options(session_options(false))

	if path != "" {
		go session_cleanup_loop(path)
	}

	return sessions.Sessions(session_cookie_name, store)
}

// session_cleanup_loop : 오래된 세션 파일을 주기적으로 치운다.
//
// 파일 저장소는 스스로 청소하지 않는다. 문서를 볼 때마다 "최근 본 문서" 가
// 세션에 저장되므로, 쿠키를 물고 오지 않는 크롤러는 요청마다 파일을 하나씩
// 남긴다. 그냥 두면 볼륨이 파일로 찬다.
func session_cleanup_loop(path string) {
	for {
		session_cleanup(path)
		time.Sleep(12 * time.Hour)
	}
}

func session_cleanup(path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	// 쿠키가 이미 만료된 세션은 아무도 다시 쓸 수 없다.
	limit := time.Now().Add(-time.Duration(session_max_age) * time.Second)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "session_") {
			continue
		}

		info, err := entry.Info()
		if err != nil || info.ModTime().After(limit) {
			continue
		}

		_ = os.Remove(filepath.Join(path, entry.Name()))
	}
}

func Get_session(c *gin.Context) sessions.Session {
	session := sessions.Default(c)
	session.Options(session_options(is_https(c)))

	return session
}

func session_options(secure bool) sessions.Options {
	return sessions.Options{
		Path:     "/",
		MaxAge:   session_max_age,
		Secure:   secure,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func is_https(c *gin.Context) bool {
	return c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
}
