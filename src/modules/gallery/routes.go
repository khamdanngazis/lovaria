package gallery

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Deps struct {
	Service  *Service
	Weddings WeddingReader
}

// Register memasang route gallery di group /dashboard/weddings/:weddingID
// (sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := NewHandler(deps.Service, deps.Weddings)
	w.GET("/gallery", h.List)
	// Satu foto (maks. 10 MB) per request + overhead multipart.
	w.POST("/gallery/items", h.Upload, middleware.BodyLimit("11M"))
	w.PATCH("/gallery/items/:itemID", h.Update)
	w.DELETE("/gallery/items/:itemID", h.Delete)
	w.PATCH("/gallery/items/:itemID/position", h.Move)
	w.POST("/gallery/items/:itemID/cover", h.Cover)
}
