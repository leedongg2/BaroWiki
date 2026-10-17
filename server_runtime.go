package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"opennamu/route/tool"

	"github.com/gin-gonic/gin"
)

func request_capacity_limit() int {
	limit, err := strconv.Atoi(os.Getenv("NAMU_MAX_INFLIGHT"))
	if err != nil || limit < 1 || limit > 4096 {
		return 128
	}
	return limit
}

// DB 연결과 렌더 작업이 무한히 늘어나지 않도록 대기열 없이 과부하를 알린다.
func request_capacity(limit int) gin.HandlerFunc {
	if limit < 1 {
		limit = 128
	}
	slots := make(chan struct{}, limit)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) &&
			(path == "/healthz" || path == "/readyz" || strings.HasPrefix(path, "/view/") || strings.HasPrefix(path, "/views/")) {
			c.Next()
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			c.Next()
		default:
			c.Header("Retry-After", "2")
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "server_busy", "message": "접속이 많아 처리하지 못했습니다. 잠시 후 다시 시도해 주세요."})
		}
	}
}

func request_protection() gin.HandlerFunc {
	protection := http.NewCrossOriginProtection()
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		// 계정별 화면과 API가 공용 CDN 캐시에 남지 않게 한다.
		c.Header("Cache-Control", "private, no-store")
		if err := protection.Check(c.Request); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross_origin_request", "message": "사이트에서 페이지를 다시 열고 시도해 주세요."})
			return
		}
		c.Next()
	}
}

func register_health_routes(r *gin.Engine) {
	live := func(c *gin.Context) { c.Status(http.StatusOK) }
	ready := func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		var count int
		err := tool.DB_connect().QueryRowContext(ctx, "select count(*) from other where name = 'ver'").Scan(&count)
		if err != nil || count == 0 {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		c.Status(http.StatusOK)
	}
	r.GET("/healthz", live)
	r.HEAD("/healthz", live)
	r.GET("/readyz", ready)
	r.HEAD("/readyz", ready)
}

func serve_wiki(r *gin.Engine, port string) error {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		log.Print("서버를 종료합니다. 처리 중인 요청을 기다립니다.")
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("서버 종료 대기 초과: %v", err)
			_ = server.Close()
		}
		err := <-result
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
