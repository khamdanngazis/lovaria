package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/config"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestMiddlewareRequestIDAndLogging(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	e := New(testConfig(t), log)
	e.GET("/ping", func(c echo.Context) error { return c.String(http.StatusOK, "pong") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	rid := rec.Header().Get(echo.HeaderXRequestID)
	if rid == "" {
		t.Fatal("X-Request-ID tidak di-set")
	}
	if !strings.Contains(buf.String(), rid) || !strings.Contains(buf.String(), `"uri":"/ping"`) {
		t.Errorf("log tidak memuat request: %s", buf.String())
	}
}

func TestRecoverFromPanic(t *testing.T) {
	var buf bytes.Buffer
	e := New(testConfig(t), slog.New(slog.NewJSONHandler(&buf, nil)))
	e.GET("/boom", func(c echo.Context) error { panic("boom") })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Errorf("panic tidak di-log: %s", buf.String())
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	cfg := testConfig(t)
	cfg.Port = port
	cfg.ShutdownTimeout = 2 * time.Second
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	e := New(cfg, log)
	e.GET("/ok", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, e, cfg, log) }()

	url := fmt.Sprintf("http://127.0.0.1:%d/ok", port)
	var resp *http.Response
	for i := 0; i < 50; i++ {
		if resp, err = http.Get(url); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("server tidak bisa dihubungi: %v", err)
	}
	_ = resp.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server tidak berhenti")
	}
}
