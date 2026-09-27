package guestbook

import "github.com/labstack/echo/v4"

type Deps struct {
	Service *Service
}

// Register memasang route kelola buku tamu di group /dashboard/weddings/:weddingID
// (sudah RequireAuth + RequireWeddingOwner). Route publik ada di public site.
func Register(w *echo.Group, deps Deps) {
	h := &Handler{svc: deps.Service}
	w.GET("/guestbook", h.List)
	w.PATCH("/guestbook/:entryID", h.SetHidden)
	w.DELETE("/guestbook/:entryID", h.Delete)
}
