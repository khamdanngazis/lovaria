package publicsite

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

func (f fixture) postRSVP(code string, form url.Values, htmx bool) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/i/"+code+"/rsvp", strings.NewReader(form.Encode()))
	r.Host = "lovoria.test"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	f.e.ServeHTTP(rec, r)
	return rec
}

func token(code string) string {
	h := &Handler{Secret: []byte(testSecret)}
	return h.rsvpToken(code, testNow)
}

func rsvpForm(code, status, pax, msg string) url.Values {
	return url.Values{"token": {token(code)}, "status": {status}, "pax": {pax}, "message": {msg}}
}

func (f fixture) publishedGuest(t *testing.T, maxPax string) (wedding.Wedding, guest.Guest) {
	t.Helper()
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	g, err := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi Santoso", MaxPax: maxPax})
	if err != nil {
		t.Fatal(err)
	}
	return w, g
}

func TestRSVPSubmitHTMXAndReopen(t *testing.T) {
	f := newFixture(t)
	w, g := f.publishedGuest(t, "3")

	// Halaman tamu memuat form dengan token & action; tanpa cookie CSRF pun POST diterima.
	page := f.get("/i/"+g.InvitationCode, nil)
	body := page.Body.String()
	if !strings.Contains(body, `action="/i/`+g.InvitationCode+`/rsvp"`) || !strings.Contains(body, `name="token" value="`+token(g.InvitationCode)+`"`) {
		t.Fatal("form RSVP tidak lengkap di halaman undangan")
	}
	if cc := page.Header().Get("Cache-Control"); cc != "private, no-cache" {
		t.Errorf("Cache-Control halaman tamu = %q", cc)
	}

	rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "2", "  Selamat ya!  "), true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Konfirmasi Anda tersimpan") || !strings.Contains(rec.Body.String(), `id="rsvp"`) {
		t.Fatalf("htmx submit: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := f.guests.Get(ctx, w.ID, g.ID)
	if got.RSVPStatus != guest.StatusAttending || got.RSVPPax != 2 || got.RSVPMessage != "Selamat ya!" || got.RSVPAt == nil {
		t.Fatalf("tersimpan = %+v", got)
	}
	// Langsung tercermin di statistik T07.
	st, _ := f.guests.Stats(ctx, w.ID)
	if st.Attending != 1 || st.PaxAttending != 2 || st.Pending != 0 {
		t.Errorf("stats = %+v", st)
	}
	// Membuka link lagi → status tampil.
	body = f.get("/i/"+g.InvitationCode, nil).Body.String()
	if !strings.Contains(body, "Konfirmasi Anda: Hadir · 2 orang") || !strings.Contains(body, "Perbarui konfirmasi") {
		t.Error("status RSVP tidak tampil saat link dibuka lagi")
	}
}

func TestRSVPValidationAndIdempotent(t *testing.T) {
	f := newFixture(t)
	w, g := f.publishedGuest(t, "2")

	// pax > max_pax ditolak server.
	rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "5", ""), true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Jumlah orang 1–2") {
		t.Fatalf("pax > max: %d", rec.Code)
	}
	if got, _ := f.guests.Get(ctx, w.ID, g.ID); got.RSVPStatus != guest.StatusPending {
		t.Errorf("tidak boleh tersimpan: %+v", got)
	}
	// Status kosong ditolak.
	if rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "", "1", ""), true); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status kosong: %d", rec.Code)
	}

	// Submit ganda cepat → tetap satu tamu, jawaban terakhir menang.
	for _, st := range []string{"attending", "attending", "declined"} {
		if rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, st, "2", ""), true); rec.Code != http.StatusOK {
			t.Fatalf("submit %s: %d", st, rec.Code)
		}
	}
	all, _ := f.guests.All(ctx, w.ID)
	if len(all) != 1 || all[0].RSVPStatus != guest.StatusDeclined || all[0].RSVPPax != 0 {
		t.Errorf("idempoten: %+v", all)
	}
}

