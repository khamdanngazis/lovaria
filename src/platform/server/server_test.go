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
	"github.com/khamdanngazis/lovaria/src/platform/web"
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

func TestCSRF(t *testing.T) {
	e := New(testConfig(t), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	e.GET("/form", func(c echo.Context) error { return c.String(http.StatusOK, web.CSRFToken(c.Request().Context())) })
	e.POST("/form", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	post := func(mod func(*http.Request)) int {
		req := httptest.NewRequest(http.MethodPost, "/form", strings.NewReader(""))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
		mod(req)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	// Ambil token + cookie lewat GET.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/form", nil))
	token := rec.Body.String()
	cookies := rec.Result().Cookies()
	if token == "" || len(cookies) == 0 {
		t.Fatalf("token/cookie tidak di-set: %q %v", token, cookies)
	}
	withCookie := func(r *http.Request) {
		for _, ck := range cookies {
			r.AddCookie(ck)
		}
	}

	if code := post(func(*http.Request) {}); code != http.StatusForbidden {
		t.Errorf("tanpa token: %d, want 403", code)
	}
	if code := post(func(r *http.Request) { withCookie(r); r.Header.Set(CSRFHeader, "salah") }); code != http.StatusForbidden {
		t.Errorf("token salah: %d, want 403", code)
	}
	if code := post(func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }); code != http.StatusForbidden {
		t.Errorf("cross-site: %d, want 403", code)
	}
	if code := post(func(r *http.Request) { withCookie(r); r.Header.Set(CSRFHeader, token) }); code != http.StatusNoContent {
		t.Errorf("token header benar: %d, want 204", code)
	}
	if code := post(func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin") }); code != http.StatusNoContent {
		t.Errorf("same-origin browser: %d, want 204", code)
	}
}

func TestGlobalBodyLimit(t *testing.T) {
	e := New(testConfig(t), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	e.POST("/x", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("a", 13<<20)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("body 13 MB: %d, want 413", rec.Code)
	}
}

func TestGzipTextResponses(t *testing.T) {
	e := New(testConfig(t), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	e.GET("/page", func(c echo.Context) error { return c.HTML(http.StatusOK, strings.Repeat("<p>undangan</p>", 200)) })
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Header().Get("Content-Encoding") != "gzip" || rec.Body.Len() >= 3000 {
		t.Errorf("tidak terkompresi: %q, %d byte", rec.Header().Get("Content-Encoding"), rec.Body.Len())
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

func TestPublicFormPath(t *testing.T) {
	for p, want := range map[string]bool{
		"/i/ABCDEFG/rsvp":                 true,
		"/i/ABCDEFG/guestbook":            true,
		"/w/budi-sari/guestbook":          true,
		"/guestbook":                      true,
		"/w/budi-sari/rsvp":               false, // RSVP hanya lewat kode tamu
		"/i//rsvp":                        false,
		"/i/ABCDEFG/rsvp/x":               false,
		"/dashboard/weddings/x/guestbook": false,
		"/auth/login":                     false,
		"/x/guestbook":                    false,
	} {
		if got := PublicFormPath(p); got != want {
			t.Errorf("PublicFormPath(%q) = %v, want %v", p, got, want)
		}
	}
}
