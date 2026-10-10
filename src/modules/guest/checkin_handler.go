package guest

import (
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Check-in tamu dengan QR di hari H (T31) — halaman pengaturan di dashboard.

// GET /dashboard/weddings/:weddingID/checkin
func (h *Handler) CheckinPage(c echo.Context) error {
	w := ctxWedding(c)
	notice := ""
	switch c.QueryParam("ok") {
	case "on":
		notice = "Check-in QR diaktifkan. QR kehadiran tampil di undangan pribadi tiap tamu."
	case "off":
		notice = "Check-in QR dimatikan. QR tidak lagi tampil di undangan."
	case "link":
		notice = "Link penerima tamu dibuat. Link sebelumnya (bila ada) tidak berlaku lagi."
	case "revoked":
		notice = "Link penerima tamu dicabut."
	case "marked":
		notice = "Tamu ditandai datang."
	case "undone":
		notice = "Check-in dibatalkan."
	case "walkin":
		notice = "Catatan tamu tambahan dihapus."
	}
	ctx := c.Request().Context()
	link, _, err := h.svc.ActiveCheckinLink(ctx, w.ID)
	if err != nil {
		return err
	}
	att, err := h.attendance(c, w)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, checkinPage(w, notice, link, att))
}

// attendanceState: ringkasan kehadiran untuk pasangan.
type attendanceState struct {
	W       wedding.Wedding
	Stats   CheckinStats
	Recent  []Guest
	Walkins []Walkin
	Loc     *time.Location
}

func (h *Handler) attendance(c echo.Context, w wedding.Wedding) (attendanceState, error) {
	ctx := c.Request().Context()
	a := attendanceState{W: w}
	var err error
	if a.Stats, a.Recent, err = h.svc.CheckinSummary(ctx, w.ID, 15); err != nil {
		return a, err
	}
	if a.Walkins, err = h.svc.ListWalkins(ctx, w.ID, 15); err != nil {
		return a, err
	}
	if a.Loc, err = time.LoadLocation(w.Timezone); err != nil {
		a.Loc = time.FixedZone("WIB", 7*3600)
	}
	return a, nil
}

// GET /dashboard/weddings/:weddingID/checkin/attendance — fragmen ringkasan
// kehadiran (dipanggil berkala selama check-in dibuka).
func (h *Handler) Attendance(c echo.Context) error {
	att, err := h.attendance(c, ctxWedding(c))
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, attendanceCard(att))
}

// GET /dashboard/weddings/:weddingID/checkin/search?q= — cari tamu untuk
// ditandai datang secara manual oleh pasangan.
func (h *Handler) OwnerCheckinSearch(c echo.Context) error {
	w := ctxWedding(c)
	q := c.QueryParam("q")
	gs, err := h.svc.SearchForCheckin(c.Request().Context(), w.ID, q)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return web.Render(c, http.StatusOK, ownerSearchResults(w, q, gs, loc))
}

// POST /dashboard/weddings/:weddingID/checkin/mark (guest_id, undo=1?) —
// pasangan menandai / membatalkan kedatangan tamu. Butuh fitur aktif; boleh
// juga setelah hari H untuk merapikan catatan.
func (h *Handler) OwnerCheckinMark(c echo.Context) error {
	w := ctxWedding(c)
	ctx := c.Request().Context()
	if !w.CheckinEnabled {
		return echo.NewHTTPError(http.StatusConflict, "aktifkan check-in QR lebih dulu")
	}
	id, err := uuid.Parse(c.FormValue("guest_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	ok := "marked"
	if c.FormValue("undo") == "1" {
		ok = "undone"
		err = h.svc.UndoCheckIn(ctx, w.ID, id)
	} else {
		var g Guest
		if g, err = h.svc.Get(ctx, w.ID, id); err == nil {
			_, err = h.svc.CheckIn(ctx, w.ID, id, g.SuggestedPax(), ViaOwner)
		}
		if errors.Is(err, ErrAlreadyCheckedIn) {
			err = nil
		}
	}
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err != nil {
		return err
	}
	return web.Redirect(c, w.DashboardURL("/checkin")+"?ok="+ok)
}

// POST /dashboard/weddings/:weddingID/checkin/walkins/:id/delete
func (h *Handler) DeleteWalkin(c echo.Context) error {
	w := ctxWedding(c)
	id, err := uuid.Parse(c.Param("id"))
	if err == nil {
		err = h.svc.DeleteWalkin(c.Request().Context(), w.ID, id)
	}
	if errors.Is(err, ErrNotFound) || (err != nil && id == uuid.Nil) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	if err != nil {
		return err
	}
	return web.Redirect(c, w.DashboardURL("/checkin")+"?ok=walkin")
}

// POST /dashboard/weddings/:weddingID/checkin/link (action=new|revoke) — link
// halaman pemindai untuk penerima tamu.
func (h *Handler) SaveCheckinLink(c echo.Context) error {
	w := ctxWedding(c)
	ctx := c.Request().Context()
	if c.FormValue("action") == "revoke" {
		if err := h.svc.RevokeCheckinLink(ctx, w.ID); err != nil {
			return err
		}
		return web.Redirect(c, w.DashboardURL("/checkin")+"?ok=revoked")
	}
	if !w.CheckinEnabled {
		return echo.NewHTTPError(http.StatusConflict, "aktifkan check-in QR lebih dulu")
	}
	if _, err := h.svc.NewCheckinLink(ctx, w.ID); err != nil {
		return err
	}
	return web.Redirect(c, w.DashboardURL("/checkin")+"?ok=link")
}

// POST /dashboard/weddings/:weddingID/checkin (enabled=1|0)
func (h *Handler) SaveCheckin(c echo.Context) error {
	w := ctxWedding(c)
	enabled := c.FormValue("enabled") == "1"
	if enabled && !w.CheckinAvailable() {
		return echo.NewHTTPError(http.StatusForbidden)
	}
	if err := h.weddings.SetCheckinEnabled(c.Request().Context(), w.ID, enabled); err != nil {
		return err
	}
	ok := "off"
	if enabled {
		ok = "on"
	}
	return web.Redirect(c, w.DashboardURL("/checkin")+"?ok="+ok)
}
