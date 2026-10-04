package wedding

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// newTestServer merakit stack HTTP; user login ditentukan header X-Test-User
// (pengganti session auth, yang sudah diuji di modul auth).
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
				c.SetRequest(r.WithContext(web.WithUser(r.Context(), web.User{ID: id, Name: "Test", Role: "couple"})))
			}
			return next(c)
		}
	})
	Register(e.Group("/dashboard/weddings"), Deps{Service: f.svc})
	return e
}

// req mengirim request sebagai user; Sec-Fetch-Site same-origin meniru browser (lolos CSRF).
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

func TestWizardEndToEnd(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")

	// Belum punya wedding → daftar mengarahkan ke wizard.
	if rec := req(e, owner, http.MethodGet, "/dashboard/weddings", nil, false); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard/weddings/new" {
		t.Fatalf("list kosong: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := req(e, owner, http.MethodGet, "/dashboard/weddings/new", nil, false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Nama mempelai pria") {
		t.Fatalf("langkah 1: %d", rec.Code)
	}

	// Langkah 1 dengan nama kosong → 422, tetap di langkah 1.
	rec := req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/1", url.Values{"groom_name": {""}, "bride_name": {"Sarah"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Nama mempelai pria wajib diisi") {
		t.Fatalf("langkah 1 invalid: %d", rec.Code)
	}

	// Langkah 1 valid → langkah 2, judul otomatis terisi, nama dibawa sebagai hidden field.
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/1", url.Values{"groom_name": {"Samuel"}, "bride_name": {"Sarah"}}, true)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `value="Pernikahan Samuel &amp; Sarah"`) || !strings.Contains(body, `type="hidden" name="groom_name" value="Samuel"`) {
		t.Fatalf("langkah 2: %d %s", rec.Code, body)
	}
	if strings.Contains(body, "<html") {
		t.Error("htmx harus menerima fragment")
	}

	// Kembali dari langkah 2 → langkah 1 dengan nilai tetap.
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/2", url.Values{"groom_name": {"Samuel"}, "bride_name": {"Sarah"}, "back": {"1"}}, true)
	if !strings.Contains(rec.Body.String(), `id="wedding-groom-name" name="groom_name" type="text" value="Samuel"`) {
		t.Errorf("kembali ke langkah 1 kehilangan nilai: %s", rec.Body.String())
	}

	// Langkah 2 tanggal tidak valid → 422.
	all := url.Values{"groom_name": {"Samuel"}, "bride_name": {"Sarah"}, "title": {"Pernikahan Kami"}, "wedding_date": {"2026-13-40"}}
	if rec := req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/2", all, true); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("tanggal invalid: %d", rec.Code)
	}
	all.Set("wedding_date", "2026-12-12")
	if rec := req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/2", all, true); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Deskripsi singkat") {
		t.Fatalf("langkah 3: %d", rec.Code)
	}

	// Selesai → wedding dibuat, redirect ke halaman wedding.
	all.Set("description", "Dengan penuh syukur")
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings", all, true)
	loc := rec.Header().Get("HX-Redirect")
	if rec.Code != http.StatusOK || !strings.HasPrefix(loc, "/dashboard/weddings/") || !strings.HasSuffix(loc, "?welcome=1") {
		t.Fatalf("create: %d %q", rec.Code, loc)
	}
	rec = req(e, owner, http.MethodGet, loc, nil, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Samuel &amp; Sarah") || !strings.Contains(rec.Body.String(), "Sabtu, 12 Desember 2026") {
		t.Errorf("overview: %d", rec.Code)
	}
}

