package tool

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func TestGetIPUsesConfiguredForwardedFor(t *testing.T) {
	previousSetting := load_ip_select
	load_ip_select = "HTTP_X_FORWARDED_FOR"
	defer func() { load_ip_select = previousSetting }()

	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	defer gin.SetMode(previousMode)

	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("01234567890123456789012345678901"))))
	var gotIP string
	router.GET("/", func(c *gin.Context) {
		gotIP = Get_IP(c)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "172.16.13.66:8080"
	request.Header.Set("X-Forwarded-For", "203.0.113.42, 172.16.13.66")
	router.ServeHTTP(httptest.NewRecorder(), request)

	if gotIP != "203.0.113.42" {
		t.Fatalf("Get_IP() = %q, want configured forwarded client IP %q", gotIP, "203.0.113.42")
	}
}

func TestGetIPUsesFlyClientIPByDefault(t *testing.T) {
	previousSetting := load_ip_select
	load_ip_select = "default"
	defer func() { load_ip_select = previousSetting }()

	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	defer gin.SetMode(previousMode)

	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("01234567890123456789012345678901"))))
	var gotIP string
	router.GET("/", func(c *gin.Context) {
		gotIP = Get_IP(c)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "172.16.13.66:8080"
	request.Header.Set("Fly-Client-IP", "203.0.113.42")
	router.ServeHTTP(httptest.NewRecorder(), request)

	if gotIP != "203.0.113.42" {
		t.Fatalf("Get_IP() = %q, want Fly-Client-IP %q", gotIP, "203.0.113.42")
	}
}