package publicsite

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/platform/web"
	"github.com/khamdanngazis/lovaria/src/templates/shared"
)

const guestbookNotice = "Terima kasih! Ucapan Anda terkirim."

// guestbookEntries mengubah entri service ke view (tanggal di zona waktu wedding).
func guestbookEntries(es []guestbook.Entry, tz string) []view.GuestbookEntry {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	out := make([]view.GuestbookEntry, len(es))
	for i, e := range es {
		out[i] = view.GuestbookEntry{Name: e.Name, Message: e.Message, DateText: web.FormatDateID(e.CreatedAt.In(loc))}
	}
	return out
}

// setGuestbookForm mengisi action, token, dan nama tamu (bila lewat /i/:code).
func (h *Handler) setGuestbookForm(v *view.View, res Resolved) {
	v.Guestbook.Action = res.Prefix + "/guestbook"
	v.Guestbook.Token = h.formToken("guestbook", res.Wedding.ID.String(), h.clock())
	if res.Guest != nil && v.Guestbook.Name == "" {
		v.Guestbook.Name = res.Guest.Name
	}
}

// guestbookView: data section buku ucapan untuk fragment htmx.
func (h *Handler) guestbookView(ctx context.Context, res Resolved) (view.View, error) {
	v := view.View{AllowGuestbook: res.Wedding.AllowsGuestbook()}
	es, more, err := h.Guestbook.Visible(ctx, res.Wedding.ID, uuid.Nil, guestbook.PublicPage)
	if err != nil {
		return v, err
	}
	v.Guestbook.Entries = guestbookEntries(es, res.Wedding.Timezone)
	if more {
		v.Guestbook.MoreBefore = es[len(es)-1].ID.String()
	}
	h.setGuestbookForm(&v, res)
	return v, nil
}

func (h *Handler) renderGuestbook(c echo.Context, status int, v view.View) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	return web.Render(c, status, shared.GuestbookSection(v))
}

// GuestbookPost: POST {prefix}/guestbook — payload {name, message, token, website}.
func (h *Handler) GuestbookPost(c echo.Context) error {
	ctx := c.Request().Context()
	res, _ := FromContext(ctx)
	if res.Preview {
		return notFound(c)
	}
	v, err := h.guestbookView(ctx, res)
	if err != nil {
		return err
	}
	if !res.Wedding.AllowsGuestbook() {
		v.AllowGuestbook = true
		v.Guestbook.Error = "Maaf, buku ucapan sudah ditutup."
		return h.renderGuestbook(c, http.StatusForbidden, v)
	}
	name, message := c.FormValue("name"), c.FormValue("message")
	success := func() error {
		if !web.IsHTMX(c) {
			return c.Redirect(http.StatusSeeOther, res.Prefix+"?guestbook=ok#guestbook")
		}
		v.Guestbook.Notice = guestbookNotice
		return h.renderGuestbook(c, http.StatusOK, v)
	}
	// Honeypot terisi → bot: balas seolah berhasil, tanpa menyimpan.
	if c.FormValue("website") != "" {
		return success()
	}
	if !h.validFormToken("guestbook", res.Wedding.ID.String(), c.FormValue("token"), h.clock()) {
		v.Guestbook.Name, v.Guestbook.Message = name, message
		v.Guestbook.Error = "Halaman sudah terlalu lama dibuka. Silakan kirim ulang."
		return h.renderGuestbook(c, http.StatusForbidden, v)
	}
	var guestID *uuid.UUID
	if res.Guest != nil {
		guestID = &res.Guest.ID
	}
	_, err = h.Guestbook.Post(ctx, res.Wedding.ID, guestID, name, message)
	var ve guestbook.ValidationError
	if errors.As(err, &ve) {
		v.Guestbook.Errors, v.Guestbook.Name, v.Guestbook.Message = ve, name, message
		return h.renderGuestbook(c, http.StatusUnprocessableEntity, v)
	}
	if err != nil {
		return err
	}
	if v, err = h.guestbookView(ctx, res); err != nil { // daftar terbaru memuat pesan baru
		return err
	}
	return success()
}

// GuestbookMore: GET {prefix}/guestbook?before=<id> — potongan pesan berikutnya.
func (h *Handler) GuestbookMore(c echo.Context) error {
	ctx := c.Request().Context()
	res, _ := FromContext(ctx)
	if !res.Wedding.AllowsGuestbook() {
		return notFound(c)
	}
	before, err := uuid.Parse(c.QueryParam("before"))
	if err != nil {
		return notFound(c)
	}
	es, more, err := h.Guestbook.Visible(ctx, res.Wedding.ID, before, guestbook.PublicPage)
	if err != nil {
		return err
	}
	v := view.View{AllowGuestbook: true}
	v.Guestbook.Entries = guestbookEntries(es, res.Wedding.Timezone)
	v.Guestbook.Action = res.Prefix + "/guestbook"
	if more {
		v.Guestbook.MoreBefore = es[len(es)-1].ID.String()
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=30")
	c.Response().Header().Set("X-Robots-Tag", "noindex")
	if !web.IsHTMX(c) {
		// Tanpa JS: halaman sederhana berisi potongan berikutnya.
		return web.Render(c, http.StatusOK, guestbookMorePage(v, res.Prefix))
	}
	return web.Render(c, http.StatusOK, shared.GuestbookEntries(v))
}

// GuestbookLimit: batas kiriman buku ucapan per IP (in-memory, single instance).
type GuestbookLimit struct {
	PerMinute float64
	Burst     int
}

var defaultGuestbookLimit = GuestbookLimit{PerMinute: 5, Burst: 5}

func (h *Handler) guestbookLimiter(l GuestbookLimit) echo.MiddlewareFunc {
	if l.PerMinute <= 0 {
		l = defaultGuestbookLimit
	}
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate: rate.Limit(l.PerMinute / 60), Burst: l.Burst, ExpiresIn: 10 * time.Minute,
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) { return c.RealIP(), nil },
		DenyHandler: func(c echo.Context, _ string, _ error) error {
			res, _ := FromContext(c.Request().Context())
			v, err := h.guestbookView(c.Request().Context(), res)
			if err != nil {
				return err
			}
			v.Guestbook.Name, v.Guestbook.Message = c.FormValue("name"), c.FormValue("message")
			v.Guestbook.Error = "Terlalu banyak ucapan dari perangkat ini. Coba lagi sebentar lagi."
			return h.renderGuestbook(c, http.StatusTooManyRequests, v)
		},
	})
}
