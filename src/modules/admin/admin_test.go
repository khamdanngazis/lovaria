package admin

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/domain"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/storage"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

type fixture struct {
	e        *echo.Echo
	pool     *pgxpool.Pool
	svc      *Service
	auth     *auth.Service
	weddings *wedding.Service
	themes   *theme.Service
	gallery  *gallery.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, _ := config.LoadFrom(func(k string) string { return map[string]string{"APP_ENV": config.EnvTest}[k] })
	store, _ := storage.NewLocal(t.TempDir(), "/media")
	as := auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log)
	ws := wedding.NewService(wedding.NewRepository(pool))
	gs := gallery.NewService(gallery.NewRepository(pool), store, ws, 500<<20, log)
	ts := theme.NewService(pool, ws)
	svc := NewService(Deps{
		Pool: pool, Users: as, Weddings: ws, Guests: guest.NewService(guest.NewRepository(pool), "http://x"),
		Gallery: gs, Domains: domain.NewService(pool, nil, domain.Config{}, log), Themes: ts,
		Secret: []byte("rahasia-test-rahasia-test-rahasia-test"), Log: log,
	})
	ws.SetAdminAccess(svc)
	ws.SetArchiveDaysSource(svc.ArchiveDays)
	gs.SetQuotaSource(svc.QuotaBytes)

	e := server.New(cfg, log)
	mw := auth.NewMiddleware(as, false, log)
	e.Use(mw.LoadSession)
	dash := e.Group("/dashboard", mw.RequireAuth)
	owned := wedding.Register(dash.Group("/weddings"), wedding.Deps{Service: ws})
	theme.Register(owned, theme.Deps{Service: ts})
	Register(e.Group("/admin", mw.RequireAuth, mw.RequireRole(auth.RoleAdmin)), svc)
	return fixture{e: e, pool: pool, svc: svc, auth: as, weddings: ws, themes: ts, gallery: gs}
}

