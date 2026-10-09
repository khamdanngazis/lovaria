package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
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

type fixture struct {
	e    *echo.Echo
	home *Home
	auth *auth.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, _ := config.LoadFrom(func(k string) string {
		return map[string]string{"APP_ENV": config.EnvTest}[k]
	})
	store, _ := storage.NewLocal(t.TempDir(), "/media")
	ws := wedding.NewService(wedding.NewRepository(pool))
	evs := event.NewService(event.NewRepository(pool))
	ws.SetEventCounter(evs)
	h := &Home{
		Weddings: ws, Events: evs, Stories: story.NewService(story.NewRepository(pool)),
		Gallery: gallery.NewService(gallery.NewRepository(pool), store, ws, 500<<20, log), Themes: theme.NewService(pool, ws),
		Guests: guest.NewService(guest.NewRepository(pool), "https://lovoria.test"), Guestbook: guestbook.NewService(pool, guestbook.NewWordFilter(guestbook.DefaultBlockedWords)),
	}
	ws.SetDomains(nil, "https://lovoria.test")
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
	wedding.Register(e.Group("/dashboard/weddings"), wedding.Deps{Service: ws, Home: h.Widgets})
	return fixture{e: e, home: h, auth: auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log)}
}

func (f fixture) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.home.Weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "Samuel", BrideName: "Sarah", Title: "Pernikahan K & S", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func (f fixture) overview(t *testing.T, user uuid.UUID, w wedding.Wedding) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, w.DashboardURL(""), nil)
	r.Header.Set("X-Test-User", user.String())
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview: %d", rec.Code)
	}
	return rec.Body.String()
}

