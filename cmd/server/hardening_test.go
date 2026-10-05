package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/server"
)

// hardeningApp merakit aplikasi lengkap (newApp + routes) seperti produksi.
func hardeningApp(t *testing.T) (*app, *echo.Echo) {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	cfg, err := config.LoadFrom(func(k string) string {
		return map[string]string{"APP_ENV": config.EnvTest, "STORAGE_DRIVER": "local", "STORAGE_LOCAL_DIR": t.TempDir(), "BASE_URL": "https://lovoria.test"}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := newApp(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), pool)
	if err != nil {
		t.Fatal(err)
	}
	return a, a.routes()
}

func sessionFor(t *testing.T, a *app, email string) (auth.User, string) {
	t.Helper()
	u, err := a.auth.Register(context.Background(), auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := a.auth.CreateSession(context.Background(), u.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return u, tok
}

func request(e *echo.Echo, method, path, token string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(url.Values{"name": {"x"}, "title": {"Dibajak"}}.Encode()))
	r.Host = "lovoria.test"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if token != "" {
		r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

// TestTenantIsolationAllDashboardRoutes: SETIAP route /dashboard/weddings/:weddingID/…
// yang terdaftar di router (bukan daftar manual — route baru otomatis ikut)
// diakses user lain dengan wedding milik korban → harus 404, dan data korban tidak berubah.
func TestTenantIsolationAllDashboardRoutes(t *testing.T) {
	a, e := hardeningApp(t)
	owner, _ := sessionFor(t, a, "korban@example.com")
	_, attacker := sessionFor(t, a, "penyerang@example.com")
	w, err := a.weddings.CreateWedding(context.Background(), owner.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "Asli", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	param := regexp.MustCompile(`:[A-Za-z]+`)
	n := 0
	for _, r := range e.Routes() {
		if !strings.HasPrefix(r.Path, "/dashboard/weddings/:weddingID") {
			continue
		}
		path := param.ReplaceAllStringFunc(r.Path, func(p string) string {
			if p == ":weddingID" {
				return w.ID.String()
			}
			return uuid.New().String()
		})
		n++
		if rec := request(e, r.Method, path, attacker, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s oleh user lain: %d, want 404", r.Method, r.Path, rec.Code)
		}
	}
	if n < 60 {
		t.Fatalf("hanya %d route dashboard wedding terdaftar — router berubah?", n)
	}
	if got, _ := a.weddings.GetWedding(context.Background(), w.ID); got.Title != "Asli" {
		t.Error("data korban berubah")
	}
	t.Logf("%d route dashboard wedding diuji", n)
}

func TestSecurityHeadersAndErrorPages(t *testing.T) {
	a, e := hardeningApp(t)
	_, tok := sessionFor(t, a, "a@example.com")

	// Dashboard: CSP dengan eval (Alpine), no-store, HSTS lewat HTTPS.
	rec := request(e, http.MethodGet, "/dashboard/weddings", tok, map[string]string{"X-Forwarded-Proto": "https"})
	h := rec.Header()
	if h.Get("Content-Security-Policy") != server.CSPDashboard || !strings.Contains(h.Get("Cache-Control"), "no-store") ||
		!strings.HasPrefix(h.Get("Strict-Transport-Security"), "max-age=31536000") || strings.Contains(h.Get("Strict-Transport-Security"), "includeSubDomains") ||
		h.Get("X-Frame-Options") != "SAMEORIGIN" || h.Get("Permissions-Policy") == "" {
		t.Errorf("header dashboard: %v", h)
	}
	// Halaman publik: CSP tanpa eval.
	rec = request(e, http.MethodGet, "/", "", nil)
	if csp := rec.Header().Get("Content-Security-Policy"); csp != server.CSPPublic || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP publik: %q", csp)
	}
	// Tidak ada script / handler inline yang akan diblok CSP. Blok data JSON-LD
	// (type="application/ld+json", T27) bukan script yang dieksekusi, jadi tidak
	// terkena CSP dan dikecualikan dari pemeriksaan.
	ldBlock := regexp.MustCompile(`(?s)<script id="[a-z-]+" type="application/ld\+json">.*?</script>`)
	for _, p := range []string{"/", "/login", "/register", "/privacy", "/terms", "/tema", "/tema/signature"} {
		body := ldBlock.ReplaceAllString(request(e, http.MethodGet, p, "", nil).Body.String(), "")
		if regexp.MustCompile(`<script(?:\s[^>]*)?>\s*[^<\s]`).MatchString(body) || regexp.MustCompile(`\son(?:click|submit|load|focus|change|input)=`).MatchString(body) {
			t.Errorf("%s memuat script/handler inline", p)
		}
	}

	// 404 HTML bergaya, JSON bila diminta, event untuk htmx.
	rec = request(e, http.MethodGet, "/tidak-ada", "", map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Halaman tidak ditemukan") || !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Errorf("404 HTML: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = request(e, http.MethodGet, "/tidak-ada", "", map[string]string{"Accept": "application/json"})
	if !strings.Contains(rec.Body.String(), `"message"`) {
		t.Errorf("404 JSON: %s", rec.Body.String())
	}
	// htmx: teks pendek (ditampilkan sebagai toast oleh lovoria.js), bukan halaman HTML.
	rec = request(e, http.MethodGet, "/tidak-ada", "", map[string]string{"HX-Request": "true"})
	if b := rec.Body.String(); rec.Code != http.StatusNotFound || strings.Contains(b, "<") || !strings.Contains(b, "tidak ada") {
		t.Errorf("404 htmx: %d %q", rec.Code, b)
	}
	for _, p := range []string{"/privacy", "/terms"} {
		if rec := request(e, http.MethodGet, p, "", nil); rec.Code != http.StatusOK {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
}

// 5xx: pesan internal tidak bocor, pelapor error (Sentry) dipanggil.
func TestInternalErrorNotLeaked(t *testing.T) {
	e := server.New(config.Config{Env: config.EnvTest}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var reported error
	server.ReportErrors(e, func(_ *http.Request, err error) { reported = err })
	e.GET("/boom", func(echo.Context) error { return errors.New("pq: password authentication failed for user railway") })
	rec := request(e, http.MethodGet, "/boom", "", map[string]string{"Accept": "text/html"})
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "password authentication") || !strings.Contains(rec.Body.String(), "Terjadi kesalahan") {
		t.Errorf("500: %d %s", rec.Code, rec.Body.String())
	}
	if reported == nil {
		t.Error("error 5xx harus dilaporkan")
	}
	reported = nil
	e.GET("/nf", func(echo.Context) error { return echo.NewHTTPError(http.StatusNotFound) })
	request(e, http.MethodGet, "/nf", "", nil)
	if reported != nil {
		t.Error("4xx tidak perlu dilaporkan")
	}
}