// user membuat akun + sesi; mengembalikan user & token cookie.
func (f fixture) user(t *testing.T, email string, admin bool) (auth.User, string) {
	t.Helper()
	in := auth.RegisterInput{Name: "User " + email, Email: email, Password: "password123"}
	create := f.auth.Register
	if admin {
		create = f.auth.CreateAdmin
	}
	u, err := create(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	tok, _, err := f.auth.CreateSession(ctx, u.ID, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return u, tok
}

func (f fixture) wedding(t *testing.T, owner uuid.UUID) wedding.Wedding {
	t.Helper()
	w, err := f.weddings.CreateWedding(ctx, owner, wedding.CreateInput{GroomName: "Khamdan", BrideName: "Sarah", Title: "Pernikahan K & S", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

type reqOpt struct {
	form    url.Values
	cookies []*http.Cookie
}

func (f fixture) do(method, path, token string, o reqOpt) *httptest.ResponseRecorder {
	var body io.Reader
	if o.form != nil {
		body = strings.NewReader(o.form.Encode())
	}
	r := httptest.NewRequest(method, path, body)
	if o.form != nil {
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	}
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if token != "" {
		r.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: token})
	}
	for _, c := range o.cookies {
		r.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, r)
	return rec
}

func (f fixture) auditCount(t *testing.T) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM admin_audit_logs").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// adminRoutes: semua route yang terpasang di /admin (dari router, bukan daftar manual).
func (f fixture) adminRoutes() []*echo.Route {
	var out []*echo.Route
	for _, r := range f.e.Routes() {
		if r.Path == "/admin" || strings.HasPrefix(r.Path, "/admin/") {
			out = append(out, r)
		}
	}
	return out
}

func fill(path string) string {
	return regexp.MustCompile(`:[a-zA-Z]+`).ReplaceAllStringFunc(path, func(p string) string {
		if p == ":themeID" {
			return "elegant"
		}
		return uuid.New().String()
	})
}

func TestCoupleCannotAccessAdmin(t *testing.T) {
	f := newFixture(t)
	_, couple := f.user(t, "couple@example.com", false)
	_, admin := f.user(t, "admin@example.com", true)
	routes := f.adminRoutes()
	if len(routes) < 15 {
		t.Fatalf("hanya %d route admin terdaftar", len(routes))
	}
	before := f.auditCount(t)
	for _, r := range routes {
		path := fill(r.Path)
		if rec := f.do(r.Method, path, couple, reqOpt{form: url.Values{"x": {"1"}}}); rec.Code != http.StatusForbidden {
			t.Errorf("couple %s %s: %d, want 403", r.Method, path, rec.Code)
		}
		if rec := f.do(r.Method, path, "", reqOpt{form: url.Values{"x": {"1"}}}); rec.Code != http.StatusSeeOther && rec.Code != http.StatusFound {
			t.Errorf("anonim %s %s: %d, want redirect login", r.Method, path, rec.Code)
		}
	}
	if f.auditCount(t) != before {
		t.Error("request yang ditolak tidak boleh menulis audit")
	}
	// Admin bisa membuka semua halaman GET utama.
	for _, p := range []string{"/admin", "/admin/users", "/admin/weddings", "/admin/themes", "/admin/packages", "/admin/storage", "/admin/domains", "/admin/audit"} {
		if rec := f.do(http.MethodGet, p, admin, reqOpt{}); rec.Code != http.StatusOK {
			t.Errorf("admin GET %s: %d", p, rec.Code)
		}
	}
}

func TestEveryWriteIsAudited(t *testing.T) {
	f := newFixture(t)
	_, admin := f.user(t, "admin@example.com", true)
	cu, _ := f.user(t, "couple@example.com", false)
	w := f.wedding(t, cu.ID)
	// Wedding terbit (langsung di DB: checklist publikasi bukan fokus test ini).
	if _, err := f.pool.Exec(ctx, "UPDATE weddings SET status = 'published' WHERE id = $1", w.ID); err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		name, method, path string
		form               url.Values
		action             string
	}{
		{"buat paket", http.MethodPost, "/admin/packages", url.Values{"name": {"Premium"}, "storage_mb": {"2048"}, "archive_days": {"730"}, "price_display": {"Rp 199.000"}}, "package.create"},
		{"nonaktifkan user", http.MethodPost, "/admin/users/" + cu.ID.String() + "/disabled", url.Values{"disabled": {"1"}}, "user.disable"},
		{"aktifkan user", http.MethodPost, "/admin/users/" + cu.ID.String() + "/disabled", url.Values{"disabled": {"0"}}, "user.enable"},
		{"nonaktifkan tema", http.MethodPost, "/admin/themes/modern", url.Values{"enabled": {"0"}}, "theme.disable"},
		{"aktifkan tema", http.MethodPost, "/admin/themes/modern", url.Values{"enabled": {"1"}}, "theme.enable"},
		{"lihat dashboard", http.MethodPost, "/admin/weddings/" + w.ID.String() + "/view", url.Values{}, "wedding.view_as_couple"},
	}
	for _, s := range steps {
		before := f.auditCount(t)
		rec := f.do(s.method, s.path, admin, reqOpt{form: s.form})
		if rec.Code != http.StatusSeeOther || strings.Contains(rec.Header().Get("Location"), "err=") {
			t.Fatalf("%s: %d %s", s.name, rec.Code, rec.Header().Get("Location"))
		}
		es, _, _ := f.svc.AuditLog(ctx, "", 1)
		if f.auditCount(t) != before+1 || es[0].Action != s.action || es[0].AdminEmail != "admin@example.com" {
			t.Errorf("%s: audit %+v", s.name, es[0])
		}
	}
	ps, _ := f.svc.ListPackages(ctx)
	pkg := ps[0].ID.String()
	for _, s := range []struct {
		name, path, action string
		form               url.Values
	}{
		{"ubah paket", "/admin/packages/" + pkg, "package.update", url.Values{"name": {"Premium+"}, "storage_mb": {"4096"}, "archive_days": {"730"}}},
		{"pasang paket", "/admin/weddings/" + w.ID.String() + "/package", "wedding.package", url.Values{"package_id": {pkg}}},
		{"ubah status", "/admin/weddings/" + w.ID.String() + "/status", "wedding.status", url.Values{"status": {wedding.StatusDraft}}},
		{"lepas paket", "/admin/weddings/" + w.ID.String() + "/package", "wedding.package", url.Values{"package_id": {""}}},
		{"hapus paket", "/admin/packages/" + pkg + "/delete", "package.delete", url.Values{}},
	} {
		before := f.auditCount(t)
		rec := f.do(http.MethodPost, s.path, admin, reqOpt{form: s.form})
		es, _, _ := f.svc.AuditLog(ctx, "", 1)
		if rec.Code != http.StatusSeeOther || f.auditCount(t) != before+1 || es[0].Action != s.action {
			t.Errorf("%s: %d %s audit=%v", s.name, rec.Code, rec.Header().Get("Location"), es[0].Action)
		}
	}
	// Aksi yang gagal validasi tidak menulis audit.
	before := f.auditCount(t)
	rec := f.do(http.MethodPost, "/admin/packages", admin, reqOpt{form: url.Values{"name": {""}, "storage_mb": {"0"}}})
	if !strings.Contains(rec.Header().Get("Location"), "err=") || f.auditCount(t) != before {
		t.Error("validasi gagal: tidak boleh tercatat")
	}
}

func TestDisableUserBlocksLogin(t *testing.T) {
	f := newFixture(t)
	a, admin := f.user(t, "admin@example.com", true)
	cu, couple := f.user(t, "couple@example.com", false)
	w := f.wedding(t, cu.ID)
	if rec := f.do(http.MethodGet, w.DashboardURL(""), couple, reqOpt{}); rec.Code != http.StatusOK {
		t.Fatalf("sebelum: %d", rec.Code)
	}
	f.do(http.MethodPost, "/admin/users/"+cu.ID.String()+"/disabled", admin, reqOpt{form: url.Values{"disabled": {"1"}}})
	// Sesi lama tidak berlaku, login ditolak dengan pesan jelas.
	if rec := f.do(http.MethodGet, w.DashboardURL(""), couple, reqOpt{}); rec.Code == http.StatusOK {
		t.Error("sesi user nonaktif masih berlaku")
	}
	if _, err := f.auth.Authenticate(ctx, "couple@example.com", "password123"); !errors.Is(err, auth.ErrAccountDisabled) {
		t.Errorf("login: %v", err)
	}
	if _, err := f.auth.Authenticate(ctx, "couple@example.com", "salah-password"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("password salah tetap generik: %v", err)
	}
	// Admin tidak bisa menonaktifkan dirinya sendiri.
	rec := f.do(http.MethodPost, "/admin/users/"+a.ID.String()+"/disabled", admin, reqOpt{form: url.Values{"disabled": {"1"}}})
	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Error("menonaktifkan diri sendiri harus ditolak")
	}
}

func TestPackageControlsQuotaAndArchive(t *testing.T) {
	f := newFixture(t)
	_, admin := f.user(t, "admin@example.com", true)
	cu, _ := f.user(t, "couple@example.com", false)
	w := f.wedding(t, cu.ID)
	if u, _ := f.gallery.StorageUsage(ctx, w.ID); u.QuotaBytes != 500<<20 {
		t.Fatalf("default kuota: %d", u.QuotaBytes)
	}
	f.do(http.MethodPost, "/admin/packages", admin, reqOpt{form: url.Values{"name": {"Mini"}, "storage_mb": {"50"}, "archive_days": {"30"}}})
	ps, _ := f.svc.ListPackages(ctx)
	f.do(http.MethodPost, "/admin/weddings/"+w.ID.String()+"/package", admin, reqOpt{form: url.Values{"package_id": {ps[0].ID.String()}}})
	if u, _ := f.gallery.StorageUsage(ctx, w.ID); u.QuotaBytes != 50<<20 {
		t.Errorf("kuota paket: %d", u.QuotaBytes)
	}
	if days, ok, _ := f.svc.ArchiveDays(ctx, w.ID); !ok || days != 30 {
		t.Errorf("arsip paket: %d %v", days, ok)
	}
	// Paket yang masih dipakai tidak bisa dihapus.
	rec := f.do(http.MethodPost, "/admin/packages/"+ps[0].ID.String()+"/delete", admin, reqOpt{form: url.Values{}})
	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Error("hapus paket terpakai harus ditolak")
	}
}

