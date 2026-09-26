// Package publicsite berisi routing & rendering website wedding publik (mobile-first).
// Resolusi wedding (Host header → fallback slug/kode) akan dipasang di sini
// sebagai satu middleware terpusat (T09).
package publicsite

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Deps struct{}

func Register(e *echo.Echo, _ Deps) {
	e.GET("/", func(c echo.Context) error {
		return web.Render(c, http.StatusOK, landingPage())
	})
}
