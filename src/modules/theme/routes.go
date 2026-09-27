package theme

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

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
	w.POST("/theme/music", h.UploadMusic, middleware.BodyLimit("9M")) // MP3 ≤ 8 MB + overhead multipart
	w.DELETE("/theme/music", h.DeleteMusic)
}