func TestAdminStatusChange(t *testing.T) {
	f := newFixture(t)
	_, admin := f.user(t, "admin@example.com", true)
	cu, _ := f.user(t, "couple@example.com", false)
	w := f.wedding(t, cu.ID)
	f.pool.Exec(ctx, "UPDATE weddings SET status = 'archived' WHERE id = $1", w.ID) //nolint:errcheck
	body := f.do(http.MethodGet, "/admin/weddings/"+w.ID.String(), admin, reqOpt{}).Body.String()
	if !strings.Contains(body, "→ Kenangan") {
		t.Fatal("admin harus bisa memilih Diarsipkan → Kenangan")
	}
	f.do(http.MethodPost, "/admin/weddings/"+w.ID.String()+"/status", admin, reqOpt{form: url.Values{"status": {wedding.StatusMemory}}})
	got, _ := f.weddings.GetWedding(ctx, w.ID)
	h, _ := f.weddings.History(ctx, w.ID, 1)
	if !got.InMemory() || len(h) != 1 || h[0].Actor != wedding.ActorAdmin {
		t.Errorf("status %s, riwayat %+v", got.Status, h)
	}
	// Transisi ilegal ditolak dengan pesan.
	rec := f.do(http.MethodPost, "/admin/weddings/"+w.ID.String()+"/status", admin, reqOpt{form: url.Values{"status": {wedding.StatusWeddingDay}}})
	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Error("transisi ilegal harus ditolak")
	}
}

