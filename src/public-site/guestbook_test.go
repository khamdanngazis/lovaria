package publicsite

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/gift"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

func (f fixture) post(path string, form url.Values, htmx bool, ip string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Host = "lovoria.test"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if ip != "" {
		r.Header.Set("X-Forwarded-For", ip)
	}
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, r)
	return rec
}

func gbToken(weddingID uuid.UUID) string {
	return (&Handler{Secret: []byte(testSecret)}).formToken("guestbook", weddingID.String(), testNow)
}

func gbForm(weddingID uuid.UUID, name, msg string) url.Values {
	return url.Values{"token": {gbToken(weddingID)}, "name": {name}, "message": {msg}}
}

func TestGuestbookPostAndXSS(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi Santoso"})

	// Lewat kode tamu: nama terisi otomatis.
	if body := f.get("/i/"+g.InvitationCode, nil).Body.String(); !strings.Contains(body, `id="gb-name" type="text" name="name" value="Budi Santoso"`) {
		t.Error("nama tamu harus terisi otomatis di form buku ucapan")
	}
	xss := `<script>alert("x")</script><img src=x onerror=alert(1)>`
	rec := f.post("/i/"+g.InvitationCode+"/guestbook", gbForm(w.ID, "Budi", xss), true, "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Ucapan Anda terkirim") {
		t.Fatalf("post: %d %s", rec.Code, body)
	}
	for _, page := range []string{body, f.get("/w/"+w.Slug, nil).Body.String()} {
		if strings.Contains(page, "<script>alert") || strings.Contains(page, "<img src=x") || !strings.Contains(page, "&lt;script&gt;") {
			t.Fatal("pesan XSS harus di-escape")
		}
	}
	es, _ := f.guestbook.List(ctx, w.ID, nil, 1)
	if len(es) != 1 || es[0].GuestID == nil || *es[0].GuestID != g.ID {
		t.Errorf("entri harus terhubung ke tamu: %+v", es)
	}

	// Lewat /w/:slug tanpa JS → redirect & pesan sukses.
	rec = f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Sari", "Bahagia selalu"), false, "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/w/"+w.Slug+"?guestbook=ok#guestbook" {
		t.Fatalf("no-JS: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if !strings.Contains(f.get("/w/"+w.Slug+"?guestbook=ok", nil).Body.String(), "Ucapan Anda terkirim") {
		t.Error("pesan sukses setelah redirect")
	}
	// Validasi → 422, isian dipertahankan.
	rec = f.post("/w/"+w.Slug+"/guestbook", gbForm(w.ID, "Sari", ""), true, "")
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Ucapan wajib diisi") || !strings.Contains(rec.Body.String(), `value="Sari"`) {
		t.Errorf("validasi: %d", rec.Code)
	}
}

func TestGuestbookSpamProtection(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	path := "/w/" + w.Slug + "/guestbook"

	// Honeypot terisi → tampak sukses, tidak tersimpan.
	form := gbForm(w.ID, "Bot", "Beli obat murah")
	form.Set("website", "http://spam.example")
	if rec := f.post(path, form, true, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Ucapan Anda terkirim") {
		t.Errorf("honeypot: %d", rec.Code)
	}
	// Token salah / milik wedding lain → 403.
	_, other := f.newWedding(t, "b@example.com", "X", "Y")
	for _, tok := range []string{"", "1.abc", gbToken(other.ID)} {
		form := gbForm(w.ID, "Budi", "Halo")
		form.Set("token", tok)
		if rec := f.post(path, form, true, ""); rec.Code != http.StatusForbidden {
			t.Errorf("token %q: %d", tok, rec.Code)
		}
	}
	// Kata kasar → tersimpan tapi disembunyikan.
	f.post(path, gbForm(w.ID, "Iseng", "dasar anjing"), true, "")
	if st, _ := f.guestbook.Stats(ctx, w.ID); st.Total != 1 || st.Hidden != 1 {
		t.Errorf("stats = %+v (honeypot tidak boleh tersimpan)", st)
	}
	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "dasar anjing") {
		t.Error("pesan tersaring tidak boleh tampil")
	}

	// Rate limit per IP: IP lain tidak terpengaruh.
	var last int
	for i := 0; i < 12; i++ {
		last = f.post(path, gbForm(w.ID, "Budi", fmt.Sprintf("Pesan %d", i)), true, "203.0.113.7").Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("rate limit: %d", last)
	}
	if rec := f.post(path, gbForm(w.ID, "Ani", "Halo"), true, "198.51.100.1"); rec.Code != http.StatusOK {
		t.Errorf("IP lain: %d", rec.Code)
	}
}

