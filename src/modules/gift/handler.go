package gift

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc *Service
}

var fields = []string{"type", "provider", "account_number", "account_name", "address"}

func formFrom(c echo.Context) form {
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, k := range fields {
		f.Values[k] = c.FormValue(k)
	}
	return f
}

func formOf(a Account) form {
	return form{Errors: map[string]string{}, Values: map[string]string{
		"type": a.Type, "provider": a.Provider, "account_number": a.AccountNumber, "account_name": a.AccountName, "address": a.Address,
	}}
}

func newForm() form {
	return form{Values: map[string]string{"type": TypeBank}, Errors: map[string]string{}}
}

func (f form) input() Input {
	return Input{Type: f.v("type"), Provider: f.v("provider"), AccountNumber: f.v("account_number"), AccountName: f.v("account_name"), Address: f.v("address")}
}

func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("gift: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func accountID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("accountID"))
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

func (h *Handler) state(c echo.Context, w wedding.Wedding) (pageState, error) {
	as, err := h.svc.List(c.Request().Context(), w.ID)
	return pageState{W: w, Accounts: as, Form: newForm()}, err
}

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

func (h *Handler) invalid(c echo.Context, w wedding.Wedding, id uuid.UUID, f form) error {
	if web.IsHTMX(c) {
		web.Retarget(c, "#"+formID(id))
		return web.Render(c, http.StatusUnprocessableEntity, editor(w, id, f))
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	s.Form, s.Creating, s.EditingID = f, id == uuid.Nil, id
	return web.Render(c, http.StatusUnprocessableEntity, page(s))
}

// GET .../gifts
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

// GET .../gifts/new
func (h *Handler) New(c echo.Context) error {
	w := ctxWedding(c)
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, editor(w, uuid.Nil, newForm()))
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	s.Creating = true
	return web.Render(c, http.StatusOK, page(s))
}

// POST .../gifts
func (h *Handler) Create(c echo.Context) error {
	w := ctxWedding(c)
	f := formFrom(c)
	_, err := h.svc.Create(c.Request().Context(), w.ID, f.input())
	var v ValidationError
	if errors.As(err, &v) {
		f.Errors = v
		return h.invalid(c, w, uuid.Nil, f)
	}
	if err != nil {
		return err
	}
	return h.done(c, w)
}

// GET .../gifts/:accountID/edit
func (h *Handler) Edit(c echo.Context) error {
	w := ctxWedding(c)
	id, err := accountID(c)
	if err != nil {
		return err
	}
	a, err := h.svc.Get(c.Request().Context(), w.ID, id)
	if err != nil {
		return notFound(err)
	}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, editorItem(w, a.ID, formOf(a)))
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	s.EditingID, s.Form = a.ID, formOf(a)
	return web.Render(c, http.StatusOK, page(s))
}

// PATCH .../gifts/:accountID
func (h *Handler) Update(c echo.Context) error {
	w := ctxWedding(c)
	id, err := accountID(c)
	if err != nil {
		return err
	}
	f := formFrom(c)
	_, err = h.svc.Update(c.Request().Context(), w.ID, id, f.input())
	var v ValidationError
	if errors.As(err, &v) {
		f.Errors = v
		return h.invalid(c, w, id, f)
	}
	if err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// DELETE .../gifts/:accountID
func (h *Handler) Delete(c echo.Context) error {
	w := ctxWedding(c)
	id, err := accountID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// PATCH .../gifts/:accountID/position (direction=up|down)
func (h *Handler) Move(c echo.Context) error {
	w := ctxWedding(c)
	id, err := accountID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Move(c.Request().Context(), w.ID, id, c.FormValue("direction") == "up"); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}
