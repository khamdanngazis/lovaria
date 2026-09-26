// Package dashboard berisi UI couple. Autentikasi dipasang di T03.
package dashboard

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Deps struct{}

func Register(g *echo.Group, _ Deps) {
	g.GET("", func(c echo.Context) error {
		return web.Render(c, http.StatusOK, homePage())
	})
}
