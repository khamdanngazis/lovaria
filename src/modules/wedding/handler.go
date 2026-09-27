package wedding

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc         *Service
	archiveDays int
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc, archiveDays: 365}
}

var (
	wizardFields = []string{"groom_name", "bride_name", "title", "wedding_date", "description"}
	stepFields   = map[int][]string{1: {"groom_name", "bride_name"}, 2: {"title", "wedding_date"}, 3: {"description"}}
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
	return web.Render(c, http.StatusOK, listPage(ws))
}

// GET /dashboard/weddings/new → langkah 1 wizard.
func (h *Handler) New(c echo.Context) error {
	return web.Render(c, http.StatusOK, newPage(1, form{Values: map[string]string{}, Errors: map[string]string{}}))
}

// POST /dashboard/weddings/new/steps/:step → validasi langkah, tampilkan langkah berikutnya.
// Form membawa nilai semua langkah (hidden field). back=1 kembali tanpa validasi.
func (h *Handler) Step(c echo.Context) error {
	step, err := strconv.Atoi(c.Param("step"))
	if err != nil || step < 1 || step > wizardSteps {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	f := formFrom(c, wizardFields)

	if c.FormValue("back") != "" {
		prev := max(step-1, 1)
		return render(c, http.StatusOK, wizardStep(prev, f), newPage(prev, f))
	}

	fields := map[string]string{}
	for _, k := range stepFields[step] {
		fields[k] = f.v(k)
	}
	if f.applyErr(ValidateFields(fields)) {
		return render(c, http.StatusUnprocessableEntity, wizardStep(step, f), newPage(step, f))
	}

	next := step + 1
	if next == 2 && f.v("title") == "" {
		f.Values["title"] = fmt.Sprintf("Pernikahan %s & %s", firstWord(f.v("groom_name")), firstWord(f.v("bride_name")))
	}
	return render(c, http.StatusOK, wizardStep(next, f), newPage(next, f))
}

// POST /dashboard/weddings (form: groom_name, bride_name, title, wedding_date, description)
func (h *Handler) Create(c echo.Context) error {
	u, err := currentUser(c)
	if err != nil {
		return err
	}
	f := formFrom(c, wizardFields)
	w, err := h.svc.CreateWedding(c.Request().Context(), u.ID, CreateInput{
		GroomName: f.v("groom_name"), BrideName: f.v("bride_name"),
		Title: f.v("title"), WeddingDate: f.v("wedding_date"), Description: f.v("description"),
	})
	if f.applyErr(err) {
		// Kembali ke langkah paling awal yang punya error.
		step := wizardSteps
		for s := wizardSteps; s >= 1; s-- {
			for _, k := range stepFields[s] {
				if f.e(k) != "" {
					step = s
				}
			}
		}
		return render(c, http.StatusUnprocessableEntity, wizardStep(step, f), newPage(step, f))
	}
	if err != nil {
		return err
	}
	return web.Redirect(c, weddingURL(w, "?welcome=1"))
}

// ---------- Per wedding (sudah lewat RequireWeddingOwner) ----------

func mustWedding(c echo.Context) Wedding {
	w, ok := FromContext(c.Request().Context())
	if !ok {
		panic("wedding: handler dipasang tanpa RequireWeddingOwner")
	}
	return w
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
	return overviewState{W: w, Couple: couple, Checklist: checklist, History: history}, nil
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