func TestRSVPWithoutJS(t *testing.T) {
	f := newFixture(t)
	_, g := f.publishedGuest(t, "1")
	rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "declined", "", "Maaf"), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/i/"+g.InvitationCode+"?rsvp=ok#rsvp" {
		t.Fatalf("fallback tanpa JS: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	body := f.get("/i/"+g.InvitationCode+"?rsvp=ok", nil).Body.String()
	if !strings.Contains(body, "Konfirmasi Anda tersimpan") {
		t.Error("pesan sukses tidak tampil setelah redirect")
	}
}

func TestRSVPGuards(t *testing.T) {
	f := newFixture(t)
	w, g := f.publishedGuest(t, "2")

	// Token salah / kedaluwarsa / milik kode lain.
	h := &Handler{Secret: []byte(testSecret)}
	for name, tok := range map[string]string{
		"kosong":      "",
		"palsu":       "123.abc",
		"kode lain":   h.rsvpToken("ZZZZZZZZ", testNow),
		"kedaluwarsa": h.rsvpToken(g.InvitationCode, testNow.Add(-40*24*time.Hour)),
	} {
		form := rsvpForm(g.InvitationCode, "attending", "1", "")
		form.Set("token", tok)
		if rec := f.postRSVP(g.InvitationCode, form, true); rec.Code != http.StatusForbidden {
			t.Errorf("token %s: %d", name, rec.Code)
		}
	}

	// /w/:slug tidak punya form, hanya arahan memakai link pribadi.
	if body := f.get("/w/"+w.Slug, nil).Body.String(); !strings.Contains(body, "Gunakan link undangan pribadi Anda untuk RSVP") || strings.Contains(body, `name="token"`) {
		t.Error("/w/:slug harus menampilkan arahan link pribadi")
	}
	// Kode tidak ada → 404.
	if rec := f.postRSVP("ZZZZZZZZ", rsvpForm("ZZZZZZZZ", "attending", "1", ""), true); rec.Code != http.StatusNotFound {
		t.Errorf("kode tidak ada: %d", rec.Code)
	}

	// Setelah hari H (Kenangan) RSVP read-only.
	if rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "1", ""), true); rec.Code != http.StatusOK {
		t.Fatalf("submit awal: %d", rec.Code)
	}
	f.setStatus(w.ID, wedding.StatusMemory)
	rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "declined", "", ""), true)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "sudah ditutup") {
		t.Errorf("memory: %d", rec.Code)
	}
	if got, _ := f.guests.Get(ctx, w.ID, g.ID); got.RSVPStatus != guest.StatusAttending {
		t.Errorf("RSVP berubah saat ditutup: %s", got.RSVPStatus)
	}
	body := f.get("/i/"+g.InvitationCode, nil).Body.String()
	if !strings.Contains(body, "Konfirmasi Anda: Hadir · 1 orang") || strings.Contains(body, `name="token"`) {
		t.Error("memory: harus tampil ringkasan tanpa form")
	}

	// Draft: tidak menerima RSVP.
	f.setStatus(w.ID, wedding.StatusDraft)
	if rec := f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "1", ""), true); rec.Code != http.StatusNotFound {
		t.Errorf("draft: %d", rec.Code)
	}
}

func TestRSVPRateLimitPerCode(t *testing.T) {
	f := newFixture(t)
	w, g := f.publishedGuest(t, "1")
	other, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Ani"})
	var last int
	for i := 0; i < 12; i++ {
		last = f.postRSVP(g.InvitationCode, rsvpForm(g.InvitationCode, "attending", "1", ""), true).Code
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("setelah banyak kiriman: %d, want 429", last)
	}
	// Kode lain tidak terpengaruh.
	if rec := f.postRSVP(other.InvitationCode, rsvpForm(other.InvitationCode, "attending", "1", ""), true); rec.Code != http.StatusOK {
		t.Errorf("kode lain: %d", rec.Code)
	}
}
