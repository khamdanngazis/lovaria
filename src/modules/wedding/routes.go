package wedding

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
}

// Register memasang route dashboard wedding pada group /dashboard/weddings
// (group wajib sudah dilindungi RequireAuth). Route per wedding dilindungi
// RequireWeddingOwner; modul lain memasang route-nya di OwnerGroup.
func Register(g *echo.Group, deps Deps) {
	h := NewHandler(deps.Service)

	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/new", h.New)
	g.POST("/new/steps/:step", h.Step)

	w := OwnerGroup(g, deps.Service)
	w.GET("", h.Overview)
	w.GET("/info", h.InfoPage)
	w.PATCH("/info", h.UpdateInfo)
	w.GET("/couple", h.CouplePage)
	w.PATCH("/couple", h.UpdateCouple)
}

// OwnerGroup mengembalikan group /:weddingID yang dilindungi RequireWeddingOwner.
// Modul dashboard lain (events, guests, gallery, ...) memasang route di sini.
func OwnerGroup(g *echo.Group, svc *Service) *echo.Group {
	return g.Group("/:weddingID", svc.RequireWeddingOwner)
}
