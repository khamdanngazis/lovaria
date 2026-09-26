package event

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// newTestServer merakit stack seperti main.go; user login dari header X-Test-User.
func newTestServer(t *testing.T, f fixture) *echo.Echo {
	t.Helper()
	cfg, _ := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	e := server.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := uuid.Parse(c.Request().Header.Get("X-Test-User")); err == nil {
				r := c.Request()
				c.SetRequest(r.WithContext(web.WithUser(r.Context(), web.User{ID: id, Name: "T"})))
			}
			return next(c)
		}
	})
	owned := wedding.Register(e.Group("/dashboard/weddings"), wedding.Deps{Service: f.weddings})
	Register(owned, Deps{Service: f.svc})
	return e
}

func req(e *echo.Echo, user uuid.UUID, method, path string, form url.Values, htmx bool) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, path, body)
	if form != nil {
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	}
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("X-Test-User", user.String())
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

func validForm(name, date, start string) url.Values {
	return url.Values{"name": {name}, "type": {TypeReception}, "date": {date}, "start_time": {start}, "venue": {"Gedung"}}
}

func TestEventCRUDViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/events")

	rec := req(e, owner, http.MethodGet, base, nil, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Belum ada acara") {
		t.Fatalf("list kosong: %d", rec.Code)
	}
	// Form tambah (htmx) → fragment, tanggal default = tanggal wedding.
	rec = req(e, owner, http.MethodGet, base+"/new", nil, true)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "<html") || !strings.Contains(rec.Body.String(), `value="2026-12-12"`) {
		t.Fatalf("new: %d", rec.Code)
	}

	// Validasi gagal → 422, retarget ke form.
	rec = req(e, owner, http.MethodPost, base, url.Values{"name": {""}, "date": {"2026-12-12"}, "start_time": {"08:00"}, "venue": {"x"}, "maps_url": {"https://evil.com"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#event-form-new" {
		t.Fatalf("invalid: %d retarget=%q", rec.Code, rec.Header().Get("HX-Retarget"))
	}
	if !strings.Contains(rec.Body.String(), "Nama acara wajib diisi") || !strings.Contains(rec.Body.String(), "Google Maps") {
		t.Errorf("pesan error tidak ada: %s", rec.Body.String())
	}

	// Sukses (htmx) → section terbaru.
	rec = req(e, owner, http.MethodPost, base, validForm("Resepsi", "2026-12-12", "11:00"), true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `id="events"`) || !strings.Contains(rec.Body.String(), "Resepsi") {
		t.Fatalf("create htmx: %d", rec.Code)
	}
	// Sukses tanpa JS → redirect ke daftar.
	rec = req(e, owner, http.MethodPost, base, validForm("Akad", "2026-12-12", "08:00"), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != base {
		t.Fatalf("create no-JS: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	evs, _ := f.svc.ListEvents(ctx, w.ID)
	if len(evs) != 2 || evs[0].Name != "Akad" {
		t.Fatalf("urutan kronologis: %+v", evs)
	}
	akad, resepsi := evs[0], evs[1]

	// Edit inline (htmx) → form sebagai <li>.
	rec = req(e, owner, http.MethodGet, base+"/"+akad.ID.String()+"/edit", nil, true)
	if !strings.Contains(rec.Body.String(), `<li id="event-`+akad.ID.String()) || !strings.Contains(rec.Body.String(), `value="Akad"`) {
		t.Errorf("edit: %s", rec.Body.String())
	}
	// Update tanpa JS (_method=PATCH).
	form := validForm("Akad Nikah", "2026-12-12", "08:00")
	form.Set("_method", "PATCH")
	if rec := req(e, owner, http.MethodPost, base+"/"+akad.ID.String(), form, false); rec.Code != http.StatusSeeOther {
		t.Fatalf("update no-JS: %d", rec.Code)
	}
	// Update invalid (htmx) → retarget ke form edit.
	rec = req(e, owner, http.MethodPatch, base+"/"+akad.ID.String(), url.Values{"name": {"x"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#event-form-"+akad.ID.String() {
		t.Errorf("update invalid: %d %q", rec.Code, rec.Header().Get("HX-Retarget"))
	}

	// Reorder: Resepsi naik → paling atas; lalu urutkan per tanggal → Akad di atas lagi.
	if rec := req(e, owner, http.MethodPatch, base+"/"+resepsi.ID.String()+"/position", url.Values{"direction": {"up"}}, true); rec.Code != http.StatusOK {
		t.Fatalf("move: %d", rec.Code)
	}
	if evs, _ := f.svc.ListEvents(ctx, w.ID); evs[0].ID != resepsi.ID {
		t.Error("move tidak tersimpan")
	}
	if rec := req(e, owner, http.MethodPatch, base+"/"+resepsi.ID.String()+"/position", url.Values{"direction": {"sideways"}}, true); rec.Code != http.StatusBadRequest {
		t.Errorf("direction invalid: %d", rec.Code)
	}
	if rec := req(e, owner, http.MethodPatch, base+"/order", url.Values{"by": {"date"}}, true); rec.Code != http.StatusOK {
		t.Fatalf("sort: %d", rec.Code)
	}
	if evs, _ := f.svc.ListEvents(ctx, w.ID); evs[0].Name != "Akad Nikah" {
		t.Errorf("sort: %+v", evs)
	}

	// Hapus.
	rec = req(e, owner, http.MethodDelete, base+"/"+resepsi.ID.String(), nil, true)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `id="event-`+resepsi.ID.String()) {
		t.Errorf("delete: %d", rec.Code)
	}
	if rec := req(e, owner, http.MethodDelete, base+"/"+resepsi.ID.String(), nil, true); rec.Code != http.StatusNotFound {
		t.Errorf("delete 2x: %d", rec.Code)
	}
}

// CRUD events hanya untuk owner wedding: user lain → 404 di semua route.
func TestEventRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, wb := f.newWedding(t, "bob@example.com")
	ev, err := f.svc.CreateEvent(ctx, w.ID, Input{Name: "Akad", Type: TypeAkad, Date: "2026-12-12", StartTime: "08:00", Venue: "Masjid"})
	if err != nil {
		t.Fatal(err)
	}
	base := w.DashboardURL("/events")
	item := base + "/" + ev.ID.String()

	cases := []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, base, nil},
		{http.MethodGet, base + "/new", nil},
		{http.MethodPost, base, validForm("Palsu", "2026-12-12", "08:00")},
		{http.MethodGet, item + "/edit", nil},
		{http.MethodPatch, item, validForm("Dibajak", "2026-12-12", "08:00")},
		{http.MethodDelete, item, nil},
		{http.MethodPatch, item + "/position", url.Values{"direction": {"down"}}},
		{http.MethodPatch, base + "/order", url.Values{"by": {"date"}}},
	}
	for _, c := range cases {
		if rec := req(e, bob, c.method, c.path, c.form, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", c.method, c.path, rec.Code)
		}
	}
	// ID acara alice lewat URL wedding bob sendiri → tetap 404 (query memfilter wedding_id).
	if rec := req(e, bob, http.MethodGet, wb.DashboardURL("/events/"+ev.ID.String()+"/edit"), nil, true); rec.Code != http.StatusNotFound {
		t.Errorf("acara alice via wedding bob: %d", rec.Code)
	}
	if rec := req(e, bob, http.MethodPatch, wb.DashboardURL("/events/"+ev.ID.String()), validForm("Dibajak", "2026-12-12", "08:00"), true); rec.Code != http.StatusNotFound {
		t.Errorf("patch via wedding bob: %d", rec.Code)
	}

	evs, _ := f.svc.ListEvents(ctx, w.ID)
	if len(evs) != 1 || evs[0].Name != "Akad" {
		t.Fatalf("data alice berubah: %+v", evs)
	}
}