// checklist mengembalikan status tiap langkah onboarding di HTML ("Sudah:" / "Belum:").
func checklist(html string) map[string]bool {
	out := map[string]bool{}
	for _, label := range []string{"Profil pasangan", "Acara", "Cerita cinta", "Galeri foto", "Tampilan", "Daftar tamu", "Publikasikan"} {
		switch {
		case strings.Contains(html, "Sudah:</span> "+label) || strings.Contains(html, "Sudah:</span>"+label):
			out[label] = true
		case strings.Contains(html, "Belum:</span> "+label) || strings.Contains(html, "Belum:</span>"+label):
			out[label] = false
		}
	}
	return out
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := 0; x < 40; x++ {
		img.Set(x, x%30, color.RGBA{R: 200, A: 255})
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestOnboardingChecklist(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com")

	// User baru: semua langkah belum selesai.
	html := f.overview(t, owner, w)
	cl := checklist(html)
	if len(cl) != 7 {
		t.Fatalf("checklist tidak lengkap: %v", cl)
	}
	for label, done := range cl {
		if done {
			t.Errorf("user baru: %s sudah selesai?", label)
		}
	}
	if !strings.Contains(html, "0/7") || !strings.Contains(html, "H-") {
		t.Error("progress 0/7 & hitung mundur harus tampil")
	}

	desc := "Putra/putri tercinta"
	if _, err := f.home.Weddings.UpdateCouple(ctx, w.ID, wedding.CoupleInput{GroomName: "Samuel", BrideName: "Sarah", GroomDescription: desc, BrideDescription: desc}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.home.Events.CreateEvent(ctx, w.ID, event.Input{Name: "Akad", Type: "akad", Date: "2026-12-12", StartTime: "08:00", Venue: "Masjid"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.home.Stories.CreateStory(ctx, w.ID, story.Input{Year: "2019", Title: "Bertemu"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.home.Gallery.Upload(ctx, w.ID, gallery.CategoryPrewedding, bytes.NewReader(jpegBytes(t))); err != nil {
		t.Fatal(err)
	}
	if _, err := f.home.Themes.Save(ctx, w.ID, "elegant", view.Settings{}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.home.Guests.Create(ctx, w.ID, guest.Input{Name: "Budi"}); err != nil {
		t.Fatal(err)
	}
	cl = checklist(f.overview(t, owner, w))
	for label, done := range cl {
		if done != (label != "Publikasikan") {
			t.Errorf("setelah dilengkapi: %s = %v", label, done)
		}
	}
	if _, err := f.home.Weddings.MarkPaid(ctx, w.ID, wedding.PaidGateway); err != nil { // prasyarat publikasi (T23)
		t.Fatal(err)
	}
	if _, err := f.home.Weddings.Transition(ctx, w.ID, wedding.StatusPublished, wedding.Actor{Kind: wedding.ActorUser, UserID: owner}); err != nil {
		t.Fatal(err)
	}
	if html := f.overview(t, owner, w); strings.Contains(html, "Lengkapi undanganmu") {
		t.Error("checklist disembunyikan setelah semua selesai")
	}
}

func TestSummaryNumbers(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com")
	_, other := f.newWedding(t, "b@example.com")
	gs := f.home.Guests
	var ids []uuid.UUID
	for i := 0; i < 4; i++ {
		g, _ := gs.Create(ctx, w.ID, guest.Input{Name: fmt.Sprintf("Tamu %d", i), MaxPax: "2"})
		ids = append(ids, g.ID)
	}
	gs.Create(ctx, other.ID, guest.Input{Name: "Tamu lain"})       //nolint:errcheck
	gs.UpdateRSVP(ctx, w.ID, ids[0], guest.StatusAttending, 2, "") //nolint:errcheck
	gs.UpdateRSVP(ctx, w.ID, ids[1], guest.StatusDeclined, 0, "")  //nolint:errcheck
	gs.MarkOpened(ctx, w.ID, ids[0])                               //nolint:errcheck
	gs.MarkOpened(ctx, w.ID, ids[1])                               //nolint:errcheck
	gs.MarkOpened(ctx, w.ID, ids[2])                               //nolint:errcheck
	for i := 0; i < 7; i++ {
		f.home.Guestbook.Post(ctx, w.ID, nil, fmt.Sprintf("Penulis %d", i), "Selamat") //nolint:errcheck
	}
	f.home.Guestbook.Post(ctx, other.ID, nil, "Penulis lain", "x") //nolint:errcheck

	html := f.overview(t, owner, w)
	for _, want := range []string{
		"https://lovoria.test/w/" + w.Slug,
		"3 dari 4 tamu membuka", // open rate 75%
		"75%",
		"2 dari 4 tamu menjawab", // RSVP rate 50%
		"50%",
		"Penulis 6", "Penulis 2", // 5 terbaru
		"Galeri · 0 foto",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("tidak memuat %q", want)
		}
	}
	if strings.Contains(html, "Penulis 1<") || strings.Contains(html, "Penulis lain") || strings.Contains(html, "Tamu lain") {
		t.Error("hanya 5 ucapan terbaru milik wedding ini")
	}
}

func TestDashboardLoad500Guests(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com")
	in := make([]guest.Input, 500)
	for i := range in {
		in[i] = guest.Input{Name: fmt.Sprintf("Tamu %03d", i), MaxPax: "2"}
	}
	if _, _, err := f.home.Guests.AddMany(ctx, w.ID, in); err != nil {
		t.Fatal(err)
	}
	f.overview(t, owner, w) // pemanasan (koneksi pool, template)
	start := time.Now()
	for i := 0; i < 5; i++ {
		f.overview(t, owner, w)
	}
	if avg := time.Since(start) / 5; avg > 300*time.Millisecond {
		t.Errorf("beranda dengan 500 tamu: %v per request (batas 300ms)", avg)
	} else {
		t.Logf("beranda dengan 500 tamu: %v per request", avg)
	}
}

// Dashboard hanya agregator: tidak boleh mengimpor paket database / sqlc.
func TestNoQueriesInDashboard(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(p, "github.com/jackc/") || strings.HasSuffix(p, "/platform/db") || strings.HasSuffix(p, "/db") {
				t.Errorf("%s mengimpor %s — ambil data lewat service modul", file, p)
			}
		}
	}
}

// Menu akun di header (T22): pintasan + keluar, berbasis <details> (tanpa JS
// tetap berfungsi); "Panel admin" hanya untuk admin.
func TestHeaderAccountMenu(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com")
	html := f.overview(t, owner, w)
	menu := html[strings.Index(html, `aria-label="Menu akun"`):]
	menu = menu[:strings.Index(menu, "</nav>")]
	for _, want := range []string{
		"<details", "<summary", `href="/dashboard"`, `href="/dashboard/weddings"`, `href="/dashboard/weddings/new"`,
		"Buat wedding baru", "Halaman utama Lunovia", `action="/logout"`, "Keluar", "<svg",
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("menu akun tidak memuat %q", want)
		}
	}
	if strings.Contains(menu, "Panel admin") {
		t.Error("Panel admin hanya untuk akun admin")
	}
}
