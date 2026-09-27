package gift

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

func TestGiftCRUDViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/gifts")

	if rec := req(e, owner, http.MethodGet, base, nil, false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Belum ada rekening") {
		t.Fatalf("kosong: %d", rec.Code)
	}
	rec := req(e, owner, http.MethodPost, base, url.Values{"type": {"bank"}, "provider": {"BCA"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#gift-form-new" || !strings.Contains(rec.Body.String(), "Nama pemilik wajib diisi") {
		t.Fatalf("invalid: %d", rec.Code)
	}
	rec = req(e, owner, http.MethodPost, base, url.Values{"type": {"bank"}, "provider": {"BCA"}, "account_number": {"1234567890"}, "account_name": {"Budi"}}, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "1234567890") {
		t.Fatalf("create: %d", rec.Code)
	}
	as, _ := f.svc.List(ctx, w.ID)
	item := base + "/" + as[0].ID.String()
	if rec := req(e, owner, http.MethodGet, item+"/edit", nil, true); !strings.Contains(rec.Body.String(), `value="1234567890"`) {
		t.Error("edit")
	}
	rec = req(e, owner, http.MethodPost, item, url.Values{"_method": {"PATCH"}, "type": {"address"}, "address": {"Jl. Mawar 1"}}, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update no-JS: %d", rec.Code)
	}
	if a, _ := f.svc.Get(ctx, w.ID, as[0].ID); a.Type != TypeAddress || a.AccountNumber != "" {
		t.Errorf("update = %+v", a)
	}
	if rec := req(e, owner, http.MethodDelete, item, nil, true); rec.Code != http.StatusOK {
		t.Errorf("delete: %d", rec.Code)
	}
}

func TestGiftRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, wb := f.newWedding(t, "bob@example.com")
	acc, _ := f.svc.Create(ctx, w.ID, bank("BCA", "1234567890", "Alice"))
	item := w.DashboardURL("/gifts/" + acc.ID.String())
	valid := url.Values{"type": {"bank"}, "provider": {"X"}, "account_number": {"99999999"}, "account_name": {"Bob"}}

	for _, r := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, w.DashboardURL("/gifts"), nil},
		{http.MethodGet, w.DashboardURL("/gifts/new"), nil},
		{http.MethodPost, w.DashboardURL("/gifts"), valid},
		{http.MethodGet, item + "/edit", nil},
		{http.MethodPatch, item, valid},
		{http.MethodDelete, item, nil},
		{http.MethodPatch, item + "/position", url.Values{"direction": {"up"}}},
		{http.MethodGet, wb.DashboardURL("/gifts/" + acc.ID.String() + "/edit"), nil},
		{http.MethodPatch, wb.DashboardURL("/gifts/" + acc.ID.String()), valid},
		{http.MethodDelete, wb.DashboardURL("/gifts/" + acc.ID.String()), nil},
	} {
		if rec := req(e, bob, r.method, r.path, r.form, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", r.method, r.path, rec.Code)
		}
	}
	if as, _ := f.svc.List(ctx, w.ID); len(as) != 1 || as[0].AccountName != "Alice" {
		t.Error("akun alice berubah")
	}
}
