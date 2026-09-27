// Package publicsite berisi routing & rendering website wedding publik
// (mobile-first). Resolusi wedding HANYA lewat Resolver.ResolveWedding.
package publicsite

import "github.com/labstack/echo/v4"

type Deps struct {
	Resolver *Resolver
	Handler  *Handler
}

func Register(e *echo.Echo, d Deps) {
	rw := d.Resolver.ResolveWedding
	h := d.Handler

	// Domain utama: "/" = landing. Custom domain (T15): "/" = undangan.
	e.GET("/", h.Home, rw)
	e.GET("/events/:file", h.Calendar, rw) // custom domain

	e.GET("/w/:slug", h.Invitation, rw)
	e.GET("/w/:slug/events/:file", h.Calendar, rw)
	e.GET("/i/:code", h.Invitation, rw)
	e.GET("/i/:code/events/:file", h.Calendar, rw)
}
