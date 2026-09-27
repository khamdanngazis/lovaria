package guestbook

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/google/uuid"
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
		panic("guestbook: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

// filterOf: "" = semua, "shown" / "hidden".
func filterOf(s string) (string, *bool) {
	switch s {
	case "shown":
		f := false
		return s, &f
	case "hidden":
		t := true
		return s, &t
	}
	return "", nil
}

func (h *Handler) state(c echo.Context, w wedding.Wedding) (pageState, error) {
	ctx := c.Request().Context()
	s := pageState{W: w}
	var hidden *bool
	s.Filter, hidden = filterOf(c.FormValue("filter"))
	s.Page, _ = strconv.Atoi(c.FormValue("page"))
	s.Page = max(1, s.Page)
	var err error
	if s.Entries, err = h.svc.List(ctx, w.ID, hidden, s.Page); err != nil {
		return s, err
	}
	s.Stats, err = h.svc.Stats(ctx, w.ID)
	return s, err
}

// GET .../guestbook?filter=&page=
func (h *Handler) List(c echo.Context) error {
	w := ctxWedding(c)
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, section(s))
	}
	return web.Render(c, http.StatusOK, page(s))
}

func entryID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("entryID"))
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusNotFound)
	}
	return id, nil
}

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return err
}

// done: htmx → section terbaru (filter & halaman dari hx-include), tanpa JS → redirect.
func (h *Handler) done(c echo.Context, w wedding.Wedding) error {
	if !web.IsHTMX(c) {
		return c.Redirect(http.StatusSeeOther, base(w))
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, section(s))
}

// PATCH .../guestbook/:entryID (hidden=1|0)
func (h *Handler) SetHidden(c echo.Context) error {
	w := ctxWedding(c)
	id, err := entryID(c)
	if err != nil {
		return err
	}
	if _, err := h.svc.SetHidden(c.Request().Context(), w.ID, id, c.FormValue("hidden") == "1"); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// DELETE .../guestbook/:entryID
func (h *Handler) Delete(c echo.Context) error {
	w := ctxWedding(c)
	id, err := entryID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// PATCH .../guestbook/:entryID/favorite (favorite=1|0)
func (h *Handler) SetFavorite(c echo.Context) error {
	w := ctxWedding(c)
	id, err := entryID(c)
	if err != nil {
		return err
	}
	if _, err := h.svc.SetFavorite(c.Request().Context(), w.ID, id, c.FormValue("favorite") == "1"); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}
