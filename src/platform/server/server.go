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

	e.Pre(middleware.RemoveTrailingSlashWithConfig(middleware.TrailingSlashConfig{
		RedirectCode: http.StatusMovedPermanently,
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

	return e
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
