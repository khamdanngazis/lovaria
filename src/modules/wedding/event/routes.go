package event

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
}

// Register memasang route acara di group /dashboard/weddings/:weddingID
// (wedding.OwnerGroup — sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := NewHandler(deps.Service)
	w.GET("/events", h.List)
	w.POST("/events", h.Create)
	w.GET("/events/new", h.New)
	w.PATCH("/events/order", h.Sort)
	w.GET("/events/:eventID/edit", h.Edit)
	w.PATCH("/events/:eventID", h.Update)
	w.DELETE("/events/:eventID", h.Delete)
	w.PATCH("/events/:eventID/position", h.Move)
}
