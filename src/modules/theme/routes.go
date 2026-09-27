package theme

import "github.com/labstack/echo/v4"

type Deps struct {
	Service   *Service
	Previewer Previewer
}

// Register memasang halaman tema di group /dashboard/weddings/:weddingID
// (sudah RequireAuth + RequireWeddingOwner).
func Register(w *echo.Group, deps Deps) {
	h := &Handler{svc: deps.Service, previews: deps.Previewer}
	w.GET("/theme", h.Page)
	w.PATCH("/theme", h.Save)
	w.GET("/theme/preview", h.Preview)
}
