package wedding

import (
	"context"
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
	Register(e.Group("/dashboard/weddings"), Deps{Service: f.svc, Themes: testThemes})
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

// testThemes: pilihan tema wizard untuk test (modul theme tidak diimpor di sini).
func testThemes(context.Context) ([]ThemeChoice, error) {
	return []ThemeChoice{
		{ID: "signature", Name: "Lunovia Signature", Featured: true, Thumb: "/static/img/themes/signature.webp", DemoURL: "/w/contoh-signature"},
		{ID: "elegant", Name: "Elegan"},
		{ID: "jawa", Name: "Javanese Heritage", Region: "Jawa"},
	}, nil
}

// T29: wizard dua langkah — pilih tema dulu, lalu nama & tanggal; wedding
// dibuat dengan tema pilihan dan mendarat di halaman pratinjau pertama.
func TestWizardEndToEnd(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")

	// Belum punya wedding → daftar mengarahkan ke wizard.
	if rec := req(e, owner, http.MethodGet, "/dashboard/weddings", nil, false); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/dashboard/weddings/new" {
		t.Fatalf("list kosong: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	// Langkah 1: galeri tema (koleksi utama + Koleksi Daerah), tanpa kolom nama.
	rec := req(e, owner, http.MethodGet, "/dashboard/weddings/new", nil, false)
	body := rec.Body.String()
	for _, want := range []string{"Pilih tema undangan", `name="theme_id" value="signature"`, `name="theme_id" value="jawa"`, "Koleksi Daerah", "Pilihan Lunovia", `href="/w/contoh-signature"`, "bayar hanya saat undangan diterbitkan"} {
		if !strings.Contains(body, want) {
			t.Errorf("langkah 1 tidak memuat %q", want)
		}
	}
	if rec.Code != http.StatusOK || strings.Contains(body, "Nama mempelai pria") {
		t.Fatalf("langkah 1: %d", rec.Code)
	}

	// Tanpa memilih tema / tema tak dikenal → 422, tetap di langkah 1.
	for _, v := range []url.Values{{}, {"theme_id": {"tidak-ada"}}} {
		if rec := req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/1", v, true); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Pilih salah satu tema") {
			t.Fatalf("langkah 1 invalid (%v): %d", v, rec.Code)
		}
	}
	// Tema dipilih → langkah 2: nama & tanggal, tema dibawa sebagai hidden field.
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/1", url.Values{"theme_id": {"jawa"}}, true)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `type="hidden" name="theme_id" value="jawa"`) || !strings.Contains(body, "Javanese Heritage") ||
		!strings.Contains(body, "Nama mempelai pria") || !strings.Contains(body, `name="wedding_date"`) || strings.Contains(body, `name="slug"`) || strings.Contains(body, `name="title"`) {
		t.Fatalf("langkah 2: %d %s", rec.Code, body)
	}
	if strings.Contains(body, "<html") {
		t.Error("htmx harus menerima fragment")
	}
	// "Ganti tema" kembali ke langkah 1 dengan pilihan & isian tetap.
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings/new/steps/2", url.Values{"theme_id": {"jawa"}, "groom_name": {"Samuel"}, "back": {"1"}}, true)
	if b := rec.Body.String(); !strings.Contains(b, `name="theme_id" value="jawa" checked`) || !strings.Contains(b, `type="hidden" name="groom_name" value="Samuel"`) {
		t.Errorf("kembali ke langkah 1 kehilangan nilai: %s", b)
	}

	// Data tidak valid → 422 di langkah 2 dengan pesan per kolom.
	all := url.Values{"theme_id": {"jawa"}, "groom_name": {""}, "bride_name": {"Sarah"}, "wedding_date": {"2026-13-40"}}
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings", all, true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Nama mempelai pria wajib diisi") || !strings.Contains(rec.Body.String(), `aria-current="step"`) {
		t.Fatalf("create invalid: %d", rec.Code)
	}
	if ws, _ := f.svc.ListWeddingsByOwner(ctx, owner); len(ws) != 0 {
		t.Error("wedding tidak boleh dibuat")
	}

	// Selesai → wedding draf dengan tema pilihan; judul & alamat otomatis.
	all.Set("groom_name", "Samuel Pratama")
	all.Set("wedding_date", "2026-12-12")
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings", all, true)
	loc := rec.Header().Get("HX-Redirect")
	if rec.Code != http.StatusOK || !strings.HasPrefix(loc, "/dashboard/weddings/") || !strings.HasSuffix(loc, "/start") {
		t.Fatalf("create: %d %q", rec.Code, loc)
	}
	ws, _ := f.svc.ListWeddingsByOwner(ctx, owner)
	if len(ws) != 1 || ws[0].ThemeID != "jawa" || ws[0].Title != "Pernikahan Samuel & Sarah" || ws[0].Slug != "samuel-sarah" || ws[0].Status != StatusDraft || ws[0].IsPaid() {
		t.Fatalf("wedding: %+v", ws)
	}
	// Halaman pratinjau pertama: iframe pratinjau + ajakan melengkapi; tanpa harga / tombol terbit.
	rec = req(e, owner, http.MethodGet, loc, nil, false)
	body = rec.Body.String()
	for _, want := range []string{"Undangan kalian sudah jadi", `<iframe src="` + strings.TrimSuffix(loc, "/start") + `/theme/preview"`, "Lanjut lengkapi undangan", "gratis sampai kalian siap"} {
		if !strings.Contains(body, want) {
			t.Errorf("halaman mulai tidak memuat %q", want)
		}
	}
	if rec.Code != http.StatusOK || strings.Contains(body, "Publikasikan") || strings.Contains(body, "Rp") {
		t.Errorf("halaman mulai: %d (tidak boleh menonjolkan publikasi/harga)", rec.Code)
	}
	// Halaman mulai milik wedding orang lain → 404.
	if rec := req(e, f.user(t, "b@example.com"), http.MethodGet, loc, nil, false); rec.Code != http.StatusNotFound {
		t.Errorf("start orang lain: %d", rec.Code)
	}
}

