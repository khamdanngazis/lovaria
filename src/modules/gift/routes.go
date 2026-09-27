package gift

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
}

// Register memasang route kelola hadiah di group /dashboard/weddings/:weddingID
// (sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := &Handler{svc: deps.Service}
	w.GET("/gifts", h.List)
	w.POST("/gifts", h.Create)
	w.GET("/gifts/new", h.New)
	w.GET("/gifts/:accountID/edit", h.Edit)
	w.PATCH("/gifts/:accountID", h.Update)
	w.DELETE("/gifts/:accountID", h.Delete)
	w.PATCH("/gifts/:accountID/position", h.Move)
}
