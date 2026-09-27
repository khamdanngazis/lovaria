package guest

import (
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/labstack/echo/v4/middleware"
)

type Deps struct {
	Service  *Service
	Weddings *wedding.Service
}

// Register memasang route tamu di group /dashboard/weddings/:weddingID
// (sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := NewHandler(deps.Service, deps.Weddings)
	w.GET("/guests", h.List)
	w.GET("/rsvp", h.RSVP)
	w.GET("/share", h.SharePage)
	w.POST("/share/template", h.SaveShareTemplate)
	w.POST("/share/template/reset", h.ResetShareTemplate)
	w.POST("/guests/:guestID/shared", h.MarkShared)
	w.POST("/guests", h.Create)
	w.DELETE("/guests", h.BulkDelete)
	w.GET("/guests/new", h.New)
	w.GET("/guests/export", h.Export)
	w.GET("/guests/paste", h.PastePage)
	w.POST("/guests/paste", h.PasteReview, middleware.BodyLimit("3M"))
	w.POST("/guests/paste/confirm", h.PasteConfirm, middleware.BodyLimit("3M"))
	w.GET("/guests/import", h.ImportPage)
	w.GET("/guests/import/template", h.ImportTemplate)
	w.POST("/guests/import", h.ImportPreview, middleware.BodyLimit("3M"))
	w.POST("/guests/import/confirm", h.ImportConfirm, middleware.BodyLimit("3M"))
	w.GET("/guests/:guestID/edit", h.Edit)
	w.PATCH("/guests/:guestID", h.Update)
	w.DELETE("/guests/:guestID", h.Delete)
}
