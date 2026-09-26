package story

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

func TestStoryCRUDViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("/stories")

	rec := req(e, owner, http.MethodPost, base, url.Values{"title": {""}, "year": {"2019"}, "day": {"3"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || rec.Header().Get("HX-Retarget") != "#story-form-new" || !strings.Contains(rec.Body.String(), "Pilih bulan dulu") {
		t.Fatalf("invalid: %d %s", rec.Code, rec.Body.String())
	}
	rec = req(e, owner, http.MethodPost, base, url.Values{"title": {"Jadian"}, "year": {"2020"}, "month": {"2"}}, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Februari 2020") {
		t.Fatalf("create: %d", rec.Code)
	}
	if rec := req(e, owner, http.MethodPost, base, url.Values{"title": {"Kenal"}, "year": {"2019"}}, false); rec.Code != http.StatusSeeOther {
		t.Fatalf("create no-JS: %d", rec.Code)
	}
	ss, _ := f.svc.ListStories(ctx, w.ID)
	if len(ss) != 2 || ss[0].Title != "Kenal" {
		t.Fatalf("kronologis: %+v", ss)
	}

	// Edit menampilkan bulan terpilih.
	rec = req(e, owner, http.MethodGet, base+"/"+ss[1].ID.String()+"/edit", nil, true)
	if !strings.Contains(rec.Body.String(), `<option value="2" selected>Februari</option>`) {
		t.Errorf("edit tidak memilih bulan: %s", rec.Body.String())
	}
	if rec := req(e, owner, http.MethodPatch, base+"/"+ss[1].ID.String()+"/position", url.Values{"direction": {"up"}}, true); rec.Code != http.StatusOK {
		t.Fatalf("move: %d", rec.Code)
	}
	if ss2, _ := f.svc.ListStories(ctx, w.ID); ss2[0].Title != "Jadian" {
		t.Error("move tidak tersimpan")
	}
	if rec := req(e, owner, http.MethodDelete, base+"/"+ss[0].ID.String(), nil, true); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestStoryRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, wb := f.newWedding(t, "bob@example.com")
	s, err := f.svc.CreateStory(ctx, w.ID, Input{Title: "Kenal", Year: "2019"})
	if err != nil {
		t.Fatal(err)
	}
	base := w.DashboardURL("/stories")
	item := base + "/" + s.ID.String()
	valid := url.Values{"title": {"Dibajak"}, "year": {"2019"}}

	for _, c := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, base, nil},
		{http.MethodPost, base, valid},
		{http.MethodGet, item + "/edit", nil},
		{http.MethodPatch, item, valid},
		{http.MethodDelete, item, nil},
		{http.MethodPatch, item + "/position", url.Values{"direction": {"up"}}},
		{http.MethodPatch, base + "/order", url.Values{"by": {"date"}}},
		{http.MethodPatch, wb.DashboardURL("/stories/" + s.ID.String()), valid},
	} {
		if rec := req(e, bob, c.method, c.path, c.form, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", c.method, c.path, rec.Code)
		}
	}
	if got, _ := f.svc.GetStory(ctx, w.ID, s.ID); got.Title != "Kenal" {
		t.Fatal("cerita alice berubah")
	}
}
