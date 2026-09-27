package theme_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/gift"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/storage"
	"github.com/khamdanngazis/lovaria/src/platform/web"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
)

type app struct {
	e        *echo.Echo
	weddings *wedding.Service
	themes   *theme.Service
	auth     *auth.Service
	gallery  *gallery.Service
}

func newApp(t *testing.T) app {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, _ := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	store, _ := storage.NewLocal(t.TempDir(), "/media")
	ws := wedding.NewService(wedding.NewRepository(pool))
	themes := theme.NewService(pool, ws)
	photos := gallery.NewService(gallery.NewRepository(pool), store, ws, 500<<20, log)
	themes.SetMusicStore(photos)
	views := &publicsite.ViewBuilder{
		Weddings: ws, Events: event.NewService(event.NewRepository(pool)), Stories: story.NewService(story.NewRepository(pool)),
		Gallery: photos, Themes: themes, CacheTTL: -1,
		Guestbook: guestbook.NewService(pool, nil), Gifts: gift.NewService(pool),
	}
	e := server.New(cfg, log)
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
	theme.Register(owned, theme.Deps{Service: themes, Previewer: views})
	return app{e: e, weddings: ws, themes: themes, gallery: photos, auth: auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log)}
}

func (a app) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	ctx := context.Background()
	u, err := a.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := a.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "Khamdan", BrideName: "Sarah", Title: "T", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func (a app) do(user uuid.UUID, method, path string, form url.Values) *httptest.ResponseRecorder {
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
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, r)
	return rec
}

func TestThemePageSaveAndPreview(t *testing.T) {
	a := newApp(t)
	owner, w := a.newWedding(t, "a@example.com")
	base := w.DashboardURL("/theme")

	rec := a.do(owner, http.MethodGet, base, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="elegant" checked`) {
		t.Fatalf("page: %d", rec.Code)
	}
	// Regresi: field opsional tidak boleh "required" — pilihan "Bawaan tema" bernilai
	// kosong, jadi validasi browser akan memblokir submit.
	for _, sel := range []string{`id="theme-font-heading" name="font_heading" required`, `id="theme-font-body" name="font_body" required`} {
		if strings.Contains(rec.Body.String(), sel) {
			t.Errorf("select font tidak boleh required: %s", sel)
		}
	}
	// Regresi: contoh font di kartu tema harus CSS valid (tanpa kutip yang ter-escape ganda).
	if !strings.Contains(rec.Body.String(), "font-family:Cormorant Garamond;") || strings.Contains(rec.Body.String(), "&amp;#39;") {
		t.Error("style font-family kartu tema tidak valid")
	}
	for _, name := range []string{"Elegan", "Minimalis", "Romantis", "Modern"} {
		if !strings.Contains(rec.Body.String(), name) {
			t.Errorf("kartu tema %s tidak ada", name)
		}
	}

	// Invalid: warna bukan hex & font di luar whitelist → 422, tidak tersimpan.
	rec = a.do(owner, http.MethodPatch, base, url.Values{"theme_id": {"romantic"}, "primary_color": {"merah"}, "font_heading": {"Comic Sans MS"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Warna harus format hex") || !strings.Contains(rec.Body.String(), "Font tidak tersedia") {
		t.Fatalf("invalid: %d", rec.Code)
	}
	if got, _ := a.weddings.GetWedding(context.Background(), w.ID); got.ThemeID != "elegant" {
		t.Error("tema berubah walau validasi gagal")
	}

	// Valid (form biasa: POST + _method=PATCH).
	rec = a.do(owner, http.MethodPost, base, url.Values{"_method": {"PATCH"}, "theme_id": {"romantic"}, "primary_color": {"#AA3355"}, "font_heading": {"Cinzel"}})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != base+"?saved=1" {
		t.Fatalf("save: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if st, _ := a.themes.Settings(context.Background(), w.ID); st.PrimaryColor != "#aa3355" || st.FontHeading != "Cinzel" {
		t.Errorf("settings = %+v", st)
	}

	// Preview tersimpan: tema romantic + warna tersimpan + data contoh.
	rec = a.do(owner, http.MethodGet, base+"/preview", nil)
	html := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(html, `data-theme="romantic"`) || !strings.Contains(html, "--lv-primary:#aa3355;") || !strings.Contains(html, "Khamdan") || !strings.Contains(html, "sample-photo-1.svg") {
		t.Fatalf("preview: %d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Error("preview tidak boleh di-cache")
	}

	// Preview dengan override belum disimpan: yang valid dipakai, yang invalid diabaikan.
	q := url.Values{"theme_id": {"modern"}, "primary_color": {"#000"}, "font_body": {"Nunito"}, "font_heading": {""}}
	html = a.do(owner, http.MethodGet, base+"/preview?"+q.Encode(), nil).Body.String()
	if !strings.Contains(html, `data-theme="modern"`) || !strings.Contains(html, "family=Nunito") || !strings.Contains(html, "--lv-primary:#aa3355;") {
		t.Errorf("preview override salah")
	}
}

func TestThemeRoutesOwnerOnly(t *testing.T) {
	a := newApp(t)
	_, w := a.newWedding(t, "alice@example.com")
	bob, _ := a.newWedding(t, "bob@example.com")
	base := w.DashboardURL("/theme")
	for _, c := range []struct {
		method, path string
		form         url.Values
	}{
		{http.MethodGet, base, nil},
		{http.MethodGet, base + "/preview", nil},
		{http.MethodPatch, base, url.Values{"theme_id": {"modern"}}},
		{http.MethodPost, base + "/music", url.Values{}},
		{http.MethodDelete, base + "/music", nil},
	} {
		if rec := a.do(bob, c.method, c.path, c.form); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d, want 404", c.method, c.path, rec.Code)
		}
	}
	if got, _ := a.weddings.GetWedding(context.Background(), w.ID); got.ThemeID != "elegant" {
		t.Error("bob berhasil mengganti tema alice")
	}
}
