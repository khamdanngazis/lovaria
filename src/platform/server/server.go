// Package server menyiapkan instance Echo beserta middleware global dan
// menjalankannya dengan graceful shutdown.
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// MaxBodySize adalah batas ukuran body request global.
const MaxBodySize = "12M"

const (
	// CSRFHeader dan CSRFField adalah tempat token CSRF dikirim (htmx: header, form biasa: field).
	CSRFHeader = "X-CSRF-Token"
	CSRFField  = "_csrf"
)

// New membuat Echo dengan middleware standar: request ID, recover, request log (slog JSON).
func New(cfg config.Config, log *slog.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Debug = cfg.IsDevelopment()
	e.Server.ReadHeaderTimeout = 10 * time.Second
	e.Server.ReadTimeout = 30 * time.Second
	e.Server.WriteTimeout = 60 * time.Second
	e.Server.IdleTimeout = 120 * time.Second

	// Batas body global dipasang paling awal: MethodOverride & CSRF membaca form
	// sebelum middleware per route, jadi tanpa ini body raksasa sempat dibaca
	// (dan bisa tumpah ke file temp di disk). Upload foto memakai batas 11 MB sendiri.
	e.Pre(middleware.BodyLimit(MaxBodySize))
	e.Pre(middleware.RemoveTrailingSlashWithConfig(middleware.TrailingSlashConfig{
		RedirectCode: http.StatusMovedPermanently,
	}))
	// Form HTML hanya bisa GET/POST: field _method=PATCH|PUT|DELETE pada POST
	// menjadikannya method tersebut (htmx memakai hx-patch langsung).
	e.Pre(middleware.MethodOverrideWithConfig(middleware.MethodOverrideConfig{
		Getter: middleware.MethodFromForm("_method"),
	}))
	e.Use(middleware.RequestID())
	e.Use(requestLogger(log))
	e.Use(middleware.RecoverWithConfig(middleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			log.ErrorContext(c.Request().Context(), "panic recovered",
				slog.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				slog.String("error", err.Error()),
				slog.String("stack", string(stack)),
			)
			return err
		},
	}))
	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:      "0",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "SAMEORIGIN",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}))
	e.Use(csrf(cfg))
	// Kompres respons teks (HTML/CSS/JS/JSON). Foto sudah terkompresi → dilewati.
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level:     5,
		MinLength: 1024,
		Skipper: func(c echo.Context) bool {
			return strings.HasPrefix(c.Request().URL.Path, "/media/")
		},
	}))

	return e
}

// csrf melindungi semua request non-GET. Browser modern lolos lewat header
// Sec-Fetch-Site (same-origin); selain itu wajib token (double-submit cookie)
// di header X-CSRF-Token atau field form _csrf. Gagal → 403.
func csrf(cfg config.Config) echo.MiddlewareFunc {
	protect := middleware.CSRFWithConfig(middleware.CSRFConfig{
		Skipper:        skipInfra,
		TokenLookup:    "header:" + CSRFHeader + ",form:" + CSRFField,
		CookieName:     "lovoria_csrf",
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSecure:   cfg.CookieSecure(),
		CookieSameSite: http.SameSiteLaxMode,
		ErrorHandler: func(_ error, _ echo.Context) error {
			return echo.NewHTTPError(http.StatusForbidden, "CSRF token tidak valid")
		},
	})
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return protect(func(c echo.Context) error {
			if tok, ok := c.Get("csrf").(string); ok {
				r := c.Request()
				c.SetRequest(r.WithContext(web.WithCSRFToken(r.Context(), tok)))
			}
			return next(c)
		})
	}
}

func skipInfra(c echo.Context) bool {
	p := c.Request().URL.Path
	return p == "/healthz" || p == "/readyz" || strings.HasPrefix(p, "/static/")
}

func requestLogger(log *slog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		Skipper: func(c echo.Context) bool {
			p := c.Request().URL.Path
			return p == "/healthz" || strings.HasPrefix(p, "/static/")
		},
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogLatency:   true,
		LogRequestID: true,
		LogHost:      true,
		LogRemoteIP:  true,
		LogError:     true,
		HandleError:  true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []slog.Attr{
				slog.String("request_id", v.RequestID),
				slog.String("method", v.Method),
				slog.String("host", v.Host),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.Duration("latency", v.Latency),
				slog.String("remote_ip", v.RemoteIP),
			}
			level := slog.LevelInfo
			if v.Error != nil {
				attrs = append(attrs, slog.String("error", v.Error.Error()))
			}
			if v.Status >= http.StatusInternalServerError {
				level = slog.LevelError
			}
			log.LogAttrs(c.Request().Context(), level, "request", attrs...)
			return nil
		},
	})
}

// Run menjalankan server sampai ctx dibatalkan (mis. SIGTERM dari Railway),
// lalu melakukan graceful shutdown dengan batas waktu cfg.ShutdownTimeout.
func Run(ctx context.Context, e *echo.Echo, cfg config.Config, log *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("server starting", slog.String("addr", cfg.Addr()))
		if err := e.Start(cfg.Addr()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	log.Info("server shutting down", slog.Duration("timeout", cfg.ShutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("server stopped")
	return <-errCh
}
