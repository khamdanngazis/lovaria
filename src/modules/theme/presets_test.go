package theme

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

func TestSignaturePresetsAvailable(t *testing.T) {
	d := registry["signature"]
	want := []string{"signature-lavender", "sage-garden", "blush-romance", "midnight-elegance", "terracotta-bloom"}
	var got []string
	for _, p := range d.Presets {
		got = append(got, p.ID)
		// Aset floral per preset di-commit (e2e/floral-tints.js, ≤ 80 KB per berkas).
		for _, part := range []string{"cluster", "corner", "ribbon-left", "ribbon", "petal"} {
			fi, err := os.Stat(filepath.Join("..", "..", "..", "static", "img", "themes", "signature", "floral", p.ID, part+".webp"))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Size() > 80*1024 {
				t.Errorf("%s/%s: floral %d byte > 80 KB", p.ID, part, fi.Size())
			}
		}
		// Teks di atas latar Soft (bagian hadiah).
		for name, fg := range map[string]string{"Ink": p.Ink, "Deep": p.Deep} {
			if c := Contrast(fg, p.Soft); c < MinContrast {
				t.Errorf("%s: %s di atas Soft %.2f < %.1f", p.ID, name, c, MinContrast)
			}
		}
		if c := Contrast(p.Ink, p.Paper); c < MinContrast {
			t.Errorf("%s: Ink di atas Paper %.2f", p.ID, c)
		}
		if c := Contrast(p.Muted, p.Paper); c < MinContrast {
			t.Errorf("%s: Muted di atas Paper %.2f", p.ID, c)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("preset = %v, want %v", got, want)
	}
	if d.Tokens.Preset != want[0] {
		t.Errorf("preset bawaan = %q", d.Tokens.Preset)
	}
}

var (
	cssBlock  = regexp.MustCompile(`(?s)<style>.*?</style>`)
	floralRef = regexp.MustCompile(`floral/[a-z-]+/([a-z-]+)\.webp(\?v=[0-9a-f]+)?`)
)

// Pergantian warna tidak boleh mengubah identitas layout (T33 §4.6): HTML semua
// preset identik setelah token CSS & path floral dinormalkan.
func TestPresetsOnlyChangeColors(t *testing.T) {
	normalize := func(html string) string {
		return floralRef.ReplaceAllString(cssBlock.ReplaceAllString(html, ""), "floral/$1")
	}
	var first string
	for i, p := range registry["signature"].Presets {
		v := sample()
		v.ThemeID = "signature"
		v.Settings.ColorPreset = p.ID
		html := render(t, v)
		if !strings.Contains(html, "--lv-primary:"+p.Primary) || !strings.Contains(html, "--lv-soft:"+p.Soft) {
			t.Errorf("%s: token preset tidak dipakai", p.ID)
		}
		if !strings.Contains(html, "floral/"+p.ID+"/") {
			t.Errorf("%s: floral preset tidak dipakai", p.ID)
		}
		if i == 0 {
			first = normalize(html)
		} else if normalize(html) != first {
			t.Errorf("%s: struktur HTML berbeda dari preset pertama", p.ID)
		}
	}
}

func TestSignatureIgnoresFreeCustomization(t *testing.T) {
	v := sample()
	v.ThemeID = "signature"
	v.Settings = view.Settings{
		ColorPreset: "sage-garden", PrimaryColor: "#123456", FontHeading: "Cinzel", FontBody: "Poppins", Background: "#000000",
		SectionOrder: []string{"gallery", "events", "couple"},
	}
	html := render(t, v)
	for _, absent := range []string{"#123456", "Cinzel", "Poppins", "--lv-background:#000000"} {
		if strings.Contains(html, absent) {
			t.Errorf("Signature tidak boleh memakai %q", absent)
		}
	}
	if !strings.Contains(html, "--lv-primary:#536b58") {
		t.Error("preset terpilih tidak dipakai")
	}
	// Preset tak dikenal → preset pertama, tidak pernah masuk ke CSS/URL mentah.
	v.Settings = view.Settings{ColorPreset: `x"><script>`}
	html = render(t, v)
	if !strings.Contains(html, "--lv-primary:#6b4e71") || !strings.Contains(html, "floral/signature-lavender/") || strings.Contains(html, `x"><script>`) {
		t.Error("preset tak dikenal harus jatuh ke preset pertama")
	}
}

// Urutan T33 §5, apa pun urutan tersimpan; penyembunyian tetap berlaku.
func TestSignatureSectionOrder(t *testing.T) {
	v := sample()
	v.ThemeID = "signature"
	v.AllowRSVP, v.AllowGuestbook = true, true
	v.Checkin.QRURL = "/i/ABC/qr.png"
	v.Gifts = []view.Gift{{Type: "bank", Provider: "BCA", AccountNumber: "123", AccountName: "A"}}
	v.Settings.SectionOrder = []string{"gift", "gallery", "couple", "rsvp", "events"}
	html := render(t, v)
	prev := -1
	for _, id := range []string{"opening", "undangan", "wedding-hero", "couple", "story", "events", "gallery", "rsvp", "checkin", "gift", "guestbook", "closing"} {
		i := strings.Index(html, `id="`+id+`"`)
		if i < 0 {
			t.Fatalf("bagian %s tidak ada", id)
		}
		if i < prev {
			t.Errorf("bagian %s tidak urut", id)
		}
		prev = i
	}
	v.Settings.HiddenSections = []string{"gallery"}
	if strings.Contains(render(t, v), `id="gallery"`) {
		t.Error("bagian yang disembunyikan tetap tampil")
	}
}

// Tema lain tetap menghormati kustomisasi & urutan pasangan, dan CSS-nya tidak
// memuat token khusus preset.
func TestOtherThemesKeepCustomization(t *testing.T) {
	for _, d := range All() {
		if d.Fixed {
			continue
		}
		v := sample()
		v.ThemeID = d.ID
		v.Settings = view.Settings{PrimaryColor: "#123456", ColorPreset: "sage-garden", SectionOrder: []string{"gallery", "couple"}}
		html := render(t, v)
		if !strings.Contains(html, "--lv-primary:#123456") {
			t.Errorf("%s: warna kustom hilang", d.ID)
		}
		for _, absent := range []string{"--lv-paper", "--lv-font-display", "floral/"} {
			if strings.Contains(html, absent) {
				t.Errorf("%s: tidak boleh memuat %q", d.ID, absent)
			}
		}
		if strings.Index(html, `id="gallery"`) > strings.Index(html, `id="couple"`) {
			t.Errorf("%s: urutan tersimpan tidak dipakai", d.ID)
		}
	}
}

func TestSignatureCheckinStates(t *testing.T) {
	base := func() view.View {
		v := sample()
		v.ThemeID = "signature"
		return v
	}
	// Tamu dengan QR: QR fungsional dalam kartu tamu.
	v := base()
	v.Checkin.QRURL = "/i/ABC/qr.png"
	if html := render(t, v); !strings.Contains(html, "Guest Pass") || !strings.Contains(html, `src="/i/ABC/qr.png"`) {
		t.Error("kartu tamu harus memuat QR")
	}
	// Link umum: info saja.
	v = base()
	v.Guest = nil
	v.Checkin.Available = true
	if html := render(t, v); !strings.Contains(html, "QR kehadiran pribadi Anda tersedia") || strings.Contains(html, "qr.png") {
		t.Error("link umum: info tanpa QR")
	}
	// Preview: contoh kartu tanpa gambar QR apa pun.
	v = base()
	v.Checkin.Sample = true
	if html := render(t, v); !strings.Contains(html, "Contoh — QR pribadi tamu") || strings.Contains(html, "qr.png") {
		t.Error("preview: contoh kartu tanpa QR palsu")
	}
	// Check-in mati: tidak tampil.
	if strings.Contains(render(t, base()), `id="checkin"`) {
		t.Error("check-in mati: bagian tidak boleh tampil")
	}
}
