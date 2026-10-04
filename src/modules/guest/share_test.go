package guest

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestWhatsAppURLEncoding(t *testing.T) {
	msg := "Kepada Yth.\nBapak *Budi & Keluarga*\n\nHadir ya 🙏💍 #bahagia 100%\nhttps://lovoria.my.id/i/ABCD234?x=1"
	u := WhatsAppURL("6281234567890", msg)
	if !strings.HasPrefix(u, "https://wa.me/6281234567890?text=") {
		t.Fatalf("prefix: %s", u)
	}
	raw := strings.TrimPrefix(u, "https://wa.me/6281234567890?text=")
	if strings.ContainsAny(raw, " \n+#&") {
		t.Errorf("karakter mentah tersisa: %s", raw)
	}
	if !strings.Contains(raw, "%0A") || !strings.Contains(raw, "%20") || !strings.Contains(raw, "%F0%9F%99%8F") {
		t.Errorf("baris baru / spasi / emoji tidak ter-encode: %s", raw)
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Query().Get("text") != msg {
		t.Errorf("decode ulang tidak sama: %q", parsed.Query().Get("text"))
	}
	// Tanpa nomor → WhatsApp meminta memilih kontak.
	if got := WhatsAppURL("", "hai"); got != "https://wa.me/?text=hai" {
		t.Errorf("tanpa nomor: %s", got)
	}
}

func TestRenderTemplate(t *testing.T) {
	got := Render(DefaultTemplates[LangID], ShareVars{GuestName: "Budi", Couple: "Samuel & Sarah", Date: "Sabtu, 12 Desember 2026", Link: "https://x/i/A"})
	for _, want := range []string{"*Budi*", "*Samuel & Sarah*", "Sabtu, 12 Desember 2026", "https://x/i/A", "🙏"} {
		if !strings.Contains(got, want) {
			t.Errorf("tidak memuat %q", want)
		}
	}
	if strings.Contains(got, "{") {
		t.Errorf("placeholder tersisa: %s", got)
	}
}

func TestShareTemplateCRUD(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	st, _ := f.svc.ShareTemplate(ctx, w.ID)
	if st.Custom || st.Language != LangID || st.Body != DefaultTemplates[LangID] {
		t.Fatalf("bawaan: %+v", st)
	}
	var v ValidationError
	if _, err := f.svc.SaveShareTemplate(ctx, w.ID, LangID, "Datang ya"); !errors.As(err, &v) || v["body"] == "" {
		t.Errorf("tanpa {link}: %v", err)
	}
	if _, err := f.svc.SaveShareTemplate(ctx, w.ID, "fr", "{link}"); !errors.As(err, &v) || v["language"] == "" {
		t.Errorf("bahasa: %v", err)
	}
	if _, err := f.svc.SaveShareTemplate(ctx, w.ID, LangEN, "Hi {guest_name}\r\n{link}\n"); err != nil {
		t.Fatal(err)
	}
	st, _ = f.svc.ShareTemplate(ctx, w.ID)
	if !st.Custom || st.Language != LangEN || st.Body != "Hi {guest_name}\n{link}" {
		t.Errorf("tersimpan: %+v", st)
	}
	_, other := f.newWedding(t, "b@example.com")
	if o, _ := f.svc.ShareTemplate(ctx, other.ID); o.Custom {
		t.Error("template bocor ke wedding lain")
	}
	f.svc.ResetShareTemplate(ctx, w.ID) //nolint:errcheck
	if st, _ := f.svc.ShareTemplate(ctx, w.ID); st.Custom {
		t.Error("reset")
	}
}

func TestMarkSharedAndFilter(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	_, other := f.newWedding(t, "b@example.com")
	a := f.add(t, w.ID, Input{Name: "Ani"})
	f.add(t, w.ID, Input{Name: "Budi"})
	if err := f.svc.MarkShared(ctx, other.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("lintas wedding: %v", err)
	}
	if err := f.svc.MarkShared(ctx, w.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	names := func(shared string) []string {
		p, _ := f.svc.List(ctx, w.ID, Filter{Shared: shared})
		var out []string
		for _, g := range p.Guests {
			out = append(out, g.Name)
		}
		return out
	}
	if got := names("no"); len(got) != 1 || got[0] != "Budi" {
		t.Errorf("belum dibagikan: %v", got)
	}
	if got := names("yes"); len(got) != 1 || got[0] != "Ani" {
		t.Errorf("sudah dibagikan: %v", got)
	}
	if got := names(""); len(got) != 2 {
		t.Errorf("semua: %v", got)
	}
}

func TestShareViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	bob, _ := f.newWedding(t, "bob@example.com")
	f.svc.SetOrigins(func(context.Context, uuid.UUID) (string, error) { return "https://www.azida-memories.my.id", nil })
	withPhone := f.add(t, w.ID, Input{Name: "Budi & Keluarga", Phone: "0812-3456-7890"})
	noPhone := f.add(t, w.ID, Input{Name: "Sari"})

	body := send(e, owner, get(w.DashboardURL("/guests")), false).Body.String()
	link := "https://www.azida-memories.my.id/i/" + withPhone.InvitationCode
	for _, want := range []string{
		`href="https://wa.me/6281234567890?text=`,
		`href="https://wa.me/?text=`, "(pilih kontak)", // tamu tanpa nomor tetap bisa dikirim / disalin
		"Salin link", "Salin pesan",
		htmlEsc(jsString(link)), htmlEsc(jsString("https://www.azida-memories.my.id/i/" + noPhone.InvitationCode)),
		"Budi%20%26%20Keluarga", // nama di dalam pesan WA ter-encode
	} {
		if !strings.Contains(body, want) {
			t.Errorf("daftar tamu tidak memuat %q", want)
		}
	}

	// Tandai dibagikan: 204; wedding lain → 404.
	mark := w.DashboardURL("/guests/" + withPhone.ID.String() + "/shared")
	if rec := send(e, owner, formReq(http.MethodPost, mark, url.Values{}), true); rec.Code != http.StatusNoContent {
		t.Errorf("mark: %d", rec.Code)
	}
	if rec := send(e, bob, formReq(http.MethodPost, mark, url.Values{}), true); rec.Code != http.StatusNotFound {
		t.Errorf("mark wedding lain: %d", rec.Code)
	}
	if g, _ := f.svc.Get(ctx, w.ID, withPhone.ID); g.SharedAt == nil {
		t.Error("shared_at tidak terisi")
	}

	// Halaman Bagikan: preview awal ter-render di server; simpan tanpa {link} → 422.
	page := send(e, owner, get(w.DashboardURL("/share")), false).Body.String()
	if !strings.Contains(page, "Budi Santoso") || !strings.Contains(page, "Template pesan undangan") || !strings.Contains(page, "sharePreview(") {
		t.Error("halaman bagikan")
	}
	rec := send(e, owner, formReq(http.MethodPost, w.DashboardURL("/share/template"), url.Values{"language": {"id"}, "body": {"Halo {guest_name}"}}), false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "harus memuat {link}") || !strings.Contains(rec.Body.String(), "Halo {guest_name}") {
		t.Errorf("template invalid: %d", rec.Code)
	}
	rec = send(e, owner, formReq(http.MethodPost, w.DashboardURL("/share/template"), url.Values{"language": {"en"}, "body": {"Hi {guest_name}! {link}"}}), false)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("simpan: %d", rec.Code)
	}
	body = send(e, owner, get(w.DashboardURL("/guests")), false).Body.String()
	if !strings.Contains(body, "Hi%20Sari%21%20https%3A%2F%2Fwww.azida-memories.my.id%2Fi%2F"+noPhone.InvitationCode) {
		t.Error("pesan per tamu memakai template tersimpan")
	}
	// Semua route bagikan hanya untuk pemilik.
	for _, r := range []*http.Request{
		get(w.DashboardURL("/share")),
		formReq(http.MethodPost, w.DashboardURL("/share/template"), url.Values{"language": {"id"}, "body": {"{link}"}}),
		formReq(http.MethodPost, w.DashboardURL("/share/template/reset"), url.Values{}),
	} {
		if rec := send(e, bob, r, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d", r.Method, r.URL, rec.Code)
		}
	}
}

// htmlEsc: escape atribut HTML seperti templ (untuk mencocokkan isi x-data).
func htmlEsc(s string) string {
	return strings.NewReplacer(`&`, "&amp;", `"`, "&#34;", `<`, "&lt;", `>`, "&gt;", `'`, "&#39;").Replace(s)
}
