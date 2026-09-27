package theme

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Previewer menyusun data undangan untuk preview (diisi public-site ViewBuilder).
type Previewer interface {
	Preview(ctx context.Context, w wedding.Wedding) (view.View, error)
}

type Handler struct {
	svc      *Service
	previews Previewer
}

func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("theme: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func settingsFrom(get func(string) string) view.Settings {
	return view.Settings{
		PrimaryColor: get("primary_color"), FontHeading: get("font_heading"), FontBody: get("font_body"),
		Background: get("background"), CoverImage: get("cover_image"),
	}
}

// GET .../theme
func (h *Handler) Page(c echo.Context) error {
	w := ctxWedding(c)
	st, err := h.svc.Settings(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	s := pageState{W: w, ThemeID: w.ThemeID, Settings: st}
	if c.QueryParam("saved") == "1" {
		s.Notice = "Tema tersimpan."
	}
	return web.Render(c, http.StatusOK, page(s))
}

// PATCH .../theme (theme_id, primary_color, font_heading, font_body, background, cover_image)
func (h *Handler) Save(c echo.Context) error {
	w := ctxWedding(c)
	themeID := c.FormValue("theme_id")
	in := settingsFrom(c.FormValue)
	_, err := h.svc.Save(c.Request().Context(), w.ID, themeID, in)
	var se SettingsError
	switch {
	case errors.As(err, &se):
		return web.Render(c, http.StatusUnprocessableEntity, page(pageState{W: w, ThemeID: themeID, Settings: in, Errors: se}))
	case errors.Is(err, ErrUnknownTheme):
		return web.Render(c, http.StatusUnprocessableEntity, page(pageState{W: w, ThemeID: w.ThemeID, Settings: in, Errors: map[string]string{"theme_id": "Pilih salah satu tema"}}))
	case err != nil:
		return err
	}
	return c.Redirect(http.StatusSeeOther, pageURL(w)+"?saved=1")
}

// GET .../theme/preview?theme_id=&primary_color=… — undangan dengan data wedding
// (bagian kosong diisi contoh) + pengaturan dari query (belum disimpan).
func (h *Handler) Preview(c echo.Context) error {
	w := ctxWedding(c)
	v, err := h.previews.Preview(c.Request().Context(), w)
	if err != nil {
		return err
	}
	if id := c.QueryParam("theme_id"); Exists(id) {
		v.ThemeID = id
	}
	if len(c.QueryParams()) > 0 {
		v.Settings = mergeValid(v.Settings, settingsFrom(c.QueryParam))
	}
	c.Response().Header().Set("Cache-Control", "no-store")
	c.Response().Header().Set("X-Robots-Tag", "noindex")
	return web.Render(c, http.StatusOK, Render(v))
}

// mergeValid memakai nilai override per field bila valid; field tidak valid
// tetap memakai nilai tersimpan supaya preview tidak rusak saat user mengetik.
func mergeValid(saved, override view.Settings) view.Settings {
	out := override
	if _, err := ValidateSettings(override); err != nil {
		var se SettingsError
		errors.As(err, &se)
		keep := map[string]*string{
			"primary_color": &out.PrimaryColor, "font_heading": &out.FontHeading, "font_body": &out.FontBody,
			"background": &out.Background, "cover_image": &out.CoverImage,
		}
		old := map[string]string{
			"primary_color": saved.PrimaryColor, "font_heading": saved.FontHeading, "font_body": saved.FontBody,
			"background": saved.Background, "cover_image": saved.CoverImage,
		}
		for field := range se {
			*keep[field] = old[field]
		}
	}
	norm, err := ValidateSettings(out)
	if err != nil {
		return saved
	}
	return norm
}
