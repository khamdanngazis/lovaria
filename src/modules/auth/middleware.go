package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// SessionCookie adalah nama cookie session.
const SessionCookie = "lovoria_session"

// CurrentUser mengembalikan user yang sedang login (diisi oleh LoadSession).
func CurrentUser(ctx context.Context) (web.User, bool) {
	return web.CurrentUser(ctx)
}

// Middleware berisi middleware auth yang dipasang di cmd/server/main.go.
type Middleware struct {
	svc          *Service
	cookieSecure bool
	log          *slog.Logger
}

func NewMiddleware(svc *Service, cookieSecure bool, log *slog.Logger) *Middleware {
	return &Middleware{svc: svc, cookieSecure: cookieSecure, log: log}
}

// LoadSession membaca cookie session (bila ada) dan menaruh user ke context.
// Tidak pernah memblokir request — gunakan RequireAuth untuk itu.
func (m *Middleware) LoadSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		p := c.Request().URL.Path
		if strings.HasPrefix(p, "/static/") || p == "/healthz" || p == "/readyz" {
			return next(c)
		}
		ck, err := c.Cookie(SessionCookie)
		if err != nil || ck.Value == "" {
			return next(c)
		}

		u, extended, err := m.svc.SessionUser(c.Request().Context(), ck.Value)
		switch {
		case errors.Is(err, ErrSessionInvalid):
			m.clearCookie(c)
			return next(c)
		case err != nil:
			// DB bermasalah: perlakukan sebagai anonim, jangan hapus cookie.
			m.log.ErrorContext(c.Request().Context(), "auth: load session", slog.String("error", err.Error()))
			return next(c)
		}
		if extended != nil {
			m.setCookie(c, ck.Value, *extended)
		}
		r := c.Request()
		c.SetRequest(r.WithContext(web.WithUser(r.Context(), u.Web())))
		return next(c)
	}
}

// RequireAuth mengarahkan user anonim ke /login?next=<path>.
func (m *Middleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if _, ok := CurrentUser(c.Request().Context()); ok {
			return next(c)
		}
		target := "/login?next=" + url.QueryEscape(c.Request().URL.RequestURI())
		return web.Redirect(c, target)
	}
}

// RequireRole menolak (403) user yang login tapi role-nya tidak sesuai.
// Pasang setelah RequireAuth.
func (m *Middleware) RequireRole(role string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u, ok := CurrentUser(c.Request().Context())
			if !ok {
				return m.RequireAuth(next)(c)
			}
			if u.Role != role {
				return echo.NewHTTPError(http.StatusForbidden)
			}
			return next(c)
		}
	}
}

func (m *Middleware) setCookie(c echo.Context, token string, expires time.Time) {
	// Secure mengikuti config: false hanya di development (http://localhost).
	c.SetCookie(&http.Cookie{ //nolint:gosec // G124: Secure sengaja dari config
		Name:     SessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Middleware) clearCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // G124: Secure sengaja dari config
		Name:     SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// safeNext hanya menerima path relatif lokal (mencegah open redirect).
func safeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		return "/dashboard"
	}
	return next
}
