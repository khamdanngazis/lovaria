package story

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

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

var fields = []string{"year", "month", "day", "title", "description", "photo_url"}

func formFrom(c echo.Context) form {
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, k := range fields {
		f.Values[k] = c.FormValue(k)
	}
	return f
}

func formOf(s Story) form {
	f := form{Errors: map[string]string{}, Values: map[string]string{
		"year": strconv.Itoa(s.Date.Year), "title": s.Title, "description": s.Description, "photo_url": s.PhotoURL,
	}}
	if s.Date.Month != 0 {
		f.Values["month"] = strconv.Itoa(s.Date.Month)
	}
	if s.Date.Day != 0 {
		f.Values["day"] = strconv.Itoa(s.Date.Day)
	}
	return f
}

func (f form) input() Input {
	return Input{
		Year: f.v("year"), Month: f.v("month"), Day: f.v("day"),
		Title: f.v("title"), Description: f.v("description"), PhotoURL: f.v("photo_url"),
	}
}

// ctxWedding: wedding sudah diotorisasi RequireWeddingOwner.
func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("story: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func storyID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("storyID"))
	if err != nil {
		return uuid.Nil, echo.NewHTTPError(http.StatusNotFound)
	}
	return id, nil
}

func (h *Handler) state(c echo.Context, w wedding.Wedding) (pageState, error) {
	ss, err := h.svc.ListStories(c.Request().Context(), w.ID)
	return pageState{W: w, Stories: ss, Form: form{Values: map[string]string{}, Errors: map[string]string{}}}, err
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

// GET .../stories
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

// GET .../stories/new
func (h *Handler) New(c echo.Context) error {
	w := ctxWedding(c)
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
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

// POST .../stories
func (h *Handler) Create(c echo.Context) error {
	w := ctxWedding(c)
	f := formFrom(c)
	_, err := h.svc.CreateStory(c.Request().Context(), w.ID, f.input())
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

// GET .../stories/:storyID/edit
func (h *Handler) Edit(c echo.Context) error {
	w := ctxWedding(c)
	id, err := storyID(c)
	if err != nil {
		return err
	}
	e, err := h.svc.GetStory(c.Request().Context(), w.ID, id)
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

// PATCH .../stories/:storyID
func (h *Handler) Update(c echo.Context) error {
	w := ctxWedding(c)
	id, err := storyID(c)
	if err != nil {
		return err
	}
	f := formFrom(c)
	_, err = h.svc.UpdateStory(c.Request().Context(), w.ID, id, f.input())
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

// DELETE .../stories/:storyID
func (h *Handler) Delete(c echo.Context) error {
	w := ctxWedding(c)
	id, err := storyID(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteStory(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// PATCH .../stories/:storyID/position (direction=up|down)
func (h *Handler) Move(c echo.Context) error {
	w := ctxWedding(c)
	id, err := storyID(c)
	if err != nil {
		return err
	}
	dir := c.FormValue("direction")
	if dir != "up" && dir != "down" {
		return echo.NewHTTPError(http.StatusBadRequest, "direction harus up atau down")
	}
	if err := h.svc.MoveStory(c.Request().Context(), w.ID, id, dir == "up"); err != nil {
		return notFound(err)
	}
	return h.done(c, w)
}

// PATCH .../stories/order (by=date)
func (h *Handler) Sort(c echo.Context) error {
	w := ctxWedding(c)
	if c.FormValue("by") != "date" {
		return echo.NewHTTPError(http.StatusBadRequest, "by harus date")
	}
	if err := h.svc.SortStoriesByDate(c.Request().Context(), w.ID); err != nil {
		return err
	}
	return h.done(c, w)
}
