package publicsite

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/gift"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
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
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

const testSecret = "rahasia-test-rahasia-test-rahasia-test"

var testNow = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// fakeDomains memetakan host → wedding (pengganti custom domain T15).
type fakeDomains map[string]uuid.UUID

func (f fakeDomains) WeddingIDByHost(_ context.Context, host string) (uuid.UUID, bool, error) {
	id, ok := f[host]
	return id, ok, nil
}

// ActiveDomain (wedding.DomainSource): domain aktif milik wedding.
func (f fakeDomains) ActiveDomain(_ context.Context, weddingID uuid.UUID) (string, bool, error) {
	for host, id := range f {
		if id == weddingID {
			return host, true, nil
		}
	}
	return "", false, nil
}

type fixture struct {
	e         *echo.Echo
	views     *ViewBuilder
	pkgs      *fakePackages
	themes    *theme.Service
	weddings  *wedding.Service
	guests    *guest.Service
	events    *event.Service
	guestbook *guestbook.Service
	gifts     *gift.Service
	auth      *auth.Service
	domains   fakeDomains
	publish   func(uuid.UUID)
	setStatus func(uuid.UUID, string)
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, _ := config.LoadFrom(func(k string) string {
		return map[string]string{"APP_ENV": config.EnvTest, "BASE_URL": "https://lovoria.test"}[k]
	})
	store, _ := storage.NewLocal(t.TempDir(), "/media")
	ws := wedding.NewService(wedding.NewRepository(pool))
	gs := guest.NewService(guest.NewRepository(pool), cfg.BaseURL)
	evs := event.NewService(event.NewRepository(pool))
	gb := guestbook.NewService(pool, guestbook.NewWordFilter(guestbook.DefaultBlockedWords))
	gf := gift.NewService(pool)
	views := &ViewBuilder{
		Weddings: ws, Events: evs, Stories: story.NewService(story.NewRepository(pool)),
		Gallery: gallery.NewService(gallery.NewRepository(pool), store, ws, 500<<20, log), Themes: theme.NewService(pool, ws),
		Guestbook: gb, Gifts: gf,
		CacheTTL: -1, // test lama menguji isi halaman; perilaku cache diuji di TestPublicViewCache
	}
	domains := fakeDomains{}
	pkgs := &fakePackages{}
	ws.SetDomains(domains, cfg.BaseURL)

	e := server.New(cfg, log)
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc { // user login dari header (pengganti session)
		return func(c echo.Context) error {
			if id, err := uuid.Parse(c.Request().Header.Get("X-Test-User")); err == nil {
				r := c.Request()
				c.SetRequest(r.WithContext(web.WithUser(r.Context(), web.User{ID: id})))
			}
			return next(c)
		}
	})
	Register(e, Deps{
		Resolver:       &Resolver{Weddings: ws, Guests: gs, Domains: domains, BaseURL: cfg.BaseURL, ExtraHosts: []string{"lovaria.up.railway.app"}, HostHeader: "X-Forwarded-Host", Log: log},
		Handler:        &Handler{BaseURL: cfg.BaseURL, Packages: pkgs, Views: views, Guests: gs, Guestbook: gb, Events: evs, Log: log, Secret: []byte(testSecret), now: func() time.Time { return testNow }},
		RSVPLimit:      RSVPLimit{PerMinute: 60, Burst: 8},
		GuestbookLimit: GuestbookLimit{PerMinute: 60, Burst: 8},
	})
	return fixture{
		e: e, views: views, pkgs: pkgs, themes: views.Themes, weddings: ws, guests: gs, events: evs, domains: domains, guestbook: gb, gifts: gf,
		auth: auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
		publish: func(id uuid.UUID) {
			if _, err := pool.Exec(ctx, `UPDATE weddings SET status = 'published' WHERE id = $1`, id); err != nil {
				t.Fatal(err)
			}
		},
		setStatus: func(id uuid.UUID, status string) {
			if _, err := pool.Exec(ctx, `UPDATE weddings SET status = $2 WHERE id = $1`, id, status); err != nil {
				t.Fatal(err)
			}
		},
	}
}

