package auth

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Deps struct {
	Service    *Service
	Middleware *Middleware
	// RateLimit mengatur batas percobaan per IP; nol → default.
	RateLimit RateLimit
}

// RateLimit: token bucket per IP (in-memory, cukup untuk single instance).
type RateLimit struct {
	PerMinute float64
	Burst     int
}

var defaultRateLimit = RateLimit{PerMinute: 10, Burst: 10}

// Register memasang route halaman & aksi auth.
func Register(e *echo.Echo, deps Deps) {
	h := NewHandler(deps.Service, deps.Middleware)
	rl := deps.RateLimit
	if rl.PerMinute == 0 {
		rl = defaultRateLimit
	}
	// Store terpisah per aksi supaya percobaan login tidak menghabiskan kuota register.
	limitLogin, limitRegister, limitReset := limiter(rl), limiter(rl), limiter(rl)

	e.GET("/login", h.LoginPage)
	e.POST("/login", h.Login, limitLogin)
	e.POST("/logout", h.Logout)

	e.GET("/register", h.RegisterPage)
	e.POST("/register", h.Register, limitRegister)
	e.POST("/register/validate", h.ValidateRegisterField)

	e.GET("/forgot-password", h.ForgotPage)
	e.POST("/forgot-password", h.Forgot, limitReset)
	e.GET("/reset-password", h.ResetPage)
	e.POST("/reset-password", h.Reset, limitReset)
}

func limiter(rl RateLimit) echo.MiddlewareFunc {
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate:      rate.Limit(rl.PerMinute / 60),
			Burst:     rl.Burst,
			ExpiresIn: 10 * time.Minute,
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) { return c.RealIP(), nil },
		DenyHandler: func(c echo.Context, _ string, _ error) error {
			c.Response().Header().Set("Retry-After", "60")
			return web.Render(c, http.StatusTooManyRequests, tooManyRequests())
		},
	})
}