func TestReadOnlyView(t *testing.T) {
	f := newFixture(t)
	adminUser, admin := f.user(t, "admin@example.com", true)
	_, admin2 := f.user(t, "admin2@example.com", true)
	cu, couple := f.user(t, "couple@example.com", false)
	_, otherCouple := f.user(t, "other@example.com", false)
	w := f.wedding(t, cu.ID)
	other := f.wedding(t, cu.ID)
	dash := w.DashboardURL("")

	if rec := f.do(http.MethodGet, dash, admin, reqOpt{}); rec.Code != http.StatusNotFound {
		t.Fatalf("tanpa mode lihat: %d", rec.Code)
	}
	rec := f.do(http.MethodPost, "/admin/weddings/"+w.ID.String()+"/view", admin, reqOpt{form: url.Values{}})
	var view *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == viewCookie {
			view = c
		}
	}
	if view == nil || rec.Header().Get("Location") != dash {
		t.Fatalf("mulai: %d %v", rec.Code, rec.Header())
	}
	with := reqOpt{cookies: []*http.Cookie{view}}
	body := f.do(http.MethodGet, dash, admin, with).Body.String()
	if !strings.Contains(body, "Mode admin — lihat saja") || !strings.Contains(body, "Pernikahan K &amp; S") || !strings.Contains(body, `<fieldset disabled class="lv-readonly`) {
		t.Fatal("dashboard pasangan dengan banner lihat-saja")
	}
	if rec := f.do(http.MethodGet, w.DashboardURL("/theme"), admin, with); rec.Code != http.StatusOK {
		t.Errorf("halaman lain: %d", rec.Code)
	}
	// Tidak bisa mengubah apa pun.
	for _, r := range []struct{ method, path string }{
		{http.MethodPatch, w.DashboardURL("/info")},
		{http.MethodPatch, w.DashboardURL("/status")},
		{http.MethodPatch, w.DashboardURL("/theme")},
	} {
		if rec := f.do(r.method, r.path, admin, reqOpt{form: url.Values{"title": {"Dibajak"}, "theme_id": {"modern"}}, cookies: with.cookies}); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: %d, want 403", r.method, r.path, rec.Code)
		}
	}
	if got, _ := f.weddings.GetWedding(ctx, w.ID); got.Title != "Pernikahan K & S" || got.ThemeID != wedding.DefaultThemeID {
		t.Error("data berubah dalam mode lihat saja")
	}
	// Cookie hanya berlaku untuk wedding itu, admin itu, dan role admin.
	if rec := f.do(http.MethodGet, other.DashboardURL(""), admin, with); rec.Code != http.StatusNotFound {
		t.Errorf("wedding lain: %d", rec.Code)
	}
	if rec := f.do(http.MethodGet, dash, admin2, with); rec.Code != http.StatusNotFound {
		t.Errorf("admin lain: %d", rec.Code)
	}
	if rec := f.do(http.MethodGet, dash, otherCouple, with); rec.Code != http.StatusNotFound {
		t.Errorf("couple lain dengan cookie admin: %d", rec.Code)
	}
	forged := &http.Cookie{Name: viewCookie, Value: f.svc.viewToken(adminUser.ID, w.ID, time.Now().Add(time.Hour)) + "x"}
	if rec := f.do(http.MethodGet, dash, admin, reqOpt{cookies: []*http.Cookie{forged}}); rec.Code != http.StatusNotFound {
		t.Errorf("cookie palsu: %d", rec.Code)
	}
	expired := &http.Cookie{Name: viewCookie, Value: f.svc.viewToken(adminUser.ID, w.ID, time.Now().Add(-time.Minute))}
	if rec := f.do(http.MethodGet, dash, admin, reqOpt{cookies: []*http.Cookie{expired}}); rec.Code != http.StatusNotFound {
		t.Errorf("cookie kedaluwarsa: %d", rec.Code)
	}
	// Pemilik tetap bisa mengubah seperti biasa.
	if rec := f.do(http.MethodGet, dash, couple, reqOpt{}); rec.Code != http.StatusOK {
		t.Errorf("pemilik: %d", rec.Code)
	}
	// Keluar: cookie dihapus & tercatat.
	rec = f.do(http.MethodPost, "/admin/view/stop", admin, reqOpt{form: url.Values{}, cookies: with.cookies})
	es, _, _ := f.svc.AuditLog(ctx, w.ID.String(), 1)
	if rec.Header().Get("Location") != "/admin/weddings/"+w.ID.String() || es[0].Action != "wedding.view_end" {
		t.Errorf("keluar: %s %v", rec.Header().Get("Location"), es[0].Action)
	}
}

