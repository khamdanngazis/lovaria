package story

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
}

// Register memasang route love story di group /dashboard/weddings/:weddingID
// (wedding.OwnerGroup — sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := NewHandler(deps.Service)
	w.GET("/stories", h.List)
	w.POST("/stories", h.Create)
	w.GET("/stories/new", h.New)
	w.PATCH("/stories/order", h.Sort)
	w.GET("/stories/:storyID/edit", h.Edit)
	w.PATCH("/stories/:storyID", h.Update)
	w.DELETE("/stories/:storyID", h.Delete)
	w.PATCH("/stories/:storyID/position", h.Move)
}
