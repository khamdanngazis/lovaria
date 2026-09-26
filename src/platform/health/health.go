// Package health menyediakan endpoint liveness (/healthz) dan readiness (/readyz).
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// Checker memeriksa satu dependency (mis. database). Diisi mulai T02.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// CheckerFunc mengubah fungsi biasa menjadi Checker.
type CheckerFunc struct {
	N  string
	Fn func(ctx context.Context) error
}

func (c CheckerFunc) Name() string                    { return c.N }
func (c CheckerFunc) Check(ctx context.Context) error { return c.Fn(ctx) }

type Handler struct {
	checkers []Checker
	timeout  time.Duration
}

func NewHandler(checkers ...Checker) *Handler {
	return &Handler{checkers: checkers, timeout: 3 * time.Second}
}

func (h *Handler) Register(e *echo.Echo) {
	e.GET("/healthz", h.Healthz)
	e.GET("/readyz", h.Readyz)
}

// Healthz hanya memastikan proses hidup dan bisa melayani request.
func (h *Handler) Healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz menjalankan semua checker; 503 bila ada yang gagal.
func (h *Handler) Readyz(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), h.timeout)
	defer cancel()

	status := http.StatusOK
	checks := make(map[string]string, len(h.checkers))
	for _, ch := range h.checkers {
		if err := ch.Check(ctx); err != nil {
			// Detail error hanya ke log; endpoint ini publik.
			slog.WarnContext(ctx, "readiness check failed", slog.String("check", ch.Name()), slog.String("error", err.Error()))
			status = http.StatusServiceUnavailable
			checks[ch.Name()] = "unavailable"
			continue
		}
		checks[ch.Name()] = "ok"
	}

	overall := "ok"
	if status != http.StatusOK {
		overall = "unavailable"
	}
	return c.JSON(status, map[string]any{"status": overall, "checks": checks})
}