func TestThemeAvailability(t *testing.T) {
	f := newFixture(t)
	_, admin := f.user(t, "admin@example.com", true)
	cu, couple := f.user(t, "couple@example.com", false)
	w := f.wedding(t, cu.ID) // tema default: signature
	f.do(http.MethodPost, "/admin/themes/modern", admin, reqOpt{form: url.Values{"enabled": {"0"}}})
	f.do(http.MethodPost, "/admin/themes/signature", admin, reqOpt{form: url.Values{"enabled": {"0"}}})
	body := f.do(http.MethodGet, w.DashboardURL("/theme"), couple, reqOpt{}).Body.String()
	if strings.Contains(body, `value="modern"`) || !strings.Contains(body, `value="signature"`) {
		t.Error("tema nonaktif disembunyikan, kecuali yang sedang dipakai")
	}
	rec := f.do(http.MethodPatch, w.DashboardURL("/theme"), couple, reqOpt{form: url.Values{"theme_id": {"modern"}}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "tidak tersedia") {
		t.Errorf("pilih tema nonaktif: %d", rec.Code)
	}
	if rec := f.do(http.MethodPatch, w.DashboardURL("/theme"), couple, reqOpt{form: url.Values{"theme_id": {"signature"}}}); rec.Code != http.StatusSeeOther {
		t.Errorf("tema sendiri tetap bisa disimpan: %d", rec.Code)
	}
	counts := f.do(http.MethodGet, "/admin/themes", admin, reqOpt{}).Body.String()
	if !strings.Contains(counts, "1 wedding") {
		t.Error("jumlah pemakai tema")
	}
}

func TestWeddingListWith1000(t *testing.T) {
	f := newFixture(t)
	_, admin := f.user(t, "admin@example.com", true)
	cu, _ := f.user(t, "couple@example.com", false)
	// Seed cepat langsung ke tabel (data uji, bukan jalur aplikasi).
	if _, err := f.pool.Exec(ctx, `
		INSERT INTO weddings (id, owner_user_id, slug, title, wedding_date, status, storage_used_bytes)
		SELECT gen_random_uuid(), $1, 'w-' || i, 'Wedding ' || lpad(i::text, 4, '0'), DATE '2026-01-01' + (i % 365),
		       (ARRAY['draft','published','memory'])[1 + i % 3], (i * 7919 % 100000) * 1024
		FROM generate_series(1, 1000) i`, cu.ID); err != nil {
		t.Fatal(err)
	}
	f.do(http.MethodGet, "/admin/weddings", admin, reqOpt{}) // pemanasan
	// Waktu terbaik dari 3 percobaan: suite paralel berbagi satu Postgres, jadi
	// satu pengukuran tunggal bisa melambat karena beban paket test lain.
	var rec *httptest.ResponseRecorder
	took := time.Hour
	for i := 0; i < 3; i++ {
		start := time.Now()
		rec = f.do(http.MethodGet, "/admin/weddings?sort=storage&page=2", admin, reqOpt{})
		took = min(took, time.Since(start))
	}
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Halaman 2 dari 40") || strings.Count(body, "/admin/weddings/") < 25 {
		t.Fatalf("list: %d", rec.Code)
	}
	if took > 300*time.Millisecond {
		t.Errorf("list 1.000 wedding: %v (batas 300ms)", took)
	}
	t.Logf("list 1.000 wedding: %v", took)
	// "wedding 000" → 0001–0009; memory = i % 3 == 2 → hanya 0002, 0005, 0008
	// (muat dalam satu halaman, urutan acak tidak berpengaruh).
	rec = f.do(http.MethodGet, "/admin/weddings?status=memory&q=wedding+000", admin, reqOpt{})
	if b := rec.Body.String(); !strings.Contains(b, "Wedding 0002") || strings.Contains(b, "Wedding 0001") || strings.Contains(b, "Wedding 0003") {
		t.Error("filter status + cari")
	}
	if rec := f.do(http.MethodGet, "/admin/storage", admin, reqOpt{}); rec.Code != http.StatusOK || strings.Count(rec.Body.String(), "/admin/weddings/") != 20 {
		t.Errorf("storage top 20: %d", rec.Code)
	}
}

