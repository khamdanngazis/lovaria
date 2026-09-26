package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func do(t *testing.T, h *Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	h.Register(e)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestHealthz(t *testing.T) {
	rec := do(t, NewHandler(), "/healthz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestReadyzOK(t *testing.T) {
	ok := CheckerFunc{N: "db", Fn: func(context.Context) error { return nil }}
	rec := do(t, NewHandler(ok), "/readyz")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
}

func TestReadyzFailing(t *testing.T) {
	bad := CheckerFunc{N: "db", Fn: func(context.Context) error { return errors.New("down") }}
	rec := do(t, NewHandler(bad), "/readyz")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"db":"unavailable"`) || strings.Contains(rec.Body.String(), "down") {
		t.Errorf("body = %s", rec.Body)
	}
}
