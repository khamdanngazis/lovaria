package guest

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
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

var fields = []string{"name", "phone", "email", "group_name", "max_pax", "notes"}

func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("guest: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func guestID(c echo.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param("guestID"))
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

// filterFrom membaca filter dari query string maupun body (hx-include #guest-filters).
func filterFrom(c echo.Context) Filter {
	r := c.Request()
	_ = r.ParseForm()
	page, _ := strconv.Atoi(r.Form.Get("page"))
	return Filter{Q: r.Form.Get("q"), Status: r.Form.Get("status"), Group: r.Form.Get("group"), Page: page}
}

func formFrom(c echo.Context) form {
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, k := range fields {
		f.Values[k] = c.FormValue(k)
	}
	return f
}

func formOf(g Guest) form {
	return form{Errors: map[string]string{}, Values: map[string]string{
		"name": g.Name, "phone": g.Phone, "email": g.Email, "group_name": g.GroupName,
		"max_pax": strconv.Itoa(g.MaxPax), "notes": g.Notes,
	}}
}

func (f form) input() Input {
	return Input{Name: f.v("name"), Phone: f.v("phone"), Email: f.v("email"), GroupName: f.v("group_name"), MaxPax: f.v("max_pax"), Notes: f.v("notes")}
}

func (h *Handler) state(c echo.Context, w wedding.Wedding, f Filter) (listState, error) {
	ctx := c.Request().Context()
	p, err := h.svc.List(ctx, w.ID, f)
	if err != nil {
		return listState{}, err
	}
	st, err := h.svc.Stats(ctx, w.ID)
	if err != nil {
		return listState{}, err
	}
	groups, err := h.svc.Groups(ctx, w.ID)
	if err != nil {
		return listState{}, err
	}
	return listState{W: w, Filter: f, Page: p, Stats: st, Groups: groups}, nil
}

// done: htmx → daftar terbaru (+ tutup editor), tanpa JS → redirect ke daftar.
func (h *Handler) done(c echo.Context, w wedding.Wedding, notice string) error {
	f := filterFrom(c)
	s, err := h.state(c, w, f)
	if err != nil {
		return err
	}
	if !web.IsHTMX(c) {
		return c.Redirect(http.StatusSeeOther, s.query(f.Page))
	}
	s.Notice = notice
	return web.Render(c, http.StatusOK, templ.Join(list(s), clearEditor()))
}

func (h *Handler) invalid(c echo.Context, w wedding.Wedding, id uuid.UUID, g Guest, f form) error {
	link := ""
	if id != uuid.Nil {
		link = h.svc.InvitationURL(g.InvitationCode)
	}
	if web.IsHTMX(c) {
		web.Retarget(c, "#"+formID(id))
		return web.Render(c, http.StatusUnprocessableEntity, editor(w, id, g, f, link))
	}
	s, err := h.state(c, w, filterFrom(c))
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusUnprocessableEntity, page(s, editor(w, id, g, f, link)))
}

// GET .../guests?q=&status=&group=&page=
func (h *Handler) List(c echo.Context) error {
	w := ctxWedding(c)
	s, err := h.state(c, w, filterFrom(c))
	if err != nil {
		return err
	}
	if n, err := strconv.Atoi(c.QueryParam("imported")); err == nil {
		s.Notice = fmt.Sprintf("%d tamu berhasil diimport.", n)
	}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, list(s))
	}
	return web.Render(c, http.StatusOK, page(s, nil))
}

// GET .../guests/new
func (h *Handler) New(c echo.Context) error {
	w := ctxWedding(c)
	ed := editor(w, uuid.Nil, Guest{}, form{Values: map[string]string{"max_pax": "1"}, Errors: map[string]string{}}, "")
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, ed)
	}
	s, err := h.state(c, w, filterFrom(c))
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, page(s, ed))
}

