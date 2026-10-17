package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRequestCapacityRecoversAndKeepsHealthAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(error_handler(), request_capacity(1))
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	router.GET("/slow", func(c *gin.Context) { close(entered); <-release; c.Status(204) })
	router.GET("/next", func(c *gin.Context) { c.Status(204) })
	router.GET("/panic", func(c *gin.Context) { panic("test capacity release") })
	router.GET("/healthz", func(c *gin.Context) { c.Status(200) })
	router.GET("/views/main.css", func(c *gin.Context) { c.Status(200) })
	go func() {
		defer close(finished)
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/slow", nil))
	}()
	<-entered
	for _, test := range []struct {
		path string
		code int
	}{{"/next", 503}, {"/healthz", 200}, {"/views/main.css", 200}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
		if response.Code != test.code {
			t.Errorf("%s: %d", test.path, response.Code)
		}
		if test.code == 503 && response.Header().Get("Retry-After") == "" {
			t.Error("overload response has no retry advice")
		}
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("request failed to finish")
	}
	for _, test := range []struct {
		path string
		code int
	}{{"/next", 204}, {"/panic", 500}, {"/next", 204}} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", test.path, nil))
		if response.Code != test.code {
			t.Errorf("capacity leaked after completion/panic: %s %d", test.path, response.Code)
		}
	}
}

func TestRequestCapacityConfiguration(t *testing.T) {
	for _, test := range []struct {
		value string
		want  int
	}{{"", 128}, {"0", 128}, {"-1", 128}, {"no", 128}, {"4097", 128}, {"16", 16}, {"256", 256}} {
		t.Setenv("NAMU_MAX_INFLIGHT", test.value)
		if got := request_capacity_limit(); got != test.want {
			t.Errorf("%q: %d", test.value, got)
		}
	}
}

func TestRequestProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(request_protection())
	router.Any("/edit", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, test := range []struct {
		method, site, origin string
		status               int
	}{
		{"POST", "same-origin", "https://wiki.example", 204},
		{"POST", "cross-site", "https://attacker.example", 403},
		{"POST", "same-site", "https://other.example", 403},
		{"DELETE", "", "https://attacker.example", 403},
		{"POST", "", "https://wiki.example", 204},
		{"POST", "", "null", 403},
		{"POST", "", "", 204},
		{"GET", "cross-site", "https://attacker.example", 204},
	} {
		request := httptest.NewRequest(test.method, "https://wiki.example/edit", nil)
		request.Header.Set("Sec-Fetch-Site", test.site)
		request.Header.Set("Origin", test.origin)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.status {
			t.Errorf("%+v: status %d", test, response.Code)
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("private response may be cached")
		}
	}
}
