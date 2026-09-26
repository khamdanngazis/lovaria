package gallery

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// WeddingReader dipakai untuk memuat ulang wedding setelah foto utama berubah.
type WeddingReader interface {
	GetWedding(ctx context.Context, id uuid.UUID) (wedding.Wedding, error)
}

type Handler struct {
	svc      *Service
	weddings WeddingReader
}

func NewHandler(svc *Service, weddings WeddingReader) *Handler {
	return &Handler{svc: svc, weddings: weddings}
}

func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("gallery: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func itemID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("itemID"))
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

func validFilter(c string) string {
	if CategoryLabel(c) == "" {
		return ""
	}
	return c
}

func wantsJSON(c echo.Context) bool {
	return strings.Contains(c.Request().Header.Get(echo.HeaderAccept), echo.MIMEApplicationJSON)
}

func (h *Handler) state(c echo.Context, w wedding.Wedding, filter string) (pageState, error) {
	ctx := c.Request().Context()
	items, err := h.svc.ListGallery(ctx, w.ID)
	if err != nil {
		return pageState{}, err
	}
	usage, err := h.svc.StorageUsage(ctx, w.ID)
	if err != nil {
		return pageState{}, err
	}
	return pageState{W: w, Items: items, Usage: usage, Category: validFilter(filter)}, nil
}

// done merender ulang section (htmx) atau redirect ke halaman gallery (tanpa JS).
func (h *Handler) done(c echo.Context, w wedding.Wedding, filter string) error {
	s, err := h.state(c, w, filter)
	if err != nil {
		return err
	}
	if !web.IsHTMX(c) {
		return c.Redirect(http.StatusSeeOther, listURL(s))
	}
	return web.Render(c, http.StatusOK, section(s))
}

// GET .../gallery?category=&edit=
func (h *Handler) List(c echo.Context) error {
	w := ctxWedding(c)
	s, err := h.state(c, w, c.QueryParam("category"))
	if err != nil {
		return err
	}
	if id, err := uuid.Parse(c.QueryParam("edit")); err == nil {
		s.EditingID = id
	}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, section(s))
	}
	return web.Render(c, http.StatusOK, page(s))
}

type itemJSON struct {
	ID       uuid.UUID `json:"id"`
	URL      string    `json:"url"`
	ThumbURL string    `json:"thumb_url"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Category string    `json:"category"`
}

// POST .../gallery/items (multipart: file, category). Satu file per request.
func (h *Handler) Upload(c echo.Context) error {
	w := ctxWedding(c)
	fail := func(msg string) error {
		if wantsJSON(c) {
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": msg})
		}
		s, err := h.state(c, w, "")
		if err != nil {
			return err
		}
		s.Error = msg
		return web.Render(c, http.StatusUnprocessableEntity, page(s))
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return fail("Pilih file foto dulu")
	}
	f, err := fh.Open()
	if err != nil {
		return fail("File tidak bisa dibaca")
	}
	defer func() { _ = f.Close() }()

	it, err := h.svc.Upload(c.Request().Context(), w.ID, c.FormValue("category"), f)
	var ue *UploadError
	switch {
	case errors.As(err, &ue):
		return fail(ue.Msg)
	case errors.Is(err, ErrQuotaExceeded):
		return fail("Kuota penyimpanan wedding sudah penuh. Hapus beberapa foto dulu.")
	case err != nil:
		return err
	}
	if wantsJSON(c) {
		return c.JSON(http.StatusCreated, itemJSON{ID: it.ID, URL: it.URL, ThumbURL: it.ThumbURL, Width: it.Width, Height: it.Height, Category: it.Category})
	}
	return h.done(c, w, "")
}

// PATCH .../gallery/items/:itemID (caption, category, filter)
func (h *Handler) Update(c echo.Context) error {
	w := ctxWedding(c)
	id, err := itemID(c)
	if err != nil {
		return err
	}
	_, err = h.svc.UpdateItem(c.Request().Context(), w.ID, id, c.FormValue("caption"), c.FormValue("category"))
	var ue *UploadError
	if errors.As(err, &ue) {
		s, serr := h.state(c, w, c.FormValue("filter"))
		if serr != nil {
			return serr
		}
		s.Error, s.EditingID = ue.Msg, id
		return web.Render(c, http.StatusUnprocessableEntity, section(s))
	}
	if err != nil {
		return notFound(err)
	}
	return h.done(c, w, c.FormValue("filter"))
}

// DELETE .../gallery/items/:itemID
func (h *Handler) Delete(c echo.Context) error {
	w := ctxWedding(c)
	id, err := itemID(c)
	if err != nil {
		return err
	}
	if err := h.svc.DeleteItem(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return h.done(c, w, c.FormValue("category"))
}

// PATCH .../gallery/items/:itemID/position (direction=up|down)
func (h *Handler) Move(c echo.Context) error {
	w := ctxWedding(c)
	id, err := itemID(c)
	if err != nil {
		return err
	}
	dir := c.FormValue("direction")
	if dir != "up" && dir != "down" {
		return echo.NewHTTPError(http.StatusBadRequest, "direction harus up atau down")
	}
	if err := h.svc.MoveItem(c.Request().Context(), w.ID, id, dir == "up"); err != nil {
		return notFound(err)
	}
	return h.done(c, w, "")
}

// POST .../gallery/items/:itemID/cover → jadikan foto utama wedding.
func (h *Handler) Cover(c echo.Context) error {
	w := ctxWedding(c)
	id, err := itemID(c)
	if err != nil {
		return err
	}
	if err := h.svc.SetCover(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	updated, err := h.weddings.GetWedding(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	return h.done(c, updated, c.FormValue("category"))
}