func TestCreateWithInvalidDataReturnsToFirstBadStep(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")
	rec := req(e, owner, http.MethodPost, "/dashboard/weddings", url.Values{"groom_name": {""}, "bride_name": {"S"}, "title": {""}, "wedding_date": {"2026-12-12"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `aria-current="step"`) || !strings.Contains(rec.Body.String(), "Nama mempelai pria wajib diisi") {
		t.Errorf("status %d body %s", rec.Code, rec.Body.String())
	}
}

// User A tidak bisa mengakses/mengubah wedding milik user B → 404.
func TestOtherUsersWeddingIs404(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	alice, bob := f.user(t, "alice@example.com"), f.user(t, "bob@example.com")
	w, err := f.svc.CreateWedding(ctx, alice, validInput())
	if err != nil {
		t.Fatal(err)
	}
	base := "/dashboard/weddings/" + w.ID.String()

	for _, p := range []string{base, base + "/info", base + "/couple", base + "/tidak-ada"} {
		if rec := req(e, bob, http.MethodGet, p, nil, false); rec.Code != http.StatusNotFound {
			t.Errorf("bob GET %s: %d, want 404", p, rec.Code)
		}
	}
	// PATCH via htmx dan via form biasa (_method override).
	if rec := req(e, bob, http.MethodPatch, base+"/info", url.Values{"title": {"Dibajak"}, "wedding_date": {"2026-01-01"}}, true); rec.Code != http.StatusNotFound {
		t.Errorf("bob PATCH info: %d", rec.Code)
	}
	if rec := req(e, bob, http.MethodPost, base+"/couple", url.Values{"_method": {"PATCH"}, "groom_name": {"X"}, "bride_name": {"Y"}}, false); rec.Code != http.StatusNotFound {
		t.Errorf("bob POST _method=PATCH couple: %d", rec.Code)
	}
	got, _ := f.svc.GetWedding(ctx, w.ID)
	if got.Title != validInput().Title {
		t.Fatal("wedding alice berubah oleh bob!")
	}
	c, _ := f.svc.GetCouple(ctx, w.ID)
	if c.GroomName != "Samuel" {
		t.Fatal("couple alice berubah oleh bob!")
	}

	// ID tidak valid / tidak ada → 404 juga.
	for _, p := range []string{"/dashboard/weddings/bukan-uuid", "/dashboard/weddings/" + uuid.New().String()} {
		if rec := req(e, alice, http.MethodGet, p, nil, false); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	// Pemilik tetap bisa.
	if rec := req(e, alice, http.MethodGet, base+"/info", nil, false); rec.Code != http.StatusOK {
		t.Errorf("alice: %d", rec.Code)
	}
}

func TestUpdateInfoAndCoupleForms(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	base := "/dashboard/weddings/" + w.ID.String()

	rec := req(e, owner, http.MethodPatch, base+"/info", url.Values{"title": {""}, "wedding_date": {"x"}}, true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Judul wajib diisi") {
		t.Errorf("info invalid: %d", rec.Code)
	}
	rec = req(e, owner, http.MethodPatch, base+"/info", url.Values{"title": {"Baru"}, "wedding_date": {"2027-05-05"}, "description": {"d"}}, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Perubahan tersimpan.") {
		t.Errorf("info ok: %d", rec.Code)
	}
	// Form tanpa JS: POST + _method=PATCH.
	rec = req(e, owner, http.MethodPost, base+"/couple", url.Values{"_method": {"PATCH"}, "groom_name": {"Budi"}, "bride_name": {"Ani"}}, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html") || !strings.Contains(rec.Body.String(), "Perubahan tersimpan.") {
		t.Errorf("couple no-JS: %d", rec.Code)
	}
	if c, _ := f.svc.GetCouple(ctx, w.ID); c.GroomName != "Budi" {
		t.Errorf("couple tidak tersimpan: %+v", c)
	}
}

func TestPublishUnpublishViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, bob := f.user(t, "a@example.com"), f.user(t, "bob@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	base := "/dashboard/weddings/" + w.ID.String()

	// Checklist kurang (belum ada acara) → tombol nonaktif & PATCH ditolak 422.
	f.svc.SetEventCounter(countEvents(0))
	rec := req(e, owner, http.MethodGet, base, nil, false)
	if !strings.Contains(rec.Body.String(), "Minimal 1 acara") || !strings.Contains(rec.Body.String(), "disabled") {
		t.Errorf("checklist tidak tampil")
	}
	rec = req(e, owner, http.MethodPost, base+"/status", url.Values{"_method": {"PATCH"}, "status": {"published"}}, false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "belum bisa dipublikasikan: Minimal 1 acara") {
		t.Fatalf("publish tanpa acara: %d", rec.Code)
	}

	f.svc.SetEventCounter(countEvents(1))
	// User lain → 404, status tidak berubah.
	if rec := req(e, bob, http.MethodPatch, base+"/status", url.Values{"status": {"published"}}, false); rec.Code != http.StatusNotFound {
		t.Errorf("bob publish: %d", rec.Code)
	}
	rec = req(e, owner, http.MethodPost, base+"/status", url.Values{"_method": {"PATCH"}, "status": {"published"}}, false)
	if rec.Code != http.StatusSeeOther || f.status(t, w.ID) != StatusPublished {
		t.Fatalf("publish: %d %s", rec.Code, f.status(t, w.ID))
	}
	if rec := req(e, owner, http.MethodGet, rec.Header().Get("Location"), nil, false); !strings.Contains(rec.Body.String(), "Undangan dipublikasikan") || !strings.Contains(rec.Body.String(), "Tarik publikasi") {
		t.Error("notice / tombol tarik publikasi tidak tampil")
	}
	// Transisi ilegal via HTTP → 422 dengan pesan jelas.
	rec = req(e, owner, http.MethodPatch, base+"/status", url.Values{"status": {"archived"}}, false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "tidak bisa diubah dari Terbit ke Diarsipkan") {
		t.Errorf("ilegal: %d", rec.Code)
	}
	if rec := req(e, owner, http.MethodPatch, base+"/status", url.Values{"status": {"draft"}}, false); rec.Code != http.StatusSeeOther || f.status(t, w.ID) != StatusDraft {
		t.Errorf("unpublish: %d %s", rec.Code, f.status(t, w.ID))
	}
	if h, _ := f.svc.History(ctx, w.ID, 10); len(h) != 2 {
		t.Errorf("riwayat = %d", len(h))
	}
}

func TestPublishOnOrAfterWeddingDayAdvancesImmediately(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	f.svc.SetEventCounter(countEvents(1))
	owner := f.user(t, "a@example.com")
	in := validInput()
	in.WeddingDate = "2026-01-10" // sudah lewat
	w, _ := f.svc.CreateWedding(ctx, owner, in)
	f.svc.now = func() time.Time { return time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) }

	req(e, owner, http.MethodPatch, "/dashboard/weddings/"+w.ID.String()+"/status", url.Values{"status": {"published"}}, false)
	if got := f.status(t, w.ID); got != StatusMemory {
		t.Errorf("status = %s, want memory (hari H sudah lewat)", got)
	}
}

func TestArchiveVisibilityViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, bob := f.user(t, "a@example.com"), f.user(t, "bob@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	base := "/dashboard/weddings/" + w.ID.String()

	// Sebelum hari H pilihan visibilitas belum tampil.
	if strings.Contains(req(e, owner, http.MethodGet, base, nil, false).Body.String(), "Visibilitas arsip") {
		t.Error("draft: pilihan visibilitas arsip tidak perlu tampil")
	}
	if _, err := f.svc.repo.pool.Exec(ctx, `UPDATE weddings SET status = 'memory' WHERE id = $1`, w.ID); err != nil {
		t.Fatal(err)
	}
	if body := req(e, owner, http.MethodGet, base, nil, false).Body.String(); !strings.Contains(body, "Visibilitas arsip") || !strings.Contains(body, "halaman kenangan") {
		t.Fatal("kenangan: pilihan visibilitas arsip harus tampil")
	}
	rec := req(e, owner, http.MethodPost, base+"/archive-visibility", url.Values{"_method": {"PATCH"}, "visibility": {ArchivePrivate}}, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("simpan: %d", rec.Code)
	}
	if got, _ := f.svc.GetWedding(ctx, w.ID); got.ArchiveVisibility != ArchivePrivate {
		t.Errorf("visibilitas = %q", got.ArchiveVisibility)
	}
	if body := req(e, owner, http.MethodGet, rec.Header().Get("Location"), nil, false).Body.String(); !strings.Contains(body, "Visibilitas arsip disimpan") {
		t.Error("notice simpan tidak tampil")
	}
	if rec := req(e, owner, http.MethodPatch, base+"/archive-visibility", url.Values{"visibility": {"semua"}}, false); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("nilai tak dikenal: %d", rec.Code)
	}
	if rec := req(e, bob, http.MethodPatch, base+"/archive-visibility", url.Values{"visibility": {ArchivePublicVisibility}}, false); rec.Code != http.StatusNotFound {
		t.Errorf("bob: %d", rec.Code)
	}
	if got, _ := f.svc.GetWedding(ctx, w.ID); got.ArchiveVisibility != ArchivePrivate || got.ArchivePublic() {
		t.Errorf("setelah percobaan bob: %q", got.ArchiveVisibility)
	}
}
