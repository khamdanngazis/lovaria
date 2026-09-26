package event

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

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

var fields = []string{"name", "type", "date", "start_time", "end_time", "venue", "address", "maps_url", "description"}

func formFrom(c echo.Context) form {
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, k := range fields {
		f.Values[k] = c.FormValue(k)
	}
	return f
}

func formOf(e Event) form {
	return form{Errors: map[string]string{}, Values: map[string]string{
		"name": e.Name, "type": e.Type, "date": e.Date.Format(dateLayout),
		"start_time": e.StartTime, "end_time": e.EndTime, "venue": e.Venue,
		"address": e.Address, "maps_url": e.MapsURL, "description": e.Description,
	}}
}

func (f form) input() Input {
	return Input{
		Name: f.v("name"), Type: f.v("type"), Date: f.v("date"), StartTime: f.v("start_time"),
		EndTime: f.v("end_time"), Venue: f.v("venue"), Address: f.v("address"),
		MapsURL: f.v("maps_url"), Description: f.v("description"),
	}
}

// ctxWedding: wedding sudah diotorisasi RequireWeddingOwner.
func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("event: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func eventID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("eventID"))
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusNotFound)
	}
	return id, nil
}

func (h *Handler) state(c echo.Context, w wedding.Wedding) (pageState, error) {
	evs, err := h.svc.ListEvents(c.Request().Context(), w.ID)
	return pageState{W: w, Events: evs, Form: form{Values: map[string]string{}, Errors: map[string]string{}}}, err
}

// done merespons perubahan sukses: htmx → section terbaru, tanpa JS → redirect ke daftar.
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

// invalid merender ulang form dengan pesan error (422).
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

func notFound(err error) error {
	if errors.Is(err, ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return err
}

// GET .../events
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

// GET .../events/new
func (h *Handler) New(c echo.Context) error {
	w := ctxWedding(c)
	f := form{Values: map[string]string{"type": TypeReception, "date": w.WeddingDate.Format(dateLayout)}, Errors: map[string]string{}}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, editor(w, uuid.Nil, f))
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	s.Creating, s.Form = true, f
	return web.Render(c, http.StatusOK, page(s))
}

// POST .../events
func (h *Handler) Create(c echo.Context) error {
	w := ctxWedding(c)
	f := formFrom(c)
	_, err := h.svc.CreateEvent(c.Request().Context(), w.ID, f.input())
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

// GET .../events/:eventID/edit
func (h *Handler) Edit(c echo.Context) error {
	w := ctxWedding(c)
	id, err := eventID(c)
	if err != nil {
		return err
	}
	e, err := h.svc.GetEvent(c.Request().Context(), w.ID, id)
	if err != nil {
		return notFound(err)
	}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, editorItem(w, e.ID, formOf(e)))
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	s.EditingID, s.Form = e.ID, formOf(e)
	return web.Render(c, http.StatusOK, page(s))
}

// PATCH .../events/:eventID
func (h *Handler) Update(c echo.Context) error {
	w := ctxWedding(c)
	id, err := eventID(c)
	if err != nil {
		return err
	}
	f := formFrom(c)
	_, err = h.svc.UpdateEvent(c.Request().Context(), w.ID, id, f.input())
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

// DELETE .../events/:eventID
func (h *Handler) Delete(c echo.Context) error {
	w := ctxWedding(c)
	id, err := eventID(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteEvent(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// PATCH .../events/:eventID/position (direction=up|down)
func (h *Handler) Move(c echo.Context) error {
	w := ctxWedding(c)
	id, err := eventID(c)
	if err != nil {
		return err
	}
	dir := c.FormValue("direction")
	if dir != "up" && dir != "down" {
		return echo.NewHTTPError(http.StatusBadRequest, "direction harus up atau down")
	}
	if err := h.svc.MoveEvent(c.Request().Context(), w.ID, id, dir == "up"); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// PATCH .../events/order (by=date)
func (h *Handler) Sort(c echo.Context) error {
	w := ctxWedding(c)
	if c.FormValue("by") != "date" {
		return echo.NewHTTPError(http.StatusBadRequest, "by harus date")
	}
	if err := h.svc.SortEventsByDate(c.Request().Context(), w.ID); err != nil {
		return err
	}
	return h.done(c, w)
}
