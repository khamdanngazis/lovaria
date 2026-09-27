package guest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// shareCtx: bahan menyusun pesan & link per tamu di daftar tamu.
type shareCtx struct {
	Origin   string
	Template ShareTemplate
	Couple   string
	Date     string
}

func (s shareCtx) Link(g Guest) string { return Link(s.Origin, g.InvitationCode) }

func (s shareCtx) Message(g Guest) string {
	return Render(s.Template.Body, ShareVars{GuestName: g.Name, Couple: s.Couple, Date: s.Date, Link: s.Link(g)})
}

func (s shareCtx) WhatsApp(g Guest) string { return WhatsAppURL(g.Phone, s.Message(g)) }

// coupleNames: "Khamdan & Sarah" (nama depan, seperti judul undangan).
func coupleNames(c wedding.Couple) string {
	first := func(s string) string {
		for i, r := range s {
			if r == ' ' {
				return s[:i]
			}
		}
		return s
	}
	return first(c.GroomName) + " & " + first(c.BrideName)
}

func (h *Handler) share(ctx context.Context, w wedding.Wedding) (shareCtx, error) {
	s := shareCtx{}
	var err error
	if s.Origin, err = h.svc.Origin(ctx, w.ID); err != nil {
		return s, err
	}
	if s.Template, err = h.svc.ShareTemplate(ctx, w.ID); err != nil {
		return s, err
	}
	couple, err := h.weddings.GetCouple(ctx, w.ID)
	if err != nil {
		return s, err
	}
	s.Couple, s.Date = coupleNames(couple), DateText(w.WeddingDate, s.Template.Language)
	return s, nil
}

// shareState: data halaman Bagikan.
type shareState struct {
	W         wedding.Wedding
	Share     shareCtx
	Link      string // link undangan umum (custom domain bila aktif)
	Origin    string // basis alamat /w/… (untuk contoh slug)
	Redirects []wedding.SlugRedirect
	Form      ShareTemplate // isian editor (bisa belum tersimpan saat error)
	Errors    map[string]string
	Notice    string
	SlugError string
}

// general: pesan untuk link umum (tanpa nama tamu).
func (s shareState) general() string {
	return Render(s.Share.Template.Body, ShareVars{GuestName: genericGuest[s.Share.Template.Language], Couple: s.Share.Couple, Date: s.Share.Date, Link: s.Link})
}

// previewData: data Alpine editor (template bawaan & nilai contoh untuk preview).
func (s shareState) previewData() string {
	b, _ := json.Marshal(map[string]any{
		"body":     s.Form.Body,
		"defaults": DefaultTemplates,
		"sample": map[string]string{
			"guest_name": "Budi Santoso", "couple": s.Share.Couple, "link": Link(s.Share.Origin, "ABCD234"),
			"date_id": DateText(s.W.WeddingDate, LangID), "date_en": DateText(s.W.WeddingDate, LangEN),
		},
		"lang": s.Form.Language,
	})
	return "sharePreview(" + string(b) + ")"
}

func (h *Handler) shareState(c echo.Context, w wedding.Wedding) (shareState, error) {
	ctx := c.Request().Context()
	s := shareState{W: w, Errors: map[string]string{}}
	var err error
	if s.Share, err = h.share(ctx, w); err != nil {
		return s, err
	}
	if s.Link, err = h.weddings.CanonicalBaseURL(ctx, w); err != nil {
		return s, err
	}
	if s.Origin, err = h.weddings.CanonicalOrigin(ctx, w.ID); err != nil {
		return s, err
	}
	if s.Redirects, err = h.weddings.SlugRedirects(ctx, w.ID); err != nil {
		return s, err
	}
	s.Form = s.Share.Template
	switch c.QueryParam("ok") {
	case "template":
		s.Notice = "Template pesan tersimpan."
	case "reset":
		s.Notice = "Template kembali ke bawaan."
	case "slug":
		s.Notice = "Alamat undangan diganti. Alamat lama tetap dialihkan ke alamat baru selama 90 hari."
	}
	s.SlugError = c.QueryParam("slug_err")
	return s, nil
}

// GET .../share
func (h *Handler) SharePage(c echo.Context) error {
	w := ctxWedding(c)
	s, err := h.shareState(c, w)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, sharePage(s))
}

// POST .../share/template (language, body)
func (h *Handler) SaveShareTemplate(c echo.Context) error {
	w := ctxWedding(c)
	lang, body := c.FormValue("language"), c.FormValue("body")
	_, err := h.svc.SaveShareTemplate(c.Request().Context(), w.ID, lang, body)
	var v ValidationError
	if errors.As(err, &v) {
		s, serr := h.shareState(c, w)
		if serr != nil {
			return serr
		}
		s.Form, s.Errors = ShareTemplate{Language: lang, Body: body}, v
		return web.Render(c, http.StatusUnprocessableEntity, sharePage(s))
	}
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, w.DashboardURL("/share?ok=template"))
}

// POST .../share/template/reset
func (h *Handler) ResetShareTemplate(c echo.Context) error {
	w := ctxWedding(c)
	if err := h.svc.ResetShareTemplate(c.Request().Context(), w.ID); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, w.DashboardURL("/share?ok=reset"))
}

// POST .../guests/:guestID/shared — dipanggil saat Kirim WA / Salin diklik.
func (h *Handler) MarkShared(c echo.Context) error {
	w := ctxWedding(c)
	id, err := guestID(c)
	if err != nil {
		return err
	}
	if err := h.svc.MarkShared(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// jsString: literal string JavaScript (JSON) untuk atribut Alpine.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// shareRowData: x-data baris tamu (penanda dibagikan + salin).
func shareRowData(markURL string, shared bool) string {
	return "shareRow(" + jsString(markURL) + ", " + map[bool]string{true: "true", false: "false"}[shared] + ")"
}

// shareGeneralData: x-data kartu link umum (salin & Web Share API).
func shareGeneralData(link, text string) string {
	return "shareGeneral(" + jsString(link) + ", " + jsString(text) + ")"
}
