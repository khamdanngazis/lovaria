package guestbook

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

func TestDashboardModeration(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/guestbook")

	if rec := req(e, owner, http.MethodGet, base, nil, false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Belum ada ucapan") {
		t.Fatalf("kosong: %d", rec.Code)
	}
	good := f.post(t, w.ID, "Budi", "<script>alert(1)</script> Selamat")
	bad := f.post(t, w.ID, "Bot", "anjing")

	body := req(e, owner, http.MethodGet, base, nil, false).Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") || !strings.Contains(body, "&lt;script&gt;") {
		t.Error("pesan harus di-escape di dashboard")
	}
	if !strings.Contains(body, "Disembunyikan · 1") {
		t.Error("statistik disembunyikan")
	}
	// Tampilkan pesan yang tersaring otomatis (htmx, filter ikut dikirim).
	rec := req(e, owner, http.MethodPatch, base+"/"+bad.ID.String(), url.Values{"hidden": {"0"}, "filter": {"hidden"}}, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Tidak ada pesan di filter ini") {
		t.Fatalf("tampilkan: %d", rec.Code)
	}
	if es, _, _ := f.svc.Visible(ctx, w.ID, uuid.Nil, 10); len(es) != 2 {
		t.Errorf("visible = %d", len(es))
	}
	// Sembunyikan tanpa JS → redirect.
	rec = req(e, owner, http.MethodPost, base+"/"+good.ID.String(), url.Values{"_method": {"PATCH"}, "hidden": {"1"}}, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("sembunyikan: %d", rec.Code)
	}
	if es, _, _ := f.svc.Visible(ctx, w.ID, uuid.Nil, 10); len(es) != 1 {
		t.Errorf("visible setelah sembunyi = %d", len(es))
	}
	if rec := req(e, owner, http.MethodDelete, base+"/"+good.ID.String(), nil, true); rec.Code != http.StatusOK {
		t.Errorf("hapus: %d", rec.Code)
	}
	if st, _ := f.svc.Stats(ctx, w.ID); st.Total != 1 {
		t.Errorf("total = %d", st.Total)
	}
}

func TestDashboardOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, wb := f.newWedding(t, "bob@example.com")
	en := f.post(t, w.ID, "Tamu", "Untuk Alice")
	item := w.DashboardURL("/guestbook/" + en.ID.String())

	for _, r := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, w.DashboardURL("/guestbook"), nil},
		{http.MethodPatch, item, url.Values{"hidden": {"1"}}},
		{http.MethodDelete, item, nil},
		// ID entri alice lewat URL wedding bob sendiri.
		{http.MethodPatch, wb.DashboardURL("/guestbook/" + en.ID.String()), url.Values{"hidden": {"1"}}},
		{http.MethodDelete, wb.DashboardURL("/guestbook/" + en.ID.String()), nil},
	} {
		if rec := req(e, bob, r.method, r.path, r.form, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", r.method, r.path, rec.Code)
		}
	}
	if es, _, _ := f.svc.Visible(ctx, w.ID, uuid.Nil, 10); len(es) != 1 {
		t.Error("entri alice berubah")
	}
}
