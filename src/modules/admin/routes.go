package admin

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Deps struct{}

// Register memasang route panel admin. Group wajib sudah dilindungi
// RequireAuth + RequireRole(admin) di cmd/server/main.go.
func Register(g *echo.Group, _ Deps) {
	g.GET("", func(c echo.Context) error {
		return web.Render(c, http.StatusOK, homePage())
	})
}