func TestGuestbookStatusAndLoadMore(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	_, other := f.newWedding(t, "b@example.com", "X", "Y")
	path := "/w/" + w.Slug + "/guestbook"

	// Draft: tidak bisa diisi (juga oleh pemilik yang preview).
	if rec := f.post(path, gbForm(w.ID, "A", "B"), true, ""); rec.Code != http.StatusNotFound {
		t.Errorf("draft: %d", rec.Code)
	}
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(gbForm(w.ID, "A", "B").Encode()))
	r.Host = "lovoria.test"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("X-Test-User", owner.String())
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("preview pemilik: %d", rec.Code)
	}

	// Kenangan: buku ucapan tetap terbuka.
	f.setStatus(w.ID, wedding.StatusMemory)
	for i := 0; i < 12; i++ {
		if _, err := f.guestbook.Post(ctx, w.ID, nil, fmt.Sprintf("Tamu %02d", i), "Selamat"); err != nil {
			t.Fatal(err)
		}
	}
	f.guestbook.Post(ctx, other.ID, nil, "Tamu Lain", "Bukan untuk w") //nolint:errcheck
	if rec := f.post(path, gbForm(w.ID, "Tamu 12", "Masih bisa"), true, ""); rec.Code != http.StatusOK {
		t.Fatalf("memory: %d", rec.Code)
	}
	page := f.get("/w/"+w.Slug, nil).Body.String()
	if !strings.Contains(page, "Tamu 12") || strings.Contains(page, "Tamu 02") || !strings.Contains(page, "Muat lebih banyak") || strings.Contains(page, "Tamu Lain") {
		t.Fatal("halaman pertama: 10 pesan terbaru + tombol muat lebih banyak")
	}
	i := strings.Index(page, path+"?before=")
	more := page[i : i+strings.Index(page[i:], `"`)]
	rec = f.get(more, map[string]string{"HX-Request": "true"})
	if body := rec.Body.String(); rec.Code != http.StatusOK || !strings.Contains(body, "Tamu 02") || !strings.Contains(body, "Tamu 00") || strings.Contains(body, "Muat lebih banyak") || strings.Contains(body, "<html") {
		t.Errorf("muat lebih banyak: %d %s", rec.Code, body)
	}

	// Arsip: tertutup.
	f.setStatus(w.ID, wedding.StatusArchived)
	if rec := f.post(path, gbForm(w.ID, "A", "B"), true, ""); rec.Code != http.StatusForbidden {
		t.Errorf("archived: %d", rec.Code)
	}
}

func TestRSVPCopiesMessageToGuestbook(t *testing.T) {
	f := newFixture(t)
	w, g := f.publishedGuest(t, "2")
	form := rsvpForm(g.InvitationCode, "attending", "2", "Selamat ya!")
	// Default mati: tanpa centang tidak masuk buku ucapan.
	f.postRSVP(g.InvitationCode, form, true)
	if st, _ := f.guestbook.Stats(ctx, w.ID); st.Total != 0 {
		t.Fatalf("default harus mati: %+v", st)
	}
	form.Set("message", "Selamat menempuh hidup baru!")
	form.Set("to_guestbook", "1")
	for i := 0; i < 2; i++ { // kiriman ganda → satu entri
		if rec := f.postRSVP(g.InvitationCode, form, true); rec.Code != http.StatusOK {
			t.Fatalf("rsvp: %d", rec.Code)
		}
	}
	es, _ := f.guestbook.List(ctx, w.ID, nil, 1)
	if len(es) != 1 || es[0].Name != "Budi Santoso" || es[0].Message != "Selamat menempuh hidup baru!" {
		t.Errorf("entri = %+v", es)
	}
}

func TestGiftSection(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	_, other := f.newWedding(t, "b@example.com", "X", "Y")
	f.publish(other.ID)

	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), `id="gift"`) {
		t.Error("section hadiah harus disembunyikan bila tidak ada akun")
	}
	if _, err := f.gifts.Create(ctx, w.ID, gift.Input{Type: gift.TypeBank, Provider: "BCA", AccountNumber: "123 456 7890", AccountName: "Khamdan"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.gifts.Create(ctx, w.ID, gift.Input{Type: gift.TypeAddress, AccountName: "Sarah", Address: "Jl. Mawar 1, Jakarta"}); err != nil {
		t.Fatal(err)
	}
	body := f.get("/w/"+w.Slug, nil).Body.String()
	for _, want := range []string{`id="gift"`, "123 456 7890", `data-copy="123 456 7890"`, "Salin Nomor", "Jl. Mawar 1, Jakarta", "Salin Alamat"} {
		if !strings.Contains(body, want) {
			t.Errorf("tidak memuat %q", want)
		}
	}
	// Nomor rekening hanya di section hadiah: tidak di OG meta / <head>.
	head := body[:strings.Index(body, "</head>")]
	if strings.Contains(head, "7890") {
		t.Error("nomor rekening bocor ke <head> / OG meta")
	}
	if i := strings.Index(body, `id="gift"`); strings.Contains(body[:i], "7890") {
		t.Error("nomor rekening tampil di luar section hadiah")
	}
	// Tenant: wedding lain tidak menampilkan akun ini.
	if strings.Contains(f.get("/w/"+other.Slug, nil).Body.String(), "7890") {
		t.Error("akun hadiah bocor ke wedding lain")
	}
	// Arsip: halaman ringkas tanpa nomor rekening.
	f.setStatus(w.ID, wedding.StatusArchived)
	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "7890") {
		t.Error("halaman arsip tidak boleh memuat nomor rekening")
	}
}
