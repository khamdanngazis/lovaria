package publicsite

import (
	"bytes"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

// T31: QR kehadiran hanya tampil di link pribadi tamu, bila pasangan
// menyalakan check-in dan undangan sedang Terbit / Hari H.
func TestGuestQRSection(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	f.publish(w.ID)
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Bapak Budi", MaxPax: "3"})
	personal, qr := "/i/"+g.InvitationCode, "/i/"+g.InvitationCode+"/qr.png"

	// Bawaan (mati): undangan tidak berubah, gambar QR 404.
	if b := f.get(personal, nil).Body.String(); strings.Contains(b, `id="checkin"`) || strings.Contains(b, "qr.png") {
		t.Error("check-in mati: undangan tidak boleh memuat QR")
	}
	if rec := f.get(qr, nil); rec.Code != http.StatusNotFound {
		t.Errorf("check-in mati: qr.png %d", rec.Code)
	}

	if err := f.weddings.SetCheckinEnabled(ctx, w.ID, true); err != nil {
		t.Fatal(err)
	}
	f.views.Invalidate(w.ID)
	body := f.get(personal, nil).Body.String()
	for _, want := range []string{`id="checkin"`, "QR Kehadiran", `src="` + qr + `"`, "Bapak Budi", "Berlaku untuk 3 orang", `download="qr-kehadiran-` + g.InvitationCode + `.png"`, "Simpan QR"} {
		if !strings.Contains(body, want) {
			t.Errorf("undangan pribadi tidak memuat %q", want)
		}
	}
	// Link umum: tanpa QR.
	if b := f.get("/w/"+w.Slug, nil).Body.String(); strings.Contains(b, `id="checkin"`) || strings.Contains(b, "qr.png") {
		t.Error("link umum tidak boleh memuat QR")
	}
	// Gambar: PNG sah, tidak diindeks, tidak di-cache bersama.
	rec := f.get(qr, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("X-Robots-Tag") != "noindex" || !strings.HasPrefix(rec.Header().Get("Cache-Control"), "private") {
		t.Fatalf("qr.png: %d %v", rec.Code, rec.Header())
	}
	if _, err := png.Decode(bytes.NewReader(rec.Body.Bytes())); err != nil {
		t.Errorf("qr.png bukan PNG: %v", err)
	}
	// Kode tak dikenal → 404.
	if rec := f.get("/i/AAAAAAA/qr.png", nil); rec.Code != http.StatusNotFound {
		t.Errorf("kode tak dikenal: %d", rec.Code)
	}
	// Setelah RSVP hadir 2 orang, QR menyebut jumlah itu.
	if _, err := f.guests.UpdateRSVP(ctx, w.ID, g.ID, guest.StatusAttending, 2, ""); err != nil {
		t.Fatal(err)
	}
	if b := f.get(personal, nil).Body.String(); !strings.Contains(b, "Berlaku untuk 2 orang") {
		t.Error("jumlah orang harus mengikuti RSVP")
	}
	// Hari H lewat (Kenangan): QR hilang, gambar 404.
	f.setStatus(w.ID, wedding.StatusMemory)
	if b := f.get(personal, nil).Body.String(); strings.Contains(b, `id="checkin"`) {
		t.Error("kenangan: QR tidak tampil lagi")
	}
	if rec := f.get(qr, nil); rec.Code != http.StatusNotFound {
		t.Errorf("kenangan: qr.png %d", rec.Code)
	}
}
