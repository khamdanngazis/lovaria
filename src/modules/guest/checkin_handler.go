package guest

import (
	"net/http"

	"github.com/labstack/echo/v4"

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
	}
	link, _, err := h.svc.ActiveCheckinLink(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, checkinPage(w, notice, link))
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
