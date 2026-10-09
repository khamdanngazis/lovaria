package wedding

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc         *Service
	archiveDays int
	home        HomeWidgets
	themes      ThemeChoices
}

// ThemeChoice: satu tema yang ditawarkan di langkah pertama wizard (T29).
type ThemeChoice struct {
	ID, Name, Description string
	Region                string // Koleksi Daerah; "" = koleksi utama
	Thumb                 string // URL thumbnail ("" → kartu warna)
	Primary, Surface      string
	Featured              bool
	DemoURL               string // undangan contoh ("" = belum ada)
}

// ThemeChoices mengembalikan tema yang boleh dipilih pasangan baru.
type ThemeChoices func(ctx context.Context) ([]ThemeChoice, error)

// HomeWidgets menyusun bagian beranda wedding dari modul lain (ringkasan tamu,
// RSVP, ucapan, galeri, checklist — disediakan paket dashboard, T13). Modul
// wedding hanya memiliki header & kartu status; nil → tanpa widget.
type HomeWidgets func(ctx context.Context, w Wedding) (templ.Component, error)

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, archiveDays: 365}
}

var (
	// Wizard (T29): 1 = pilih tema, 2 = nama mempelai & tanggal. Judul dan alamat
	// undangan dibuat otomatis; semuanya bisa diubah setelahnya.
	wizardFields = []string{"theme_id", "groom_name", "bride_name", "wedding_date"}
	infoFields   = []string{"title", "wedding_date", "description", "main_photo_url", "timezone"}
	coupleFields = []string{"groom_name", "bride_name", "groom_photo_url", "bride_photo_url", "groom_description", "bride_description"}
)

func formFrom(c echo.Context, fields []string) form {
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, k := range fields {
		f.Values[k] = c.FormValue(k)
	}
	return f
}

func (f *form) applyErr(err error) bool {
	var v ValidationError
	if !errors.As(err, &v) {
		return false
	}
	for k, m := range v {
		f.Errors[k] = m
	}
	return true
}

func render(c echo.Context, status int, fragment, page templ.Component) error {
	if web.IsHTMX(c) {
		return web.Render(c, status, fragment)
	}
	return web.Render(c, status, page)
}

func currentUser(c echo.Context) (web.User, error) {
	u, ok := web.CurrentUser(c.Request().Context())
	if !ok {
		return web.User{}, echo.NewHTTPError(http.StatusUnauthorized)
	}
	return u, nil
}

// ---------- Daftar & wizard ----------

// GET /dashboard/weddings
func (h *Handler) List(c echo.Context) error {
	u, err := currentUser(c)
	if err != nil {
		return err
	}
	ws, err := h.svc.ListWeddingsByOwner(c.Request().Context(), u.ID)
	if err != nil {
		return err
	}
	if len(ws) == 0 {
		return web.Redirect(c, "/dashboard/weddings/new")
	}
	return web.Render(c, http.StatusOK, listPage(ws, h.svc.clock()))
}

// wizardThemes: pilihan tema wizard (kosong bila modul theme tidak dipasang).
func (h *Handler) wizardThemes(c echo.Context) ([]ThemeChoice, error) {
	if h.themes == nil {
		return nil, nil
	}
	return h.themes(c.Request().Context())
}

func themeIn(list []ThemeChoice, id string) bool {
	for _, t := range list {
		if t.ID == id {
			return true
		}
	}
	return false
}

// GET /dashboard/weddings/new → langkah 1 wizard (pilih tema). ?tema=<id> dari
// halaman tema di landing: tema sudah terpilih, langsung ke langkah 2.
func (h *Handler) New(c echo.Context) error {
	themes, err := h.wizardThemes(c)
	if err != nil {
		return err
	}
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	step := 1
	if len(themes) == 0 {
		step = 2
	} else if id := c.QueryParam("tema"); themeIn(themes, id) {
		f.Values["theme_id"] = id
		step = 2
	}
	return web.Render(c, http.StatusOK, newPage(step, f, themes))
}

