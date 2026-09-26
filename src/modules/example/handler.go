package example

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListNotes: GET /weddings/:weddingID/notes → fragment daftar catatan.
func (h *Handler) ListNotes(c echo.Context) error {
	weddingID, err := weddingIDParam(c)
	if err != nil {
		return err
	}
	notes, err := h.svc.ListNotes(c.Request().Context(), weddingID)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, noteList(notes))
}

// CreateNote: POST /weddings/:weddingID/notes (form: body) → fragment satu catatan.
func (h *Handler) CreateNote(c echo.Context) error {
	weddingID, err := weddingIDParam(c)
	if err != nil {
		return err
	}
	note, err := h.svc.AddNote(c.Request().Context(), weddingID, c.FormValue("body"))
	switch {
	case errors.Is(err, ErrEmptyNote), errors.Is(err, ErrNoteTooLong):
		return web.Render(c, http.StatusUnprocessableEntity, noteError(err.Error()))
	case err != nil:
		return err
	}
	return web.Render(c, http.StatusCreated, noteItem(note))
}

func weddingIDParam(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("weddingID"), 10, 64)
	if err != nil || id <= 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound)
	}
	return id, nil
}
