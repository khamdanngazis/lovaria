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
		for _, part := range []string{"bouquet", "cluster", "corner", "ribbon-left", "ribbon", "ribbon2", "sprig", "rose", "petal-a", "petal-b", "petal-c"} {
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

// Pergantian warna tidak boleh mengubah identitas layout (T33 §4.6, T37): HTML
// semua preset identik setelah token CSS & path floral dinormalkan.
func TestPresetsOnlyChangeColors(t *testing.T) {
	normalize := func(html string) string {
		return floralRef.ReplaceAllString(cssBlock.ReplaceAllString(html, ""), "floral/$1")
	}
	for _, d := range All() {
		var first string
		for i, p := range d.Presets {
			v := sample()
			v.ThemeID = d.ID
			v.Settings.ColorPreset = p.ID
			html := render(t, v)
			if !strings.Contains(html, "--lv-primary:"+p.Primary) || !strings.Contains(html, "--lv-soft:"+p.Soft) {
				t.Errorf("%s/%s: token preset tidak dipakai", d.ID, p.ID)
			}
			if d.ID == "signature" && !strings.Contains(html, "floral/"+p.ID+"/") {
				t.Errorf("%s: floral preset tidak dipakai", p.ID)
			}
			if i == 0 {
				first = normalize(html)
			} else if normalize(html) != first {
				t.Errorf("%s/%s: struktur HTML berbeda dari preset pertama", d.ID, p.ID)
			}
		}
	}
}

var hexColor6 = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Setiap tema punya tepat 5 tema warna lengkap (T37); token bawaan = preset pertama.
func TestPresetTokens(t *testing.T) {
	for _, d := range All() {
		if len(d.Presets) != 5 {
			t.Errorf("%s: %d preset, want 5", d.ID, len(d.Presets))
		}
		seen := map[string]bool{}
		for _, p := range d.Presets {
			if seen[p.ID] || p.ID == "" || p.Name == "" || p.Character == "" {
				t.Errorf("%s: preset %q tidak unik / tanpa nama", d.ID, p.ID)
			}
			seen[p.ID] = true
			for name, c := range map[string]string{"Primary": p.Primary, "Secondary": p.Secondary, "Accent": p.Accent, "Surface": p.Surface, "Deep": p.Deep, "Ink": p.Ink, "Muted": p.Muted, "Border": p.Border, "Paper": p.Paper, "Soft": p.Soft} {
				if !hexColor6.MatchString(c) {
					t.Errorf("%s/%s: %s %q bukan hex", d.ID, p.ID, name, c)
				}
			}
			for _, c := range []struct {
				name   string
				fg, bg string
			}{{"Ink di atas Paper", p.Ink, p.Paper}, {"Muted di atas Paper", p.Muted, p.Paper}, {"Ink di atas Soft", p.Ink, p.Soft}, {"Deep di atas Soft", p.Deep, p.Soft}} {
				if got := Contrast(c.fg, c.bg); got < MinContrast {
					t.Errorf("%s/%s: %s %.2f < %.1f", d.ID, p.ID, c.name, got, MinContrast)
				}
			}
		}
		if len(d.Presets) > 0 && d.Tokens.Preset != d.Presets[0].ID {
			t.Errorf("%s: token bawaan bukan preset pertama (%q)", d.ID, d.Tokens.Preset)
		}
	}
}

// Preset pertama = warna bawaan sebelum T37, supaya undangan yang sudah ada
// (tanpa preset tersimpan) tidak berubah.
func TestDefaultPresetKeepsPreviousColors(t *testing.T) {
	prev := map[string][7]string{ // Primary, Surface, Ink, Accent, Deep, Muted, Border
		"elegant":  {"#8a6a3c", "#fbf8f3", "#2b2b2b", "#c9a88a", "#2e2620", "#6a6258", "#e6dccb"},
		"minimal":  {"#5f5661", "#ffffff", "#1f1f1f", "#b9b2b0", "#1f1f1f", "#666666", "#e5e5e5"},
		"romantic": {"#a85a67", "#fff6f6", "#4a3b3b", "#c9a88a", "#5a3540", "#735e5e", "#f0dcdc"},
		"modern":   {"#2f7d6d", "#f4f6f5", "#16201e", "#c9a88a", "#1d4a43", "#55605d", "#dde3e1"},
		"jawa":     {"#7a4a26", "#faf5ec", "#2e2118", "#c9a063", "#3a2718", "#6b5a4a", "#e6d9c4"},
		"sunda":    {"#5d7256", "#f8f6ef", "#2f352c", "#b9a77c", "#36452f", "#5f6659", "#e2e4d6"},
		"minang":   {"#7b1e2b", "#fbf6ee", "#2a1a1a", "#c9a24b", "#4a0f1b", "#6b5555", "#ead9c8"},
		"bali":     {"#9a4a2f", "#faf4ea", "#2f2319", "#b8975a", "#3d2a1c", "#6a5a4b", "#e8dac6"},
		"bugis":    {"#6e1630", "#fbf5ee", "#24161a", "#d1a94a", "#3a0d1c", "#675257", "#ead8cf"},
	}
	for id, want := range prev {
		tk := resolve(registry[id], view.Settings{})
		got := [7]string{tk.Primary, tk.Surface, tk.Ink, tk.Accent, tk.Deep, tk.Muted, tk.Border}
		if got != want {
			t.Errorf("%s: warna bawaan berubah: %v, want %v", id, got, want)
		}
		if css := TokensCSS(id, tk); !strings.Contains(css, "--lv-background:"+want[1]+";") {
			t.Errorf("%s: latar bawaan berubah: %s", id, css)
		}
	}
}

// Tema tidak Fixed: token tema → preset → warna utama & latar sendiri (T37 §2).
func TestResolvePresetThenCustom(t *testing.T) {
	d := registry["elegant"]
	navy, _ := d.Preset("navy-gold")
	tk := resolve(d, view.Settings{ColorPreset: "navy-gold"})
	if tk.Primary != navy.Primary || tk.Deep != navy.Deep || tk.Background != navy.Surface {
		t.Errorf("preset tidak diterapkan: %+v", tk)
	}
	tk = resolve(d, view.Settings{ColorPreset: "navy-gold", PrimaryColor: "#123456", Background: "#000000"})
	if tk.Primary != "#123456" || tk.Background != "#000000" || tk.Deep != navy.Deep {
		t.Errorf("warna sendiri harus menimpa preset: %+v", tk)
	}
	if resolve(d, view.Settings{ColorPreset: "sage-garden"}) != d.Tokens {
		t.Error("preset tema lain harus diabaikan")
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
		for _, absent := range []string{"--lv-font-display", "floral/"} {
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
	v.Guest.Code = "ABC1234"
	html := render(t, v)
	for _, want := range []string{"Wedding Guest Pass", "Samuel &amp; Sarah", `src="/i/ABC/qr.png"`, "ABC1234", "Tunjukkan QR ini kepada penerima tamu", "Simpan QR", "floral/signature-lavender/rose.webp"} {
		if !strings.Contains(html, want) {
			t.Errorf("kartu tamu tidak memuat %q", want)
		}
	}
	// Link umum: info saja.
	v = base()
	v.Guest = nil
	v.Checkin.Available = true
	if html := render(t, v); !strings.Contains(html, "QR kehadiran pribadi Anda tersedia") || strings.Contains(html, "qr.png") {
		t.Error("link umum: info tanpa QR")
	}
	// Preview: contoh kartu bergambar QR contoh bertanda CONTOH (T38), bukan QR tamu.
	v = base()
	v.Checkin.Sample, v.Checkin.SampleQRURL = true, "/static/img/sample-qr.png"
	if html := render(t, v); !strings.Contains(html, `src="/static/img/sample-qr.png"`) || !strings.Contains(html, ">Contoh<") || strings.Contains(html, "/qr.png") || strings.Contains(html, "Simpan QR") {
		t.Error("preview: contoh kartu dengan QR contoh, tanpa QR tamu & tombol simpan")
	}
	// Check-in mati: tidak tampil.
	if strings.Contains(render(t, base()), `id="checkin"`) {
		t.Error("check-in mati: bagian tidak boleh tampil")
	}
}

// QR kehadiran dalam tiga keadaan di semua tema (T31, T33, T38).
func TestCheckinStatesAllThemes(t *testing.T) {
	for _, d := range All() {
		base := func() view.View {
			v := sample()
			v.ThemeID = d.ID
			v.Guest = &view.Guest{Name: "Bapak Budi", Code: "ABC1234", MaxPax: 3}
			return v
		}
		// QR asli (link pribadi).
		v := base()
		v.Checkin.QRURL = "/i/ABC1234/qr.png"
		html := render(t, v)
		for _, want := range []string{`id="checkin"`, `src="/i/ABC1234/qr.png"`, "Bapak Budi", "Berlaku untuk 3 orang", "ABC1234", "Simpan QR"} {
			if !strings.Contains(html, want) {
				t.Errorf("%s: QR asli tidak memuat %q", d.ID, want)
			}
		}
		// Contoh (preview dashboard, check-in menyala).
		v = base()
		v.Checkin.Sample, v.Checkin.SampleQRURL = true, "/static/img/sample-qr.png"
		html = render(t, v)
		if !strings.Contains(html, `id="checkin"`) || !strings.Contains(html, `src="/static/img/sample-qr.png"`) || !strings.Contains(html, ">Contoh<") || strings.Contains(html, "Simpan QR") || strings.Contains(html, "/qr.png") {
			t.Errorf("%s: contoh kartu QR salah", d.ID)
		}
		// Info (link umum).
		v = base()
		v.Checkin.Available = true
		if html := render(t, v); !strings.Contains(html, "QR kehadiran pribadi Anda tersedia") || strings.Contains(html, "qr.png") {
			t.Errorf("%s: link umum harus info tanpa QR", d.ID)
		}
		// Check-in mati: tidak tampil. Bagian ini tidak bisa disembunyikan; tampil
		// tidaknya diatur saklar check-in.
		if strings.Contains(render(t, base()), `id="checkin"`) {
			t.Errorf("%s: check-in mati tidak boleh tampil", d.ID)
		}
	}
}
