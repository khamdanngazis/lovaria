package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
)

func (f fixture) keepsake(t *testing.T, photo []byte) *Keepsake {
	t.Helper()
	k := &Keepsake{Weddings: f.home.Weddings, Stories: f.home.Stories, Guests: f.home.Guests, Guestbook: f.home.Guestbook}
	if photo != nil {
		k.Photo = func(context.Context, string) ([]byte, error) { return photo, nil }
	}
	return k
}

func TestKeepsakePDF(t *testing.T) {
	keepsakeCompress = false
	t.Cleanup(func() { keepsakeCompress = true })
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	photo := "https://media.test/cover.jpg"
	if _, err := f.home.Weddings.UpdateWeddingInfo(ctx, w.ID, wedding.InfoInput{Title: w.Title, WeddingDate: "2026-12-12", Timezone: w.Timezone, MainPhotoURL: photo}); err != nil {
		t.Fatal(err)
	}
	w, _ = f.home.Weddings.GetWedding(ctx, w.ID)
	if _, err := f.home.Stories.CreateStory(ctx, w.ID, story.Input{Year: "2019", Title: "Pertama bertemu", Description: "Di kampus 🎓"}); err != nil {
		t.Fatal(err)
	}
	g, _ := f.home.Guests.Create(ctx, w.ID, guest.Input{Name: "Budi", MaxPax: "2"})
	f.home.Guests.UpdateRSVP(ctx, w.ID, g.ID, guest.StatusAttending, 2, "") //nolint:errcheck
	for i := 0; i < 500; i++ {
		if _, err := f.home.Guestbook.Post(ctx, w.ID, nil, fmt.Sprintf("Tamu %03d", i), fmt.Sprintf("Selamat berbahagia nomor %03d 🎉", i)); err != nil {
			t.Fatal(err)
		}
	}
	hidden, _ := f.home.Guestbook.Post(ctx, w.ID, nil, "Iseng", "Pesan tersembunyi")
	if _, err := f.home.Guestbook.SetHidden(ctx, w.ID, hidden.ID, true); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	var buf bytes.Buffer
	if err := f.keepsake(t, jpegBytes(t)).Write(ctx, w, &buf); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("500 ucapan: %v (> 5 detik)", d)
	}
	pdf := buf.String()
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Fatal("bukan PDF")
	}
	for _, want := range []string{"Khamdan & Sarah", "Pertama bertemu", "Di kampus", "Selamat berbahagia nomor 000", "Selamat berbahagia nomor 499", "Tamu 250", "1 undangan", "2 orang", "500 pesan", "/Subtype /Image"} {
		if !strings.Contains(pdf, want) {
			t.Errorf("PDF tidak memuat %q", want)
		}
	}
	if strings.Contains(pdf, "Pesan tersembunyi") {
		t.Error("ucapan tersembunyi tidak boleh ikut")
	}
	// Foto gagal diambil / bukan JPEG → PDF tetap jadi tanpa foto.
	buf.Reset()
	if err := f.keepsake(t, []byte("bukan gambar")).Write(ctx, w, &buf); err != nil || strings.Contains(buf.String(), "/Subtype /Image") {
		t.Errorf("foto rusak: err=%v", err)
	}
}

func TestKeepsakeRouteOwnerOnly(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com")
	bob, _ := f.newWedding(t, "b@example.com")
	owned := wedding.Register(f.e.Group("/dashboard/w2"), wedding.Deps{Service: f.home.Weddings})
	f.keepsake(t, nil).Register(owned)
	path := "/dashboard/w2/" + w.ID.String() + "/keepsake.pdf"

	get := func(user fmt.Stringer) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Test-User", user.String())
		rec := httptest.NewRecorder()
		f.e.ServeHTTP(rec, r)
		return rec
	}
	rec := get(owner)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("Cache-Control") != "private, no-store" ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "kenangan-"+w.Slug+".pdf") {
		t.Fatalf("owner: %d %v", rec.Code, rec.Header())
	}
	if rec := get(bob); rec.Code != http.StatusNotFound {
		t.Errorf("bob: %d", rec.Code)
	}
}

func TestMemoryCard(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com")
	if html := f.overview(t, owner, w); strings.Contains(html, "Abadikan kenangan") || strings.Contains(html, "keepsake.pdf") {
		t.Error("draft: kartu kenangan & keepsake belum tampil")
	}
	if _, err := f.home.Events.CreateEvent(ctx, w.ID, event.Input{Name: "Akad", Type: "akad", Date: "2026-12-12", StartTime: "08:00", Venue: "Masjid"}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		to    string
		actor wedding.ActorKind
	}{{wedding.StatusPublished, wedding.ActorUser}, {wedding.StatusWeddingDay, wedding.ActorSystem}} {
		if _, err := f.home.Weddings.Transition(ctx, w.ID, s.to, wedding.Actor{Kind: s.actor, UserID: owner}); err != nil {
			t.Fatal(err)
		}
	}
	// Terbit / Hari H: keepsake sudah bisa diunduh, kartu kenangan belum.
	if html := f.overview(t, owner, w); strings.Contains(html, "Abadikan kenangan") || !strings.Contains(html, "keepsake.pdf") {
		t.Error("hari H: link keepsake tampil, kartu kenangan belum")
	}
	if _, err := f.home.Weddings.Transition(ctx, w.ID, wedding.StatusMemory, wedding.Actor{Kind: wedding.ActorSystem}); err != nil {
		t.Fatal(err)
	}
	html := f.overview(t, owner, w)
	if !strings.Contains(html, "Abadikan kenangan") || !strings.Contains(html, "Unggah foto hari bahagia") || !strings.Contains(html, "Pilih ucapan favorit") || !strings.Contains(html, "Unduh kenang-kenangan (PDF)") {
		t.Fatal("kenangan: kartu saran tidak lengkap")
	}
	if strings.Count(html, "border-2 border-slate-300") < 2 {
		t.Error("kedua saran belum selesai")
	}
	// Setelah ada foto hari-H & ucapan favorit → keduanya tercentang.
	if _, err := f.home.Gallery.Upload(ctx, w.ID, gallery.CategoryWedding, bytes.NewReader(jpegBytes(t))); err != nil {
		t.Fatal(err)
	}
	e, _ := f.home.Guestbook.Post(ctx, w.ID, nil, "Ani", "Selamat")
	if _, err := f.home.Guestbook.SetFavorite(ctx, w.ID, e.ID, true); err != nil {
		t.Fatal(err)
	}
	html = f.overview(t, owner, w)
	card := html[strings.Index(html, "Abadikan kenangan"):]
	card = card[:strings.Index(card, "keepsake.pdf")]
	if strings.Count(card, "✓") != 2 {
		t.Errorf("saran selesai = %d, want 2", strings.Count(card, "✓"))
	}
}
