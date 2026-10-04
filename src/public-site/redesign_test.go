package publicsite

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

// T21: wedding baru memakai tema Signature; fragmen htmx bagian bersama (RSVP,
// ucapan) tetap memakai judul tema setelah di-swap; sampul galeri mendapat srcset.
func TestSignatureFragmentsAndCover(t *testing.T) {
	f := newFixture(t)
	w, g := f.publishedGuest(t, "2")
	if wedding.DefaultThemeID != "signature" || w.ThemeID != "signature" {
		t.Fatalf("tema wedding baru = %q", w.ThemeID)
	}
	themed := func(body string) bool {
		return strings.Contains(body, "<svg") && strings.Contains(body, "text-4xl leading-tight")
	}
	if body := f.get("/i/"+g.InvitationCode, nil).Body.String(); !strings.Contains(body, `data-theme="signature"`) || !strings.Contains(body, `data-open="lock"`) || !themed(body) {
		t.Error("halaman undangan Signature")
	}
	rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "1", ""), true)
	if rec.Code != http.StatusOK || !themed(rec.Body.String()) {
		t.Errorf("fragmen RSVP harus memakai judul tema: %d", rec.Code)
	}
	rec = f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Ani", "Selamat"), true, "")
	if rec.Code != http.StatusOK || !themed(rec.Body.String()) {
		t.Errorf("fragmen ucapan harus memakai judul tema: %d", rec.Code)
	}

	// Foto utama dari galeri → sampul memakai srcset (thumbnail + ukuran penuh).
	f.exec(`INSERT INTO gallery_items (id, wedding_id, category, object_key, thumb_key, url, thumb_url, width, height, size_bytes)
		VALUES ($1, $2, 'cover', 'k', 'kt', 'https://media.test/c.jpg', 'https://media.test/c_thumb.jpg', 1600, 2000, 10)`, uuid.New(), w.ID)
	f.exec(`UPDATE weddings SET main_photo_url = 'https://media.test/c.jpg' WHERE id = $1`, w.ID)
	body := f.get("/w/"+w.Slug, nil).Body.String()
	if !strings.Contains(body, `srcset="https://media.test/c_thumb.jpg 480w, https://media.test/c.jpg 1600w"`) || !strings.Contains(body, `fetchpriority="high"`) {
		t.Error("sampul: srcset / fetchpriority")
	}
}
