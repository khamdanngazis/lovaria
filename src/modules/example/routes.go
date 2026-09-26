package example

import "github.com/labstack/echo/v4"

// Deps berisi dependency modul; disusun manual di cmd/server/main.go.
type Deps struct {
	Service *Service
}

// Register memasang route modul ke group yang diberikan.
func Register(g *echo.Group, deps Deps) {
	h := NewHandler(deps.Service)
	g.GET("/weddings/:weddingID/notes", h.ListNotes)
	g.POST("/weddings/:weddingID/notes", h.CreateNote)
}