// POST /dashboard/weddings/new/steps/:step → validasi langkah, tampilkan langkah berikutnya.
// Form membawa nilai semua langkah (hidden field). back=1 kembali tanpa validasi.
func (h *Handler) Step(c echo.Context) error {
	step, err := strconv.Atoi(c.Param("step"))
	if err != nil || step < 1 || step > wizardSteps {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	themes, err := h.wizardThemes(c)
	if err != nil {
		return err
	}
	f := formFrom(c, wizardFields)

	if c.FormValue("back") != "" {
		prev := max(step-1, 1)
		return render(c, http.StatusOK, wizardStep(prev, f, themes), newPage(prev, f, themes))
	}
	if step == 1 {
		if len(themes) > 0 && !themeIn(themes, f.v("theme_id")) {
			f.Errors["theme_id"] = "Pilih salah satu tema untuk melanjutkan"
			return render(c, http.StatusUnprocessableEntity, wizardStep(1, f, themes), newPage(1, f, themes))
		}
		return render(c, http.StatusOK, wizardStep(2, f, themes), newPage(2, f, themes))
	}
	return echo.NewHTTPError(http.StatusNotFound) // langkah terakhir dikirim ke POST /dashboard/weddings
}

// POST /dashboard/weddings (form: theme_id, groom_name, bride_name, wedding_date)
// → wedding draf dengan tema pilihan → halaman pratinjau pertama.
func (h *Handler) Create(c echo.Context) error {
	u, err := currentUser(c)
	if err != nil {
		return err
	}
	themes, err := h.wizardThemes(c)
	if err != nil {
		return err
	}
	f := formFrom(c, wizardFields)
	themeID := f.v("theme_id")
	if !themeIn(themes, themeID) {
		themeID = "" // tema tak dikenal / nonaktif → tema bawaan
	}
	w, err := h.svc.CreateWedding(c.Request().Context(), u.ID, CreateInput{
		GroomName: f.v("groom_name"), BrideName: f.v("bride_name"), WeddingDate: f.v("wedding_date"),
		Title:   fmt.Sprintf("Pernikahan %s & %s", firstWord(f.v("groom_name")), firstWord(f.v("bride_name"))),
		ThemeID: themeID,
	})
	if f.applyErr(err) {
		return render(c, http.StatusUnprocessableEntity, wizardStep(2, f, themes), newPage(2, f, themes))
	}
	if err != nil {
		return err
	}
	return web.Redirect(c, weddingURL(w, "/start"))
}

// ---------- Per wedding (sudah lewat RequireWeddingOwner) ----------

func mustWedding(c echo.Context) Wedding {
	w, ok := FromContext(c.Request().Context())
	if !ok {
		panic("wedding: handler dipasang tanpa RequireWeddingOwner")
	}
	return w
}

// GET /dashboard/weddings/:weddingID/start — pratinjau pertama setelah wizard
// (T29): pasangan langsung melihat undangannya dengan tema pilihan; bagian
// yang masih kosong diisi contoh.
func (h *Handler) Start(c echo.Context) error {
	return web.Render(c, http.StatusOK, startPage(mustWedding(c)))
}

// GET /dashboard/weddings/:weddingID
func (h *Handler) Overview(c echo.Context) error {
	o, err := h.overview(c, mustWedding(c))
	if err != nil {
		return err
	}
	o.Welcome = c.QueryParam("welcome") == "1"
	switch c.QueryParam("status") {
	case StatusPublished:
		o.Notice = "Undangan dipublikasikan. Bagikan link ke tamu dari menu Tamu."
	case StatusDraft:
		o.Notice = "Publikasi ditarik. Undangan kembali hanya bisa dilihat Anda."
	}
	if c.QueryParam("archive") == "saved" {
		o.Notice = "Visibilitas arsip disimpan."
	}
	return web.Render(c, http.StatusOK, overviewPage(o))
}

func (h *Handler) overview(c echo.Context, w Wedding) (overviewState, error) {
	ctx := c.Request().Context()
	couple, err := h.svc.GetCouple(ctx, w.ID)
	if err != nil {
		return overviewState{}, err
	}
	checklist, err := h.svc.Checklist(ctx, w.ID)
	if err != nil {
		return overviewState{}, err
	}
	history, err := h.svc.History(ctx, w.ID, 10)
	if err != nil {
		return overviewState{}, err
	}
	o := overviewState{W: w, Couple: couple, Checklist: checklist, History: history, Now: h.svc.clock()}
	if h.home != nil {
		if o.Home, err = h.home(ctx, w); err != nil {
			return overviewState{}, err
		}
	}
	return o, nil
}

// PATCH /dashboard/weddings/:weddingID/status (status=published|draft) — Publish/Unpublish.
func (h *Handler) UpdateStatus(c echo.Context) error {
	w := mustWedding(c)
	u, err := currentUser(c)
	if err != nil {
		return err
	}
	to := c.FormValue("status")
	updated, err := h.svc.Transition(c.Request().Context(), w.ID, to, Actor{Kind: ActorUser, UserID: u.ID})
	// Belum lunas (T23): arahkan ke halaman publikasi & pembayaran.
	if errors.Is(err, ErrPaymentRequired) {
		return c.Redirect(http.StatusSeeOther, weddingURL(w, "/publish"))
	}
	var te *TransitionError
	var ce *ChecklistError
	if errors.As(err, &te) || errors.As(err, &ce) {
		o, oerr := h.overview(c, w)
		if oerr != nil {
			return oerr
		}
		o.Error = err.Error()
		return web.Render(c, http.StatusUnprocessableEntity, overviewPage(o))
	}
	if err != nil {
		return err
	}
	// Dipublikasikan pada/sesudah hari H → langsung ke status yang sesuai tanggal.
	if to == StatusPublished {
		if err := h.svc.AdvanceNow(c.Request().Context(), updated.ID, h.archiveDays); err != nil {
			return err
		}
	}
	return c.Redirect(http.StatusSeeOther, weddingURL(updated, "?status="+to))
}

// PATCH /dashboard/weddings/:weddingID/archive-visibility (T19)
func (h *Handler) UpdateArchiveVisibility(c echo.Context) error {
	w := mustWedding(c)
	updated, err := h.svc.SetArchiveVisibility(c.Request().Context(), w.ID, c.FormValue("visibility"))
	if errors.Is(err, ErrInvalidArchiveVisibility) {
		o, oerr := h.overview(c, w)
		if oerr != nil {
			return oerr
		}
		o.Error = "Pilih visibilitas arsip: Publik atau Privat."
		return web.Render(c, http.StatusUnprocessableEntity, overviewPage(o))
	}
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, weddingURL(updated, "?archive=saved"))
}

