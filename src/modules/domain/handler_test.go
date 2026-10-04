package domain

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

func newTestServer(t *testing.T, svc *Service, ws *wedding.Service) *echo.Echo {
	t.Helper()
	cfg, _ := config.LoadFrom(func(k string) string { return map[string]string{"APP_ENV": config.EnvTest}[k] })
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
	owned := wedding.Register(e.Group("/dashboard/weddings"), wedding.Deps{Service: ws})
	Register(owned, Deps{Service: svc})
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

func TestDomainPageFlow(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f.svc, f.weddings)
	owner, w := f.newWedding(t, "a@example.com")
	page := w.DashboardURL("/domain")

	if rec := req(e, owner, http.MethodGet, page, nil, false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Daftarkan domain") {
		t.Fatalf("form: %d", rec.Code)
	}
	rec := req(e, owner, http.MethodPost, page, url.Values{"domain": {"https://www.x.com"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "tanpa http") || !strings.Contains(rec.Body.String(), `value="https://www.x.com"`) {
		t.Fatalf("invalid: %d", rec.Code)
	}
	rec = req(e, owner, http.MethodPost, page, url.Values{"domain": {"samuelsarah.com"}}, true)
	body := rec.Body.String()
	for _, want := range []string{"Menunggu verifikasi", "CNAME", "domains.lovoria.com", `font-semibold">@</dd>`, "CNAME flattening", "CNAME belum ditemukan"} {
		if !strings.Contains(body, want) {
			t.Errorf("setelah daftar: tidak memuat %q", want)
		}
	}
	f.cf.activate("samuelsarah.com")
	rec = req(e, owner, http.MethodPost, page+"/check", url.Values{}, true)
	if !strings.Contains(rec.Body.String(), "Status diperbarui: Aktif") || strings.Contains(rec.Body.String(), "Cara menghubungkan") {
		t.Errorf("cek ulang: %s", rec.Body.String())
	}
	// Hapus tanpa JS → redirect; hostname Cloudflare ikut terhapus.
	rec = req(e, owner, http.MethodPost, page, url.Values{"_method": {"DELETE"}}, false)
	if rec.Code != http.StatusSeeOther || len(f.cf.deleted) != 1 {
		t.Fatalf("hapus: %d %v", rec.Code, f.cf.deleted)
	}
}

func TestDomainPageDisabled(t *testing.T) {
	f := newFixture(t)
	off := NewService(f.svc.pool, nil, Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e := newTestServer(t, off, f.weddings)
	owner, w := f.newWedding(t, "a@example.com")
	body := req(e, owner, http.MethodGet, w.DashboardURL("/domain"), nil, false).Body.String()
	if !strings.Contains(body, "Custom domain belum tersedia") || strings.Contains(body, "Daftarkan domain") {
		t.Error("nonaktif: harus tampil keterangan, bukan form")
	}
}

func TestDomainRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f.svc, f.weddings)
	_, w := f.newWedding(t, "alice@example.com")
	bob, _ := f.newWedding(t, "bob@example.com")
	f.svc.Add(ctx, w.ID, "www.alice.com") //nolint:errcheck
	page := w.DashboardURL("/domain")
	for _, r := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, page, nil},
		{http.MethodPost, page, url.Values{"domain": {"www.bob.com"}}},
		{http.MethodPost, page + "/check", url.Values{}},
		{http.MethodDelete, page, nil},
	} {
		if rec := req(e, bob, r.method, r.path, r.form, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d", r.method, r.path, rec.Code)
		}
	}
	if d, err := f.svc.Get(ctx, w.ID); err != nil || d.Domain != "www.alice.com" || len(f.cf.deleted) != 0 {
		t.Error("domain alice berubah")
	}
}
