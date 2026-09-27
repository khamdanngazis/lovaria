// Package dashboard berisi UI couple. Group /dashboard dilindungi RequireAuth di cmd/server/main.go.
package dashboard

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// WeddingLister adalah bagian service wedding yang dibutuhkan beranda.
type WeddingLister interface {
	ListWeddingsByOwner(ctx context.Context, ownerID uuid.UUID) ([]wedding.Wedding, error)
}

type Deps struct {
	Weddings WeddingLister
}

func Register(g *echo.Group, deps Deps) {
	// GET /dashboard: belum punya wedding → wizard (admin → panel admin); satu →
	// langsung ke wedding itu; lebih dari satu → daftar.
	g.GET("", func(c echo.Context) error {
		u, ok := web.CurrentUser(c.Request().Context())
		if !ok {
			return echo.NewHTTPError(http.StatusUnauthorized)
		}
		ws, err := deps.Weddings.ListWeddingsByOwner(c.Request().Context(), u.ID)
		if err != nil {
			return err
		}
		switch len(ws) {
		case 0:
			if u.Role == auth.RoleAdmin {
				return web.Redirect(c, "/admin")
			}
			return web.Redirect(c, "/dashboard/weddings/new")
		case 1:
			return web.Redirect(c, "/dashboard/weddings/"+ws[0].ID.String())
		default:
			return web.Redirect(c, "/dashboard/weddings")
		}
	})
}