// T29: tema dari halaman tema di landing (?tema=<id>) langsung terpilih; tema
// tak dikenal diabaikan; tema tak sah saat membuat → tema bawaan.
func TestWizardPreselectedTheme(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")

	rec := req(e, owner, http.MethodGet, "/dashboard/weddings/new?tema=elegant", nil, false)
	if b := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(b, `type="hidden" name="theme_id" value="elegant"`) || !strings.Contains(b, "Nama mempelai pria") || !strings.Contains(b, "Ganti tema") {
		t.Fatalf("tema terpilih: %d", rec.Code)
	}
	if b := req(e, owner, http.MethodGet, "/dashboard/weddings/new?tema=tidak-ada", nil, false).Body.String(); !strings.Contains(b, "Pilih tema undangan") {
		t.Error("tema tak dikenal harus kembali ke langkah 1")
	}
	rec = req(e, owner, http.MethodPost, "/dashboard/weddings", url.Values{"theme_id": {"palsu"}, "groom_name": {"Azis"}, "bride_name": {"Ida"}, "wedding_date": {"2027-03-06"}}, false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("buat: %d %s", rec.Code, rec.Body.String())
	}
	if ws, _ := f.svc.ListWeddingsByOwner(ctx, owner); len(ws) != 1 || ws[0].ThemeID != DefaultThemeID {
		t.Fatalf("tema tak sah harus jatuh ke bawaan: %+v", ws)
	}
}

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

	// Beranda draf (T29): mengajak pratinjau & melengkapi, tanpa status
	// pembayaran; tombol publikasi tersembunyi sampai syarat terbit terpenuhi.
	body := req(e, owner, http.MethodGet, base, nil, false).Body.String()
	for _, want := range []string{"Undangan kalian masih draf", "bayar hanya saat undangan diterbitkan", `href="` + base + `/theme/preview"`, "Lanjut lengkapi", "Tombol publikasi muncul setelah syarat"} {
		if !strings.Contains(body, want) {
			t.Errorf("beranda draf tidak memuat %q", want)
		}
	}
	if strings.Contains(body, "Belum dibayar") || strings.Contains(body, "Publikasikan undangan") || strings.Contains(body, `href="`+base+`/publish"`) || strings.Contains(body, "disabled") {
		t.Error("draf belum siap: tanpa status bayar & tanpa tombol publikasi (juga bukan tombol nonaktif)")
	}
	// Syarat terpenuhi, belum lunas (T23): tombol publikasi menuju halaman harga,
	// dan PATCH status dialihkan ke sana tanpa mengubah status.
	f.svc.SetEventCounter(countEvents(1))
	if body := req(e, owner, http.MethodGet, base, nil, false).Body.String(); !strings.Contains(body, `href="`+base+`/publish" class="ui-btn ui-btn-primary`) || !strings.Contains(body, "Publikasikan undangan") || strings.Contains(body, "Belum dibayar") {
		t.Error("draf siap & belum lunas: tombol publikasi ke halaman harga")
	}
	rec := req(e, owner, http.MethodPost, base+"/status", url.Values{"_method": {"PATCH"}, "status": {"published"}}, false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != base+"/publish" || f.status(t, w.ID) != StatusDraft {
		t.Fatalf("publish sebelum lunas: %d %s %s", rec.Code, rec.Header().Get("Location"), f.status(t, w.ID))
	}
	f.paid(t, w.ID)
	if body := req(e, owner, http.MethodGet, base, nil, false).Body.String(); !strings.Contains(body, "Lunas ✓") {
		t.Error("setelah lunas: badge Lunas")
	}

	// Checklist kurang (belum ada acara) → tombol publikasi disembunyikan (T29:
	// bukan dinonaktifkan) & PATCH ditolak 422.
	f.svc.SetEventCounter(countEvents(0))
	rec = req(e, owner, http.MethodGet, base, nil, false)
	if b := rec.Body.String(); !strings.Contains(b, "Minimal 1 acara") || strings.Contains(b, "Publikasikan undangan") || strings.Contains(b, `name="status" value="published"`) {
		t.Errorf("checklist kurang: syarat tampil, tombol publikasi tersembunyi")
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
	f.paid(t, w.ID)
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

// T29: selama draf, halaman isi undangan menampilkan langkah terpandu (bebas
// dilewati); setelah terbit panduan hilang. Halaman di luar panduan tidak
// menampilkannya.
func TestGuidedStepsOnDraft(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	base := "/dashboard/weddings/" + w.ID.String()

	body := req(e, owner, http.MethodGet, base+"/couple", nil, false).Body.String()
	for _, want := range []string{
		"Lengkapi undangan", "langkah 1 dari 7", `aria-current="step"`, `href="` + base + `/theme/preview"`,
		`href="` + base + `/events" class="ui-btn ui-btn-primary`, "Lanjut: Acara", "Boleh dilewati", `href="` + base + `/start"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("halaman mempelai (draf) tidak memuat %q", want)
		}
	}
	// Halaman di luar panduan (info) tanpa pita langkah.
	if b := req(e, owner, http.MethodGet, base+"/info", nil, false).Body.String(); strings.Contains(b, "Lengkapi undangan") {
		t.Error("halaman info bukan langkah panduan")
	}
	// Setelah terbit: panduan hilang.
	f.svc.SetEventCounter(countEvents(1))
	f.paid(t, w.ID)
	if _, err := f.svc.Transition(ctx, w.ID, StatusPublished, Actor{Kind: ActorUser, UserID: owner}); err != nil {
		t.Fatal(err)
	}
	if b := req(e, owner, http.MethodGet, base+"/couple", nil, false).Body.String(); strings.Contains(b, "Lengkapi undangan") || strings.Contains(b, "Lanjut: Acara") {
		t.Error("undangan terbit tidak menampilkan panduan")
	}
}

// Navigasi ringkas: enam menu utama; halaman di dalam menu tampil sebagai tab.
// Selama draf, menu/tab yang baru berguna setelah terbit disembunyikan, dan
// pita langkah menggantikan tab di halaman panduan.
func TestSimplerNavigation(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner := f.user(t, "a@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	base := "/dashboard/weddings/" + w.ID.String()
	aside := func(body string) string {
		i := strings.Index(body, `aria-label="Menu wedding"`)
		return body[i : i+strings.Index(body[i:], "</aside>")]
	}

	// Draf: lima menu (tanpa Ucapan); istilah lama tidak dipakai lagi.
	body := req(e, owner, http.MethodGet, base+"/info", nil, false).Body.String()
	side := aside(body)
	for _, want := range []string{"Beranda", "Isi undangan", "Tampilan", "Tamu", "Pengaturan", `href="` + base + `/couple"`, `href="` + base + `/info"`} {
		if !strings.Contains(side, want) {
			t.Errorf("sidebar draf tidak memuat %q", want)
		}
	}
	for _, gone := range []string{"Ucapan", "Info wedding", "Pasangan", "Konten undangan", "RSVP", "Bagikan", "Hadiah"} {
		if strings.Contains(side, gone) {
			t.Errorf("sidebar draf masih memuat %q", gone)
		}
	}
	if n := strings.Count(side, "<li>"); n != 5 {
		t.Errorf("menu sidebar draf = %d, want 5", n)
	}
	// Pengaturan: tab Judul & tanggal · Domain, tab aktif ditandai.
	if !strings.Contains(body, `aria-label="Bagian Pengaturan"`) || !strings.Contains(body, `href="`+base+`/domain"`) || !strings.Contains(body, `aria-current="page"`) {
		t.Error("halaman info: tab Pengaturan")
	}
	// Halaman panduan (draf): pita langkah, bukan tab.
	if b := req(e, owner, http.MethodGet, base+"/couple", nil, false).Body.String(); !strings.Contains(b, "Lengkapi undangan") || strings.Contains(b, `aria-label="Bagian Isi undangan"`) {
		t.Error("draf: halaman mempelai memakai pita langkah, bukan tab")
	}

	// Terbit: enam menu; Isi undangan & Tamu bertab lengkap.
	f.svc.SetEventCounter(countEvents(1))
	f.paid(t, w.ID)
	if _, err := f.svc.Transition(ctx, w.ID, StatusPublished, Actor{Kind: ActorUser, UserID: owner}); err != nil {
		t.Fatal(err)
	}
	body = req(e, owner, http.MethodGet, base+"/couple", nil, false).Body.String()
	if side := aside(body); strings.Count(side, "<li>") != 6 || !strings.Contains(side, "Ucapan") {
		t.Errorf("sidebar terbit harus 6 menu termasuk Ucapan")
	}
	for _, want := range []string{`aria-label="Bagian Isi undangan"`, ">Mempelai</a>", ">Acara</a>", ">Cerita</a>", ">Galeri</a>", ">Hadiah</a>"} {
		if !strings.Contains(body, want) {
			t.Errorf("tab Isi undangan tidak memuat %q", want)
		}
	}
	if strings.Contains(body, "Lengkapi undangan") {
		t.Error("terbit: tanpa pita langkah")
	}
}
