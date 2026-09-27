// Package publicsite berisi routing & rendering website wedding publik
// (mobile-first). Resolusi wedding HANYA lewat Resolver.ResolveWedding.
package publicsite

import "github.com/labstack/echo/v4"

type Deps struct {
	Resolver *Resolver
	Handler  *Handler
	// RSVPLimit batas kiriman RSVP per kode tamu; nol → default.
	RSVPLimit RSVPLimit
	// GuestbookLimit batas kiriman buku ucapan per IP; nol → default.
	GuestbookLimit GuestbookLimit
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
	// Form publik: tanpa CSRF cookie (lihat server.PublicFormPath), dilindungi token HMAC + rate limit.
	e.POST("/i/:code/rsvp", h.RSVP, rw, h.rsvpLimiter(d.RSVPLimit))

	// Buku ucapan (T11): POST dilindungi token HMAC + honeypot + rate limit per IP.
	gbLimit := h.guestbookLimiter(d.GuestbookLimit)
	for _, p := range []string{"/i/:code/guestbook", "/w/:slug/guestbook", "/guestbook"} {
		e.GET(p, h.GuestbookMore, rw)
		e.POST(p, h.GuestbookPost, rw, gbLimit)
	}
}
