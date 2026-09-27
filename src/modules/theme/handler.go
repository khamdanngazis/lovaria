package theme

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
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

// settingsFrom membaca pengaturan dari form / query preview.
func settingsFrom(vals url.Values) view.Settings {
	st := view.Settings{
		PrimaryColor: vals.Get("primary_color"), FontHeading: vals.Get("font_heading"), FontBody: vals.Get("font_body"),
		Background: vals.Get("background"), CoverImage: vals.Get("cover_image"),
		MusicURL: vals.Get("music_url"), MusicEnabled: vals.Get("music_enabled") == "1",
		QuoteText: vals.Get("quote_text"), QuoteSource: vals.Get("quote_source"),
		Greeting: vals.Get("greeting"), Closing: vals.Get("closing"),
	}
	// Susunan bagian: urutan = urutan input section_order; bagian yang bisa
	// disembunyikan tapi tidak dicentang visible_sections → disembunyikan.
	if order, ok := vals["section_order"]; ok {
		st.SectionOrder = order
		visible := map[string]bool{}
		for _, id := range vals["visible_sections"] {
			visible[id] = true
		}
		for _, d := range Sections {
			if d.Hideable && !visible[d.ID] {
				st.HiddenSections = append(st.HiddenSections, d.ID)
			}
		}
	}
	return st
}

// GET .../theme
func (h *Handler) Page(c echo.Context) error {
	w := ctxWedding(c)
	st, err := h.svc.Settings(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	s := pageState{W: w, ThemeID: w.ThemeID, Settings: st}
	switch c.QueryParam("saved") {
	case "1":
		s.Notice = "Tema tersimpan."
	case "music":
		s.Notice = "Musik diunggah dan dipakai di undangan."
	case "music-deleted":
		s.Notice = "Musik unggahan dihapus."
	}
	return h.render(c, http.StatusOK, s)
}

// render mengisi daftar tema yang bisa dipilih lalu merender halaman.
func (h *Handler) render(c echo.Context, status int, s pageState) error {
	var err error
	if s.Themes, err = h.svc.Choices(c.Request().Context(), s.W.ThemeID); err != nil {
		return err
	}
	if s.Music, err = h.svc.Music(c.Request().Context(), s.W.ID); err != nil {
		return err
	}
	return web.Render(c, status, page(s))
}

// PATCH .../theme (theme_id, primary_color, font_heading, font_body, background, cover_image)
func (h *Handler) Save(c echo.Context) error {
	w := ctxWedding(c)
	themeID := c.FormValue("theme_id")
	form, err := c.FormParams()
	if err != nil {
		return err
	}
	in := settingsFrom(form)
	// Tombol naik/turun tanpa JS: "move=<id>:up|down" menggeser lalu menyimpan.
	anchor := ""
	if id, dir, ok := strings.Cut(c.FormValue("move"), ":"); ok {
		in.SectionOrder = MoveSection(in, id, dir == "up")
		anchor = "#bagian"
	}
	// Tema yang dinonaktifkan admin hanya boleh dipakai wedding yang sudah memakainya.
	if themeID != w.ThemeID {
		off, err := h.svc.Disabled(c.Request().Context())
		if err != nil {
			return err
		}
		if off[themeID] {
			return h.render(c, http.StatusUnprocessableEntity, pageState{W: w, ThemeID: w.ThemeID, Settings: in, Errors: map[string]string{"theme_id": "Tema ini sedang tidak tersedia"}})
		}
	}
	_, err = h.svc.Save(c.Request().Context(), w.ID, themeID, in)
	var se SettingsError
	switch {
	case errors.As(err, &se):
		return h.render(c, http.StatusUnprocessableEntity, pageState{W: w, ThemeID: themeID, Settings: in, Errors: se})
	case errors.Is(err, ErrUnknownTheme):
		return h.render(c, http.StatusUnprocessableEntity, pageState{W: w, ThemeID: w.ThemeID, Settings: in, Errors: map[string]string{"theme_id": "Pilih salah satu tema"}})
	case err != nil:
		return err
	}
	return c.Redirect(http.StatusSeeOther, pageURL(w)+"?saved=1"+anchor)
}

// POST .../theme/music (multipart "file", MP3 ≤ 8 MB) — unggah musik sendiri.
func (h *Handler) UploadMusic(c echo.Context) error {
	w := ctxWedding(c)
	fail := func(msg string) error {
		st, err := h.svc.Settings(c.Request().Context(), w.ID)
		if err != nil {
			return err
		}
		return h.render(c, http.StatusUnprocessableEntity, pageState{W: w, ThemeID: w.ThemeID, Settings: st, Errors: map[string]string{"music_file": msg}})
	}
	fh, err := c.FormFile("file")
	if err != nil {
		return fail("Pilih berkas MP3")
	}
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	err = h.svc.UploadMusic(c.Request().Context(), w.ID, f)
	var ue *gallery.UploadError
	switch {
	case errors.As(err, &ue):
		return fail(ue.Msg)
	case errors.Is(err, gallery.ErrQuotaExceeded):
		return fail("Kuota penyimpanan paket Anda penuh. Hapus beberapa foto terlebih dahulu.")
	case errors.Is(err, ErrMusicUnavailable):
		return fail("Unggah musik belum tersedia")
	case err != nil:
		return err
	}
	return c.Redirect(http.StatusSeeOther, pageURL(w)+"?saved=music#musik")
}

// DELETE .../theme/music — hapus musik unggahan (kuota dikembalikan).
func (h *Handler) DeleteMusic(c echo.Context) error {
	w := ctxWedding(c)
	if err := h.svc.DeleteMusicUpload(c.Request().Context(), w.ID); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, pageURL(w)+"?saved=music-deleted#musik")
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
		saved := v.Settings
		v.Settings = mergeValid(saved, settingsFrom(c.QueryParams()))
		// Musik hanya dari pilihan sah wedding ini (pustaka / unggahannya sendiri).
		music, err := h.svc.Music(c.Request().Context(), w.ID)
		if err != nil {
			return err
		}
		if v.Settings.MusicURL != "" && !music.Allowed(v.Settings.MusicURL) {
			v.Settings.MusicURL, v.Settings.MusicEnabled = saved.MusicURL, saved.MusicEnabled
		}
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
		for field := range se {
			switch field {
			case "primary_color":
				out.PrimaryColor = saved.PrimaryColor
			case "font_heading":
				out.FontHeading = saved.FontHeading
			case "font_body":
				out.FontBody = saved.FontBody
			case "background":
				out.Background = saved.Background
			case "cover_image":
				out.CoverImage = saved.CoverImage
			case "quote_text", "quote_source":
				out.QuoteText, out.QuoteSource = saved.QuoteText, saved.QuoteSource
			case "greeting":
				out.Greeting = saved.Greeting
			case "closing":
				out.Closing = saved.Closing
			case "hidden_sections", "section_order":
				out.HiddenSections, out.SectionOrder = saved.HiddenSections, saved.SectionOrder
			}
		}
	}
	norm, err := ValidateSettings(out)
	if err != nil {
		return saved
	}
	return norm
}
