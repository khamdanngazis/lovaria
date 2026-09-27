package domain

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc *Service
}

func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("domain: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func (h *Handler) state(c echo.Context, w wedding.Wedding) (pageState, error) {
	s := pageState{W: w, Enabled: h.svc.Enabled(), Target: h.svc.CNAMETarget()}
	d, err := h.svc.Get(c.Request().Context(), w.ID)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return s, err
	default:
		s.Domain = &d
	}
	return s, nil
}

func (h *Handler) render(c echo.Context, status int, s pageState) error {
	if web.IsHTMX(c) {
		return web.Render(c, status, section(s))
	}
	return web.Render(c, status, page(s))
}

// GET .../domain
func (h *Handler) Page(c echo.Context) error {
	w := ctxWedding(c)
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	return h.render(c, http.StatusOK, s)
}

// after merender ulang setelah aksi; err pengguna (validasi/Cloudflare) → 422 dengan pesan.
func (h *Handler) after(c echo.Context, w wedding.Wedding, input, notice string, err error) error {
	var v ValidationError
	userErr := errors.As(err, &v) || errors.Is(err, ErrProvider) || errors.Is(err, ErrDisabled) || errors.Is(err, ErrNotFound)
	if err != nil && !userErr {
		return err
	}
	if err == nil && !web.IsHTMX(c) {
		return c.Redirect(http.StatusSeeOther, w.DashboardURL("/domain"))
	}
	s, serr := h.state(c, w)
	if serr != nil {
		return serr
	}
	s.Input, s.Notice = input, notice
	if err != nil {
		s.Notice = ""
		if v != nil {
			s.Error = v["domain"]
		} else {
			s.Error = err.Error()
		}
		return h.render(c, http.StatusUnprocessableEntity, s)
	}
	return h.render(c, http.StatusOK, s)
}

// POST .../domain (domain=…)
func (h *Handler) Add(c echo.Context) error {
	w := ctxWedding(c)
	input := c.FormValue("domain")
	_, err := h.svc.Add(c.Request().Context(), w.ID, input)
	return h.after(c, w, input, "Domain didaftarkan. Sekarang pasang CNAME di penyedia domain Anda.", err)
}

// POST .../domain/check
func (h *Handler) Check(c echo.Context) error {
	w := ctxWedding(c)
	d, err := h.svc.Recheck(c.Request().Context(), w.ID)
	notice := "Status diperbarui: " + StatusLabel(d.Status) + "."
	return h.after(c, w, "", notice, err)
}

// DELETE .../domain
func (h *Handler) Remove(c echo.Context) error {
	w := ctxWedding(c)
	err := h.svc.Remove(c.Request().Context(), w.ID)
	return h.after(c, w, "", "Domain dihapus. Undangan kembali memakai alamat Lovoria.", err)
}