func infoValues(w Wedding) form {
	return form{Errors: map[string]string{}, Values: map[string]string{
		"title": w.Title, "wedding_date": w.WeddingDate.Format(dateLayout),
		"description": w.Description, "main_photo_url": deref(w.MainPhotoURL), "timezone": w.Timezone,
	}}
}

// GET /dashboard/weddings/:weddingID/info
func (h *Handler) InfoPage(c echo.Context) error {
	w := mustWedding(c)
	return web.Render(c, http.StatusOK, infoPage(w, infoValues(w)))
}

// PATCH /dashboard/weddings/:weddingID/info
func (h *Handler) UpdateInfo(c echo.Context) error {
	w := mustWedding(c)
	f := formFrom(c, infoFields)
	updated, err := h.svc.UpdateWeddingInfo(c.Request().Context(), w.ID, InfoInput{
		Title: f.v("title"), WeddingDate: f.v("wedding_date"), Description: f.v("description"),
		MainPhotoURL: f.v("main_photo_url"), Timezone: f.v("timezone"),
	})
	if f.applyErr(err) {
		return render(c, http.StatusUnprocessableEntity, infoForm(w, f), infoPage(w, f))
	}
	if err != nil {
		return err
	}
	f = infoValues(updated)
	f.Notice = "Perubahan tersimpan."
	return render(c, http.StatusOK, infoForm(updated, f), infoPage(updated, f))
}

func coupleValues(c Couple) form {
	return form{Errors: map[string]string{}, Values: map[string]string{
		"groom_name": c.GroomName, "bride_name": c.BrideName,
		"groom_photo_url": deref(c.GroomPhotoURL), "bride_photo_url": deref(c.BridePhotoURL),
		"groom_description": c.GroomDescription, "bride_description": c.BrideDescription,
	}}
}

// GET /dashboard/weddings/:weddingID/couple
func (h *Handler) CouplePage(c echo.Context) error {
	w := mustWedding(c)
	couple, err := h.svc.GetCouple(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, couplePage(w, coupleValues(couple)))
}

// PATCH /dashboard/weddings/:weddingID/couple
func (h *Handler) UpdateCouple(c echo.Context) error {
	w := mustWedding(c)
	f := formFrom(c, coupleFields)
	updated, err := h.svc.UpdateCouple(c.Request().Context(), w.ID, CoupleInput{
		GroomName: f.v("groom_name"), BrideName: f.v("bride_name"),
		GroomPhotoURL: f.v("groom_photo_url"), BridePhotoURL: f.v("bride_photo_url"),
		GroomDescription: f.v("groom_description"), BrideDescription: f.v("bride_description"),
	})
	if f.applyErr(err) {
		return render(c, http.StatusUnprocessableEntity, coupleForm(w, f), couplePage(w, f))
	}
	if err != nil {
		return err
	}
	f = coupleValues(updated)
	f.Notice = "Perubahan tersimpan."
	return render(c, http.StatusOK, coupleForm(w, f), couplePage(w, f))
}

// PATCH /dashboard/weddings/:weddingID/slug (slug=…) — dari halaman Bagikan (T14).
func (h *Handler) UpdateSlug(c echo.Context) error {
	w := mustWedding(c)
	_, err := h.svc.ChangeSlug(c.Request().Context(), w.ID, c.FormValue("slug"))
	var se *SlugError
	if errors.As(err, &se) {
		return c.Redirect(http.StatusSeeOther, weddingURL(w, "/share?slug_err="+url.QueryEscape(se.Msg)))
	}
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, weddingURL(w, "/share?ok=slug"))
}