func (f fixture) newWedding(t *testing.T, email, groom, bride string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: groom, BrideName: bride, Title: "T", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func (f fixture) get(path string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Host = "lovoria.test"
	for k, v := range hdr {
		if k == "Host" {
			r.Host = v
			continue
		}
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, r)
	return rec
}

func TestDraftGate(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})

	for _, p := range []string{"/w/" + w.Slug, "/i/" + g.InvitationCode} {
		rec := f.get(p, nil)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Undangan tidak ditemukan") {
			t.Errorf("draft publik %s: %d", p, rec.Code)
		}
	}
	// Owner melihat preview.
	rec := f.get("/w/"+w.Slug, map[string]string{"X-Test-User": owner.String()})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Preview") || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("owner preview: %d %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	// Owner membuka link tamu: tidak dihitung sebagai "dibuka".
	f.get("/i/"+g.InvitationCode, map[string]string{"X-Test-User": owner.String()})
	if got, _ := f.guests.Get(ctx, w.ID, g.ID); got.LastOpenedAt != nil {
		t.Error("preview owner tidak boleh menandai tamu membuka undangan")
	}
	// User lain yang login tetap 404.
	if rec := f.get("/w/"+w.Slug, map[string]string{"X-Test-User": uuid.NewString()}); rec.Code != http.StatusNotFound {
		t.Errorf("user lain: %d", rec.Code)
	}
}

func TestPublishedInvitation(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan Ngazis", "Sarah Putri")
	f.publish(w.ID)
	_, _ = f.events.CreateEvent(ctx, w.ID, event.Input{Name: "Akad Nikah", Type: event.TypeAkad, Date: "2026-12-12", StartTime: "08:00", Venue: "Masjid Istiqlal"})
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Bapak Budi & Keluarga"})

	// Umum (/w/:slug).
	rec := f.get("/w/"+w.Slug, nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("/w: %d", rec.Code)
	}
	for _, want := range []string{
		`<meta property="og:title" content="The Wedding of Khamdan &amp; Sarah">`,
		`<meta property="og:description" content="Sabtu, 12 Desember 2026 · Masjid Istiqlal">`,
		`<meta property="og:url" content="https://lovoria.test/w/` + w.Slug + `">`,
		"Tamu Undangan", "Akad Nikah", "/w/" + w.Slug + "/events/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("/w tidak memuat %q", want)
		}
	}
	if strings.Contains(body, "Preview") {
		t.Error("halaman publik tidak boleh berlabel Preview")
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=60" || rec.Header().Get("X-Robots-Tag") != "noindex" || rec.Header().Get("ETag") == "" {
		t.Errorf("header: %v", rec.Header())
	}
	// HTML yang di-cache bersama tidak boleh berisi token CSRF per pengunjung.
	if strings.Contains(body, "csrf-token") || strings.Contains(body, "X-CSRF-Token") {
		t.Error("halaman undangan publik memuat token CSRF")
	}
	// ETag → 304.
	if rec2 := f.get("/w/"+w.Slug, map[string]string{"If-None-Match": rec.Header().Get("ETag")}); rec2.Code != http.StatusNotModified {
		t.Errorf("If-None-Match: %d", rec2.Code)
	}

	// Personal (/i/:code), kode huruf kecil tetap dikenali.
	rec = f.get("/i/"+strings.ToLower(g.InvitationCode), nil)
	body = rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Bapak Budi &amp; Keluarga") || !strings.Contains(body, "Kepada Yth. Bapak Budi &amp; Keluarga — Sabtu") {
		t.Fatalf("/i: %d", rec.Code)
	}
	if !strings.Contains(body, `og:url" content="https://lovoria.test/i/`+g.InvitationCode+`"`) {
		t.Error("og:url personal harus memakai kode kanonik")
	}
	if got, _ := f.guests.Get(ctx, w.ID, g.ID); got.LastOpenedAt == nil {
		t.Error("MarkOpened tidak tercatat")
	}
}

