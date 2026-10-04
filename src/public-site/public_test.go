package publicsite

import (
	"context"
	"errors"
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
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
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
	handler   *Handler
	views     *ViewBuilder
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
	stories   *story.Service
	exec      func(sql string, args ...any)
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
	h := &Handler{BaseURL: cfg.BaseURL, PublishPrice: 149000, Views: views, Guests: gs, Guestbook: gb, Events: evs, Log: log, Secret: []byte(testSecret), now: func() time.Time { return testNow }}
	Register(e, Deps{
		Resolver:       &Resolver{Weddings: ws, Guests: gs, Domains: domains, BaseURL: cfg.BaseURL, ExtraHosts: []string{"lovaria.up.railway.app"}, HostHeader: "X-Forwarded-Host", Log: log},
		Handler:        h,
		RSVPLimit:      RSVPLimit{PerMinute: 60, Burst: 8},
		GuestbookLimit: GuestbookLimit{PerMinute: 60, Burst: 8},
	})
	return fixture{
		e: e, handler: h, views: views, themes: views.Themes, weddings: ws, guests: gs, events: evs, domains: domains, guestbook: gb, gifts: gf,
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
		stories: views.Stories,
		exec: func(sql string, args ...any) {
			if _, err := pool.Exec(ctx, sql, args...); err != nil {
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
	owner, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
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
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah Putri")
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
		`<meta property="og:title" content="The Wedding of Samuel &amp; Sarah">`,
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
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
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
	for _, want := range []string{"BEGIN:VCALENDAR", "DTSTART:20261212T010000Z", "DTEND:20261212T030000Z", `SUMMARY:Akad Nikah — Samuel & Sarah`, `LOCATION:Masjid\, Jl. A\, No. 1`, "\r\n"} {
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
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	_, other := f.newWedding(t, "b@example.com", "Andi", "Rina")
	f.publish(w.ID)
	f.publish(other.ID)
	f.domains["samuelsarah.com"] = w.ID
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	gOther, _ := f.guests.Create(ctx, other.ID, guest.Input{Name: "Cici"})

	host := map[string]string{"Host": "samuelsarah.com"}
	rec := f.get("/", host)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Samuel") || !strings.Contains(rec.Body.String(), `og:url" content="https://samuelsarah.com"`) {
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
		if body := rec.Body.String(); rec.Code != http.StatusNotFound || strings.Contains(body, "Andi") || strings.Contains(body, "Samuel") {
			t.Errorf("host tak dikenal %s: %d", p, rec.Code)
		}
	}
}

func TestMemoryAndArchivedPages(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	_, _ = f.events.CreateEvent(ctx, w.ID, event.Input{Name: "Akad Nikah", Type: event.TypeAkad, Date: "2026-12-12", StartTime: "08:00", Venue: "Masjid"})
	_, _ = f.stories.CreateStory(ctx, w.ID, story.Input{Year: "2020", Title: "Pertama bertemu"})
	_, _ = f.gifts.Create(ctx, w.ID, gift.Input{Type: gift.TypeBank, Provider: "BCA", AccountNumber: "1111111111", AccountName: "K"})
	f.exec(`INSERT INTO gallery_items (id, wedding_id, category, object_key, thumb_key, url, thumb_url, width, height, size_bytes)
		VALUES ($1, $2, 'wedding', 'k', 'kt', '/media/hari-h.jpg', '/media/hari-h-thumb.jpg', 800, 600, 10)`, uuid.New(), w.ID)
	fav, _ := f.guestbook.Post(ctx, w.ID, nil, "Ani", "Ucapan favorit dari Ani")
	_, _ = f.guestbook.Post(ctx, w.ID, nil, "Joko", "Ucapan biasa dari Joko")
	if _, err := f.guestbook.SetFavorite(ctx, w.ID, fav.ID, true); err != nil {
		t.Fatal(err)
	}

	// Terbit: tata letak undangan biasa.
	f.publish(w.ID)
	body := f.get("/w/"+w.Slug, nil).Body.String()
	if strings.Contains(body, `id="memories"`) || !strings.Contains(body, "Buka Undangan") || !strings.Contains(body, ".ics") {
		t.Error("terbit: belum tata letak kenangan")
	}

	// Kenangan: foto hari-H & ucapan favorit setelah pembuka, tanpa RSVP & kalender.
	f.setStatus(w.ID, wedding.StatusMemory)
	for _, th := range []string{"elegant", "romantic", "minimal", "modern"} {
		if _, err := f.themes.Save(ctx, w.ID, th, view.Settings{}); err != nil {
			t.Fatal(err)
		}
		rec := f.get("/i/"+g.InvitationCode, nil)
		body = rec.Body.String()
		mem := strings.Index(body, `id="memories"`)
		if rec.Code != http.StatusOK || mem < 0 || mem > strings.Index(body, `id="couple"`) {
			t.Fatalf("%s kenangan: %d, bagian kenangan harus sebelum pasangan", th, rec.Code)
		}
		for _, want := range []string{"Terima kasih telah menjadi bagian dari hari kami", "Kenangan", `href="#memories"`, "/media/hari-h-thumb.jpg", "Telah dilangsungkan pada", `name="message"`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s kenangan: tidak ada %q", th, want)
			}
		}
		// Ucapan favorit di bagian kenangan; ucapan biasa hanya di daftar.
		if memBody := body[mem:strings.Index(body, `id="couple"`)]; !strings.Contains(memBody, "Ucapan favorit dari Ani") || strings.Contains(memBody, "Joko") {
			t.Errorf("%s: ucapan pilihan salah", th)
		}
		if strings.Contains(body, `id="rsvp"`) || strings.Contains(body, ".ics") {
			t.Errorf("%s kenangan: RSVP / kalender masih tampil", th)
		}
	}

	// Kenangan: jawaban RSVP tamu sendiri tetap ditampilkan (bukan form).
	f.guests.UpdateRSVP(ctx, w.ID, g.ID, guest.StatusAttending, 1, "") //nolint:errcheck
	if body := f.get("/i/"+g.InvitationCode, nil).Body.String(); !strings.Contains(body, `id="rsvp"`) || strings.Contains(body, `name="pax"`) {
		t.Error("kenangan: ringkasan RSVP tamu tampil tanpa form")
	}

	// Arsip publik (default): halaman read-only lengkap, tanpa form, hadiah,
	// dan data RSVP pribadi.
	f.setStatus(w.ID, wedding.StatusArchived)
	for _, p := range []string{"/w/" + w.Slug, "/i/" + g.InvitationCode} {
		rec := f.get(p, nil)
		body := rec.Body.String()
		if rec.Code != http.StatusOK || strings.Contains(body, "Undangan ini telah diarsipkan") {
			t.Fatalf("arsip publik %s: %d", p, rec.Code)
		}
		for _, want := range []string{"Pertama bertemu", "Ucapan biasa dari Joko", "Ucapan favorit dari Ani", "/media/hari-h-thumb.jpg", "Arsip kenangan"} {
			if !strings.Contains(body, want) {
				t.Errorf("arsip publik %s: tidak ada %q", p, want)
			}
		}
		for _, bad := range []string{`name="message"`, `id="rsvp"`, "1111111111", `id="gift"`} {
			if strings.Contains(body, bad) {
				t.Errorf("arsip publik %s: tidak boleh ada %q", p, bad)
			}
		}
		if rec.Header().Get("X-Robots-Tag") != "noindex" {
			t.Errorf("arsip publik %s: harus noindex", p)
		}
	}
	// Semua form ditutup.
	if rec := f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Eko", "Halo"), true, ""); rec.Code != http.StatusForbidden {
		t.Errorf("POST ucapan arsip: %d", rec.Code)
	}
	if rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "1", ""), true); rec.Code != http.StatusForbidden {
		t.Errorf("POST RSVP arsip: %d", rec.Code)
	}
	// Daftar ucapan berikutnya tetap bisa dibaca.
	if rec := f.get("/w/"+w.Slug+"/guestbook?before="+fav.ID.String(), nil); rec.Code != http.StatusOK {
		t.Errorf("muat ucapan arsip publik: %d", rec.Code)
	}

	// Arsip privat: publik mendapat halaman ringkas, pemilik melihat arsip lengkap.
	if _, err := f.weddings.SetArchiveVisibility(ctx, w.ID, wedding.ArchivePrivate); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/w/" + w.Slug, "/i/" + g.InvitationCode} {
		body := f.get(p, nil).Body.String()
		if !strings.Contains(body, "Undangan ini telah diarsipkan") || strings.Contains(body, "Joko") || strings.Contains(body, "Pertama bertemu") {
			t.Errorf("arsip privat publik %s: harus halaman ringkas", p)
		}
	}
	if rec := f.get("/w/"+w.Slug+"/guestbook?before="+fav.ID.String(), nil); rec.Code != http.StatusNotFound {
		t.Errorf("muat ucapan arsip privat: %d", rec.Code)
	}
	rec := f.get("/w/"+w.Slug, map[string]string{"X-Test-User": owner.String()})
	if !strings.Contains(rec.Body.String(), "Ucapan biasa dari Joko") || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Errorf("arsip privat pemilik: %q", rec.Header().Get("Cache-Control"))
	}
	if body := f.get("/w/"+w.Slug, map[string]string{"X-Test-User": uuid.NewString()}).Body.String(); strings.Contains(body, "Joko") {
		t.Error("arsip privat: user lain tidak boleh melihat arsip lengkap")
	}
	if _, err := f.weddings.SetArchiveVisibility(ctx, w.ID, "rahasia"); !errors.Is(err, wedding.ErrInvalidArchiveVisibility) {
		t.Errorf("visibilitas tak dikenal: %v", err)
	}
}

// Favorit dari dashboard langsung terlihat walau halaman publik di-cache (T19).
func TestFavoriteInvalidatesCache(t *testing.T) {
	f := newFixture(t)
	f.views.CacheTTL = time.Hour
	f.guestbook.OnChange(f.views.Invalidate) // wiring sama dengan cmd/server
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	f.setStatus(w.ID, wedding.StatusMemory)
	e, _ := f.guestbook.Post(ctx, w.ID, nil, "Ani", "Semoga sakinah")

	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), `id="memories"`) {
		t.Fatal("belum ada favorit: bagian kenangan tidak tampil")
	}
	if _, err := f.guestbook.SetFavorite(ctx, w.ID, e.ID, true); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "Ucapan Pilihan") {
		t.Error("favorit baru harus langsung tampil (cache di-invalidate)")
	}
	if _, err := f.guestbook.SetFavorite(ctx, w.ID, e.ID, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "Ucapan Pilihan") {
		t.Error("lepas favorit harus langsung hilang")
	}
}

func TestCustomDomainRedirectAndHostHeader(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	f.publish(w.ID)
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	f.domains["www.samuelsarah.com"] = w.ID

	// /w/:slug di domain Lunovia → 301 ke custom domain (path & query ikut).
	rec := f.get("/w/"+w.Slug+"?ref=wa", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "https://www.samuelsarah.com?ref=wa" {
		t.Fatalf("redirect: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	rec = f.get("/w/"+w.Slug+"/events/x.ics", nil)
	if rec.Header().Get("Location") != "https://www.samuelsarah.com/events/x.ics" {
		t.Errorf("redirect sub-path: %s", rec.Header().Get("Location"))
	}
	// Link tamu /i/:code di domain Lunovia tetap dilayani (tidak dialihkan).
	if rec := f.get("/i/"+g.InvitationCode, nil); rec.Code != http.StatusOK {
		t.Errorf("/i/:code: %d", rec.Code)
	}
	// POST ke /w/:slug/guestbook tidak dialihkan (301 mengubah POST jadi GET).
	if rec := f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Ani", "Selamat"), true, ""); rec.Code != http.StatusOK {
		t.Errorf("POST guestbook: %d", rec.Code)
	}
	// Draft: tidak dialihkan (pemilik tetap bisa preview di domain Lunovia).
	f.setStatus(w.ID, "draft")
	if rec := f.get("/w/"+w.Slug, map[string]string{"X-Test-User": owner.String()}); rec.Code != http.StatusOK {
		t.Errorf("draft preview: %d", rec.Code)
	}
	f.publish(w.ID)

	// Proxy (Cloudflare Worker) menulis ulang Host ke domain Railway dan mengirim
	// host asli lewat X-Forwarded-Host.
	proxied := map[string]string{"Host": "lovaria.up.railway.app", "X-Forwarded-Host": "www.samuelsarah.com"}
	if rec := f.get("/", proxied); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Samuel") {
		t.Errorf("host header: %d", rec.Code)
	}
	// Domain Railway sendiri tetap host Lunovia (landing, bukan 404).
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
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	f.publish(w.ID)
	old := w.Slug
	if _, err := f.weddings.ChangeSlug(ctx, w.ID, "samuel-sarah-baru"); err != nil {
		t.Fatal(err)
	}
	rec := f.get("/w/"+old+"?ref=wa", nil)
	if rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/w/samuel-sarah-baru?ref=wa" {
		t.Fatalf("slug lama: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if rec := f.get("/w/"+old+"/events/x.ics", nil); rec.Header().Get("Location") != "/w/samuel-sarah-baru/events/x.ics" {
		t.Errorf("sub-path: %s", rec.Header().Get("Location"))
	}
	// POST ke slug lama (buku ucapan) → 308 supaya tetap POST.
	if rec := f.post("/w/"+old+"/guestbook", gbForm(w.ID, "Ani", "Halo"), false, ""); rec.Code != http.StatusPermanentRedirect {
		t.Errorf("POST slug lama: %d", rec.Code)
	}
	if rec := f.get("/w/samuel-sarah-baru", nil); rec.Code != http.StatusOK {
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
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
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
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
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
	owner, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
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
