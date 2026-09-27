package wedding

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
	// ArchiveDays: lama Kenangan sebelum diarsipkan (config LIFECYCLE_ARCHIVE_DAYS).
	ArchiveDays int
	// Home: widget beranda wedding dari paket dashboard (T13); boleh nil.
	Home HomeWidgets
}

// Register memasang route dashboard wedding pada group /dashboard/weddings
// (group wajib sudah dilindungi RequireAuth) dan mengembalikan group per wedding
// (/:weddingID, dilindungi RequireWeddingOwner) tempat sub-modul memasang route-nya.
func Register(g *echo.Group, deps Deps) *echo.Group {
	h := NewHandler(deps.Service)
	h.home = deps.Home
	if deps.ArchiveDays > 0 {
		h.archiveDays = deps.ArchiveDays
	}

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
	w.PATCH("/status", h.UpdateStatus)
	return w
}

// OwnerGroup mengembalikan group /:weddingID yang dilindungi RequireWeddingOwner.
// Modul dashboard lain (events, guests, gallery, ...) memasang route di sini.
func OwnerGroup(g *echo.Group, svc *Service) *echo.Group {
	return g.Group("/:weddingID", svc.RequireWeddingOwner)
}