// Modul admin hanya menyentuh tabelnya sendiri; data modul lain lewat service.
func TestAdminQueriesOwnTablesOnly(t *testing.T) {
	own := map[string]bool{"packages": true, "wedding_packages": true, "admin_audit_logs": true}
	files, _ := filepath.Glob("db/queries/*.sql")
	if len(files) == 0 {
		t.Fatal("query admin tidak ditemukan")
	}
	// "UPDATE <tabel>" di awal pernyataan (bukan "DO UPDATE SET").
	re := regexp.MustCompile(`(?im)(?:\bfrom|\bjoin|\binto|^update)\s+([a-z_]+)`)
	for _, file := range files {
		b, _ := os.ReadFile(file)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			if !own[strings.ToLower(m[1])] {
				t.Errorf("%s menyentuh tabel %q milik modul lain", file, m[1])
			}
		}
	}
}

func TestPackageShowOnLanding(t *testing.T) {
	f := newFixture(t)
	_, admin := f.user(t, "admin@example.com", true)
	f.do(http.MethodPost, "/admin/packages", admin, reqOpt{form: url.Values{"name": {"Tersembunyi"}, "storage_mb": {"100"}, "archive_days": {"30"}}})
	f.do(http.MethodPost, "/admin/packages", admin, reqOpt{form: url.Values{"name": {"Premium"}, "storage_mb": {"1024"}, "archive_days": {"365"}, "price_display": {"Rp 149.000"}, "show_on_landing": {"1"}, "sort_order": {"2"}}})
	f.do(http.MethodPost, "/admin/packages", admin, reqOpt{form: url.Values{"name": {"Basic"}, "storage_mb": {"200"}, "archive_days": {"180"}, "show_on_landing": {"1"}, "sort_order": {"1"}}})
	ps, err := f.svc.LandingPackages(ctx)
	if err != nil || len(ps) != 2 || ps[0].Name != "Basic" || ps[1].Name != "Premium" || !ps[1].ShowOnLanding {
		t.Fatalf("landing: %+v %v", ps, err)
	}
	rec := f.do(http.MethodPost, "/admin/packages", admin, reqOpt{form: url.Values{"name": {"X"}, "storage_mb": {"1"}, "archive_days": {"1"}, "sort_order": {"-5"}}})
	if !strings.Contains(rec.Header().Get("Location"), "err=") {
		t.Error("urutan negatif harus ditolak")
	}
}