// POST .../guests (quick=1 → dari baris "Tambah cepat")
func (h *Handler) Create(c echo.Context) error {
	w := ctxWedding(c)
	f := formFrom(c)
	quick := c.FormValue("quick") == "1"
	g, err := h.svc.Create(c.Request().Context(), w.ID, f.input())
	var v ValidationError
	if errors.As(err, &v) {
		f.Errors = v
		if quick && web.IsHTMX(c) {
			web.Retarget(c, "#guest-quick")
			return web.Render(c, http.StatusUnprocessableEntity, quickAdd(w, f, false))
		}
		return h.invalid(c, w, uuid.Nil, Guest{}, f)
	}
	if err != nil {
		return err
	}
	if quick && web.IsHTMX(c) {
		// Daftar terbaru + baris tambah cepat dikosongkan (grup diingat) & fokus ke Nama.
		s, err := h.state(c, w, filterFrom(c))
		if err != nil {
			return err
		}
		s.Notice = g.Name + " ditambahkan."
		next := form{Values: map[string]string{"group_name": f.v("group_name")}, Errors: map[string]string{}}
		return web.Render(c, http.StatusOK, templ.Join(list(s), quickAdd(w, next, true)))
	}
	return h.done(c, w, g.Name+" ditambahkan.")
}

// GET .../guests/:guestID/edit
func (h *Handler) Edit(c echo.Context) error {
	w := ctxWedding(c)
	id, err := guestID(c)
	if err != nil {
		return err
	}
	g, err := h.svc.Get(c.Request().Context(), w.ID, id)
	if err != nil {
		return notFound(err)
	}
	ed := editor(w, g.ID, g, formOf(g), h.svc.InvitationURL(g.InvitationCode))
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, ed)
	}
	s, err := h.state(c, w, filterFrom(c))
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, page(s, ed))
}

// PATCH .../guests/:guestID
func (h *Handler) Update(c echo.Context) error {
	w := ctxWedding(c)
	id, err := guestID(c)
	if err != nil {
		return err
	}
	f := formFrom(c)
	g, err := h.svc.Update(c.Request().Context(), w.ID, id, f.input())
	var v ValidationError
	if errors.As(err, &v) {
		f.Errors = v
		orig, gerr := h.svc.Get(c.Request().Context(), w.ID, id)
		if gerr != nil {
			return notFound(gerr)
		}
		return h.invalid(c, w, id, orig, f)
	}
	if err != nil {
		return notFound(err)
	}
	return h.done(c, w, g.Name+" disimpan.")
}

// DELETE .../guests/:guestID
func (h *Handler) Delete(c echo.Context) error {
	w := ctxWedding(c)
	id, err := guestID(c)
	if err != nil {
		return err
	}
	if err := h.svc.Delete(c.Request().Context(), w.ID, id); err != nil {
		return notFound(err)
	}
	return h.done(c, w, "Tamu dihapus.")
}

// DELETE .../guests (ids=...&ids=...) — hapus massal.
func (h *Handler) BulkDelete(c echo.Context) error {
	w := ctxWedding(c)
	r := c.Request()
	_ = r.ParseForm()
	var ids []uuid.UUID
	for _, raw := range r.Form["ids"] {
		if id, err := uuid.Parse(raw); err == nil {
			ids = append(ids, id)
		}
	}
	n, err := h.svc.BulkDelete(r.Context(), w.ID, ids)
	if err != nil {
		return err
	}
	return h.done(c, w, fmt.Sprintf("%d tamu dihapus.", n))
}

// GET .../guests/export → CSV (UTF-8 BOM).
func (h *Handler) Export(c echo.Context) error {
	w := ctxWedding(c)
	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "text/csv; charset=utf-8")
	res.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="tamu-%s.csv"`, w.Slug))
	res.Header().Set("Cache-Control", "no-store")
	res.WriteHeader(http.StatusOK)
	return h.svc.ExportCSV(c.Request().Context(), w.ID, res)
}

// GET .../guests/import
func (h *Handler) ImportPage(c echo.Context) error {
	return web.Render(c, http.StatusOK, importPage(ctxWedding(c), ""))
}

// GET .../guests/import/template → template CSV kosong untuk diisi.
func (h *Handler) ImportTemplate(c echo.Context) error {
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="template-tamu-lovoria.csv"`)
	return c.Blob(http.StatusOK, "text/csv; charset=utf-8", []byte(TemplateCSV))
}

