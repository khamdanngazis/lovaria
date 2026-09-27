package domain

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
}

// Register memasang route custom domain di group /dashboard/weddings/:weddingID
// (sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := &Handler{svc: deps.Service}
	w.GET("/domain", h.Page)
	w.POST("/domain", h.Add)
	w.POST("/domain/check", h.Check)
	w.DELETE("/domain", h.Remove)
}