func TestNotFoundAndLanding(t *testing.T) {
	f := newFixture(t)
	for _, p := range []string{"/i/AAAAAAA", "/i/salah", "/w/tidak-ada", "/events/" + uuid.NewString() + ".ics"} {
		rec := f.get(p, nil)
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Undangan tidak ditemukan") {
			t.Errorf("%s: %d", p, rec.Code)
		}
	}
	if rec := f.get("/", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Your Forever.") {
		t.Errorf("landing: %d", rec.Code)
	}
}

func TestCalendarICS(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	_, other := f.newWedding(t, "b@example.com", "Andi", "Rina")
	f.publish(w.ID)
	f.publish(other.ID)
	ev, _ := f.events.CreateEvent(ctx, w.ID, event.Input{Name: "Akad Nikah", Type: event.TypeAkad, Date: "2026-12-12", StartTime: "08:00", EndTime: "10:00", Venue: "Masjid", Address: "Jl. A, No. 1"})
	evOther, _ := f.events.CreateEvent(ctx, other.ID, event.Input{Name: "Resepsi", Date: "2026-12-13", StartTime: "11:00", Venue: "Gedung"})

	rec := f.get("/w/"+w.Slug+"/events/"+ev.ID.String()+".ics", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/calendar") {
		t.Fatalf("ics: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{"BEGIN:VCALENDAR", "DTSTART:20261212T010000Z", "DTEND:20261212T030000Z", `SUMMARY:Akad Nikah — Khamdan & Sarah`, `LOCATION:Masjid\, Jl. A\, No. 1`, "\r\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("ics tidak memuat %q:\n%s", want, body)
		}
	}
	// Acara wedding lain lewat URL wedding ini → 404.
	if rec := f.get("/w/"+w.Slug+"/events/"+evOther.ID.String()+".ics", nil); rec.Code != http.StatusNotFound {
		t.Errorf("acara wedding lain: %d", rec.Code)
	}
}