// POST .../guests/import (multipart: file) → pratinjau.
func (h *Handler) ImportPreview(c echo.Context) error {
	w := ctxWedding(c)
	fh, err := c.FormFile("file")
	if err != nil {
		return web.Render(c, http.StatusUnprocessableEntity, importPage(w, "Pilih file CSV dulu."))
	}
	file, err := fh.Open()
	if err != nil {
		return web.Render(c, http.StatusUnprocessableEntity, importPage(w, "File tidak bisa dibaca."))
	}
	defer func() { _ = file.Close() }()
	p, err := ParseCSV(file)
	if err != nil {
		return web.Render(c, http.StatusUnprocessableEntity, importPage(w, "Import gagal: "+err.Error()))
	}
	return web.Render(c, http.StatusOK, previewPage(w, p))
}

// POST .../guests/import/confirm (rows = CSV baris valid dari pratinjau).
func (h *Handler) ImportConfirm(c echo.Context) error {
	w := ctxWedding(c)
	p, err := ParseCSV(strings.NewReader(c.FormValue("rows")))
	if err != nil {
		return web.Render(c, http.StatusUnprocessableEntity, importPage(w, "Import gagal: "+err.Error()))
	}
	n, err := h.svc.Import(c.Request().Context(), w.ID, p.Rows)
	if err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("%s?imported=%d", base(w), n))
}

// ---------- Tempel daftar ----------

// GET .../guests/paste
func (h *Handler) PastePage(c echo.Context) error {
	w := ctxWedding(c)
	groups, err := h.svc.Groups(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, pastePage(w, "", "", "", groups))
}

// POST .../guests/paste (text, group) → tabel periksa yang bisa diedit.
func (h *Handler) PasteReview(c echo.Context) error {
	w := ctxWedding(c)
	text, group := c.FormValue("text"), c.FormValue("group")
	p, err := ParseList(text, group)
	if err != nil {
		groups, gerr := h.svc.Groups(c.Request().Context(), w.ID)
		if gerr != nil {
			return gerr
		}
		return web.Render(c, http.StatusUnprocessableEntity, pastePage(w, text, group, err.Error(), groups))
	}
	rows := make([]Input, len(p.Rows))
	errs := map[int]ValidationError{}
	for i, r := range p.Rows {
		rows[i] = r.Input
		if len(r.Errors) > 0 {
			errs[i] = r.Errors
		}
	}
	return web.Render(c, http.StatusOK, reviewPage(w, rows, errs, group))
}

// POST .../guests/paste/confirm (name[], phone[], group_name[], max_pax[], email[])
func (h *Handler) PasteConfirm(c echo.Context) error {
	w := ctxWedding(c)
	r := c.Request()
	if err := r.ParseForm(); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	col := func(key string, i int) string {
		if vals := r.PostForm[key]; i < len(vals) {
			return vals[i]
		}
		return ""
	}
	var inputs []Input
	for i := range r.PostForm["name"] {
		in := Input{Name: col("name", i), Phone: col("phone", i), GroupName: col("group_name", i), MaxPax: col("max_pax", i), Email: col("email", i)}
		if strings.TrimSpace(in.Name+in.Phone+in.Email) == "" {
			continue // baris kosong (mis. dari "+ Tambah baris") diabaikan
		}
		inputs = append(inputs, in)
	}
	n, errs, err := h.svc.AddMany(r.Context(), w.ID, inputs)
	if errors.Is(err, ErrImportEmpty) {
		return c.Redirect(http.StatusSeeOther, base(w)+"/paste")
	}
	if errors.Is(err, ErrImportTooMany) {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	if err != nil {
		return err
	}
	if errs != nil {
		return web.Render(c, http.StatusUnprocessableEntity, reviewPage(w, inputs, errs, c.FormValue("group")))
	}
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("%s?imported=%d", base(w), n))
}
