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
	}
	return web.Render(c, http.StatusOK, checkinPage(w, notice))
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
