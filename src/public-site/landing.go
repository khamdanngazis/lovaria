package publicsite

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
	"github.com/khamdanngazis/lovaria/static"
)

// DemoSlug: alamat undangan contoh per tema (dibuat `lovoria demo seed`).
func DemoSlug(themeID string) string { return "contoh-" + themeID }

// landing menyusun data landing page (T18): tema aktif (+ undangan contoh
// bila ada), paket bertanda tampil, dan status login.
func (h *Handler) landing(c echo.Context) (landingData, error) {
	ctx := c.Request().Context()
	base := strings.TrimRight(h.BaseURL, "/")
	d := landingData{URL: base + "/", OGImage: base + static.URL("img/brand/og-lovoria.png")}
	_, d.LoggedIn = web.CurrentUser(ctx)
	d.GoogleVerification, d.BingVerification = h.GoogleVerification, h.BingVerification

	off, err := h.Views.Themes.Disabled(ctx)
	if err != nil {
		return d, err
	}
	for _, t := range theme.All() {
		if off[t.ID] {
			continue // tema dinonaktifkan admin (T16) tidak ditawarkan
		}
		lt := landingTheme{ID: t.ID, Name: t.Name, Description: t.Description, Primary: t.Tokens.Primary, Surface: t.Tokens.Surface, Ink: t.Tokens.Ink, Thumb: t.Thumb(), Featured: t.Featured(),
			Pitch: t.Pitch, Region: t.Region, Accent: t.Tokens.Accent, FontHeading: t.Tokens.FontHeading, FontBody: t.Tokens.FontBody}
		w, err := h.Views.Weddings.GetWeddingBySlug(ctx, DemoSlug(t.ID))
		switch {
		case errors.Is(err, wedding.ErrNotFound):
		case err != nil:
			return d, err
		case w.IsPublic():
			lt.DemoURL = "/w/" + w.Slug
			// Kartu memakai foto sampul & nama pasangan undangan contoh.
			if covers, err := h.Views.Gallery.ByCategory(ctx, w.ID, gallery.CategoryCover, 1); err != nil {
				return d, err
			} else if len(covers) > 0 {
				lt.Photo = covers[0].ThumbURL
			}
			c, err := h.Views.Weddings.GetCouple(ctx, w.ID)
			if err != nil {
				return d, err
			}
			lt.Couple = firstName(c.GroomName) + " & " + firstName(c.BrideName)
		}
		d.Themes = append(d.Themes, lt)
	}
	d.PriceIDR = h.PublishPrice
	return d, nil
}

// GET /tema — etalase semua tema (boleh diindeks; hanya di domain Lunovia).
func (h *Handler) Themes(c echo.Context) error {
	if !h.ownHost(c.Request()) {
		return notFound(c)
	}
	d, err := h.landing(c)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	d.NavPrefix = "/"
	return web.Render(c, http.StatusOK, themesPage(d))
}

// GET /tema/:id — halaman satu tema. Tema tak dikenal / nonaktif → 404.
func (h *Handler) ThemeDetail(c echo.Context) error {
	if !h.ownHost(c.Request()) {
		return notFound(c)
	}
	d, err := h.landing(c)
	if err != nil {
		return err
	}
	for _, t := range d.Themes {
		if t.ID == c.Param("id") {
			c.Response().Header().Set("Cache-Control", "public, max-age=300")
			d.NavPrefix = "/"
			return web.Render(c, http.StatusOK, themeDetailPage(d, t))
		}
	}
	return notFound(c)
}