func TestCustomDomain(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	_, other := f.newWedding(t, "b@example.com", "Andi", "Rina")
	f.publish(w.ID)
	f.publish(other.ID)
	f.domains["khamdansarah.com"] = w.ID
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	gOther, _ := f.guests.Create(ctx, other.ID, guest.Input{Name: "Cici"})

	host := map[string]string{"Host": "khamdansarah.com"}
	rec := f.get("/", host)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Khamdan") || !strings.Contains(rec.Body.String(), `og:url" content="https://khamdansarah.com"`) {
		t.Fatalf("custom domain /: %d", rec.Code)
	}
	if rec := f.get("/i/"+g.InvitationCode, host); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Budi") {
		t.Errorf("custom domain /i: %d", rec.Code)
	}
	// Kode tamu wedding lain di custom domain ini → 404.
	if rec := f.get("/i/"+gOther.InvitationCode, host); rec.Code != http.StatusNotFound {
		t.Errorf("kode wedding lain di custom domain: %d", rec.Code)
	}
	// Host tak dikenal → 404 generik, tidak pernah wedding lain lewat path (T15).
	for _, p := range []string{"/", "/w/" + other.Slug, "/i/" + gOther.InvitationCode} {
		rec := f.get(p, map[string]string{"Host": "unknown.example"})
		if body := rec.Body.String(); rec.Code != http.StatusNotFound || strings.Contains(body, "Andi") || strings.Contains(body, "Khamdan") {
			t.Errorf("host tak dikenal %s: %d", p, rec.Code)
		}
	}
}

func TestMemoryAndArchivedPages(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})

	f.setStatus(w.ID, wedding.StatusMemory)
	rec := f.get("/i/"+g.InvitationCode, nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Terima kasih telah menjadi bagian dari hari kami") {
		t.Errorf("kenangan: %d", rec.Code)
	}

	f.setStatus(w.ID, wedding.StatusArchived)
	for _, p := range []string{"/w/" + w.Slug, "/i/" + g.InvitationCode} {
		rec = f.get(p, nil)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || !strings.Contains(body, "Undangan ini telah diarsipkan") || strings.Contains(body, `id="events"`) {
			t.Errorf("arsip %s: %d", p, rec.Code)
		}
	}
}

func TestCustomDomainRedirectAndHostHeader(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	f.domains["www.khamdansarah.com"] = w.ID

	// /w/:slug di domain Lovoria → 301 ke custom domain (path & query ikut).
	rec := f.get("/w/"+w.Slug+"?ref=wa", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "https://www.khamdansarah.com?ref=wa" {
		t.Fatalf("redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	rec = f.get("/w/"+w.Slug+"/events/x.ics", nil)
	if rec.Header().Get("Location") != "https://www.khamdansarah.com/events/x.ics" {
		t.Errorf("redirect sub-path: %s", rec.Header().Get("Location"))
	}
	// Link tamu /i/:code di domain Lovoria tetap dilayani (tidak dialihkan).
	if rec := f.get("/i/"+g.InvitationCode, nil); rec.Code != http.StatusOK {
		t.Errorf("/i/:code: %d", rec.Code)
	}
	// POST ke /w/:slug/guestbook tidak dialihkan (301 mengubah POST jadi GET).
	if rec := f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Ani", "Selamat"), true, ""); rec.Code != http.StatusOK {
		t.Errorf("POST guestbook: %d", rec.Code)
	}
	// Draft: tidak dialihkan (pemilik tetap bisa preview di domain Lovoria).
	f.setStatus(w.ID, "draft")
	if rec := f.get("/w/"+w.Slug, map[string]string{"X-Test-User": owner.String()}); rec.Code != http.StatusOK {
		t.Errorf("draft preview: %d", rec.Code)
	}
	f.publish(w.ID)

	// Proxy (Cloudflare Worker) menulis ulang Host ke domain Railway dan mengirim
	// host asli lewat X-Forwarded-Host.
	proxied := map[string]string{"Host": "lovaria.up.railway.app", "X-Forwarded-Host": "www.khamdansarah.com"}
	if rec := f.get("/", proxied); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Khamdan") {
		t.Errorf("host header: %d", rec.Code)
	}
	// Domain Railway sendiri tetap host Lovoria (landing, bukan 404).
	if rec := f.get("/", map[string]string{"Host": "lovaria.up.railway.app"}); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Your Forever.") {
		t.Errorf("extra host: %d", rec.Code)
	}
	// Header berisi host tak dikenal → 404.
	if rec := f.get("/", map[string]string{"Host": "lovaria.up.railway.app", "X-Forwarded-Host": "evil.example"}); rec.Code != http.StatusNotFound {
		t.Errorf("header host tak dikenal: %d", rec.Code)
	}
}

func TestOldSlugRedirects(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	old := w.Slug
	if _, err := f.weddings.ChangeSlug(ctx, w.ID, "khamdan-sarah-baru"); err != nil {
		t.Fatal(err)
	}
	rec := f.get("/w/"+old+"?ref=wa", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/w/khamdan-sarah-baru?ref=wa" {
		t.Fatalf("slug lama: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := f.get("/w/"+old+"/events/x.ics", nil); rec.Header().Get("Location") != "/w/khamdan-sarah-baru/events/x.ics" {
		t.Errorf("sub-path: %s", rec.Header().Get("Location"))
	}
	// POST ke slug lama (buku ucapan) → 308 supaya tetap POST.
	if rec := f.post("/w/"+old+"/guestbook", gbForm(w.ID, "Ani", "Halo"), false, ""); rec.Code != http.StatusPermanentRedirect {
		t.Errorf("POST slug lama: %d", rec.Code)
	}
	if rec := f.get("/w/khamdan-sarah-baru", nil); rec.Code != http.StatusOK {
		t.Errorf("slug baru: %d", rec.Code)
	}
	if rec := f.get("/w/tidak-pernah-ada", nil); rec.Code != http.StatusNotFound {
		t.Errorf("slug tak dikenal: %d", rec.Code)
	}
}

// Regresi: isOwnHost dulu mengisi map secara lazy tanpa sinkronisasi → request
// paralel pertama setelah start membuat proses crash (concurrent map writes).
func TestResolverConcurrentFirstRequests(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rec := f.get("/w/"+w.Slug, nil); rec.Code != http.StatusOK {
				t.Errorf("status %d", rec.Code)
			}
		}()
	}
	wg.Wait()
}

// last_opened_at diperbarui paling sering tiap 15 menit per tamu (hemat UPDATE saat ramai).
func TestMarkOpenedThrottled(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	f.get("/i/"+g.InvitationCode, nil)
	first, _ := f.guests.Get(ctx, w.ID, g.ID)
	if first.LastOpenedAt == nil {
		t.Fatal("kunjungan pertama harus tercatat")
	}
	f.get("/i/"+g.InvitationCode, nil)
	if again, _ := f.guests.Get(ctx, w.ID, g.ID); !again.LastOpenedAt.Equal(*first.LastOpenedAt) {
		t.Error("kunjungan ulang < 15 menit tidak boleh menulis ulang")
	}
}

func TestPublicViewCache(t *testing.T) {
	f := newFixture(t)
	f.views.CacheTTL = time.Hour // cache aktif di test ini
	owner, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	gift1 := gift.Input{Type: gift.TypeBank, Provider: "BCA", AccountNumber: "1111111111", AccountName: "K"}

	f.get("/w/"+w.Slug, nil)         // isi cache
	f.gifts.Create(ctx, w.ID, gift1) //nolint:errcheck
	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "1111111111") {
		t.Error("dalam TTL, perubahan dashboard belum terlihat (cache dipakai)")
	}
	// Data tamu selalu segar: jawaban RSVP langsung terlihat.
	f.guests.UpdateRSVP(ctx, w.ID, g.ID, guest.StatusAttending, 1, "") //nolint:errcheck
	if !strings.Contains(f.get("/i/"+g.InvitationCode, nil).Body.String(), "Konfirmasi Anda: Hadir") {
		t.Error("status RSVP tamu harus segar walau data wedding di-cache")
	}
	// Ucapan baru dari tamu mengosongkan cache.
	if rec := f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Ani", "Ucapan baru dari Ani"), false, ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("post: %d", rec.Code)
	}
	body := f.get("/w/"+w.Slug+"?guestbook=ok", nil).Body.String()
	if !strings.Contains(body, "Ucapan baru dari Ani") || !strings.Contains(body, "1111111111") {
		t.Error("setelah ucapan baru, halaman memuat data terbaru")
	}
	// Preview pemilik (draft) tidak memakai cache.
	f.setStatus(w.ID, "draft")
	f.gifts.Create(ctx, w.ID, gift.Input{Type: gift.TypeBank, Provider: "BNI", AccountNumber: "2222222222", AccountName: "K"}) //nolint:errcheck
	if !strings.Contains(f.get("/w/"+w.Slug, map[string]string{"X-Test-User": owner.String()}).Body.String(), "2222222222") {
		t.Error("preview pemilik harus tanpa cache")
	}
	// Ganti status mengganti kunci cache (Kenangan langsung tampil).
	f.setStatus(w.ID, wedding.StatusMemory)
	if !strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "Terima kasih telah menjadi bagian") {
		t.Error("perubahan status harus langsung terlihat")
	}
}
