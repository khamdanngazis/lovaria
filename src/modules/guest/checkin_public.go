package guest

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Halaman pemindai penerima tamu (T31): dibuka lewat link bertoken tanpa akun.
// Semua aksi dibatasi ke wedding pemilik token, dan hanya menampilkan nama,
// grup, dan jumlah orang — tidak nomor HP, email, atau catatan tamu.

const checkinWeddingKey = "checkin_wedding"

// RegisterCheckin memasang route /checkin/:token. mws: middleware halaman
// publik dari pemanggil (CSP ketat, izin kamera, no-store).
func RegisterCheckin(e *echo.Echo, deps Deps, mws ...echo.MiddlewareFunc) {
	h := NewHandler(deps.Service, deps.Weddings)
	limit := middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate: rate.Limit(4), Burst: 60, ExpiresIn: 10 * time.Minute, // pintu masuk ramai: longgar, tetap menahan tebak-tebakan
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) { return c.RealIP(), nil },
	})
	g := e.Group("/checkin/:token", append(mws, limit, h.checkinWedding)...)
	g.GET("", h.ScannerPage)
	g.GET("/search", h.ScannerSearch)
	g.POST("/scan", h.ScannerScan)
	g.POST("/confirm", h.ScannerConfirm)
	g.POST("/undo", h.ScannerUndo)
	g.POST("/walkin", h.ScannerWalkin)
}

// checkinWedding me-resolve token → wedding. Token salah / dicabut → 404 polos
// (tidak membocorkan apakah wedding itu ada).
func (h *Handler) checkinWedding(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		id, err := h.svc.WeddingByCheckinToken(ctx, c.Param("token"))
		if errors.Is(err, ErrCheckinLink) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if err != nil {
			return err
		}
		w, err := h.weddings.GetWedding(ctx, id)
		if errors.Is(err, wedding.ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if err != nil {
			return err
		}
		c.Set(checkinWeddingKey, w)
		hdr := c.Response().Header()
		hdr.Set("Cache-Control", "no-store")
		hdr.Set("X-Robots-Tag", "noindex")
		hdr.Set("Referrer-Policy", "no-referrer") // token ada di URL
		return next(c)
	}
}

// scanner: keadaan satu permintaan halaman pemindai.
type scanner struct {
	W     wedding.Wedding
	Base  string // "/checkin/<token>"
	Stats CheckinStats
	Loc   *time.Location
}

func (h *Handler) scanner(c echo.Context) (scanner, error) {
	w := c.Get(checkinWeddingKey).(wedding.Wedding)
	st, _, err := h.svc.CheckinSummary(c.Request().Context(), w.ID, 0)
	if err != nil {
		return scanner{}, err
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return scanner{W: w, Base: "/checkin/" + c.Param("token"), Stats: st, Loc: loc}, nil
}

// open: check-in sedang dibuka; bila tidak, balas fragmen "ditutup".
func (h *Handler) scannerOpen(c echo.Context, s scanner) (bool, error) {
	if s.W.CheckinOpen() {
		return true, nil
	}
	return false, web.Render(c, http.StatusOK, scanResult(s, resultClosed, Guest{}, ""))
}

// GET /checkin/:token
func (h *Handler) ScannerPage(c echo.Context) error {
	s, err := h.scanner(c)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, scannerPage(s))
}

// POST /checkin/:token/scan (code = isi QR atau kode; via = scan|manual)
func (h *Handler) ScannerScan(c echo.Context) error {
	s, err := h.scanner(c)
	if err != nil {
		return err
	}
	if ok, err := h.scannerOpen(c, s); !ok {
		return err
	}
	g, err := h.svc.GuestForCheckin(c.Request().Context(), s.W.ID, c.FormValue("code"))
	if errors.Is(err, ErrNotFound) {
		return web.Render(c, http.StatusOK, scanResult(s, resultUnknown, Guest{}, ""))
	}
	if err != nil {
		return err
	}
	via := ViaScan
	if c.FormValue("via") == ViaManual {
		via = ViaManual
	}
	if g.CheckedInAt != nil {
		return web.Render(c, http.StatusOK, scanResult(s, resultAlready, g, via))
	}
	return web.Render(c, http.StatusOK, scanResult(s, resultReady, g, via))
}

// POST /checkin/:token/confirm (guest_id, pax, via)
func (h *Handler) ScannerConfirm(c echo.Context) error {
	s, err := h.scanner(c)
	if err != nil {
		return err
	}
	if ok, err := h.scannerOpen(c, s); !ok {
		return err
	}
	id, err := uuid.Parse(c.FormValue("guest_id"))
	if err != nil {
		return web.Render(c, http.StatusOK, scanResult(s, resultUnknown, Guest{}, ""))
	}
	pax, _ := strconv.Atoi(c.FormValue("pax"))
	g, err := h.svc.CheckIn(c.Request().Context(), s.W.ID, id, pax, c.FormValue("via"))
	kind := resultDone
	switch {
	case errors.Is(err, ErrAlreadyCheckedIn):
		kind = resultAlready
	case errors.Is(err, ErrNotFound):
		return web.Render(c, http.StatusOK, scanResult(s, resultUnknown, Guest{}, ""))
	case err != nil:
		return err
	}
	if s, err = h.scanner(c); err != nil { // statistik terbaru
		return err
	}
	return web.Render(c, http.StatusOK, scanResult(s, kind, g, ""))
}

// POST /checkin/:token/undo (guest_id)
func (h *Handler) ScannerUndo(c echo.Context) error {
	s, err := h.scanner(c)
	if err != nil {
		return err
	}
	if ok, err := h.scannerOpen(c, s); !ok {
		return err
	}
	if id, perr := uuid.Parse(c.FormValue("guest_id")); perr == nil {
		if err := h.svc.UndoCheckIn(c.Request().Context(), s.W.ID, id); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
	}
	if s, err = h.scanner(c); err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, scanNote(s, "Check-in dibatalkan.", ""))
}

// GET /checkin/:token/search?q=
func (h *Handler) ScannerSearch(c echo.Context) error {
	s, err := h.scanner(c)
	if err != nil {
		return err
	}
	q := c.QueryParam("q")
	gs, err := h.svc.SearchForCheckin(c.Request().Context(), s.W.ID, q)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, searchResults(s, q, gs))
}

// POST /checkin/:token/walkin (name, pax) — tamu tambahan tanpa undangan.
func (h *Handler) ScannerWalkin(c echo.Context) error {
	s, err := h.scanner(c)
	if err != nil {
		return err
	}
	if ok, err := h.scannerOpen(c, s); !ok {
		return err
	}
	pax, _ := strconv.Atoi(c.FormValue("pax"))
	wk, err := h.svc.AddWalkin(c.Request().Context(), s.W.ID, c.FormValue("name"), pax)
	var v ValidationError
	if errors.As(err, &v) {
		return web.Render(c, http.StatusOK, scanNote(s, "", v["name"]))
	}
	if err != nil {
		return err
	}
	if s, err = h.scanner(c); err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, scanNote(s, "Tamu tambahan dicatat: "+wk.Name+" · "+strconv.Itoa(wk.Pax)+" orang.", ""))
}
