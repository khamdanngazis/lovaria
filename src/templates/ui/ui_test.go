package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/theme"
)

const stylesheet = "../../styles/app.css"

// brandTokens membaca token --color-lovoria-* dari @theme di app.css.
func brandTokens(t *testing.T) map[string]string {
	t.Helper()
	css, err := os.ReadFile(stylesheet)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, m := range regexp.MustCompile(`--color-lovoria-([a-z-]+):\s*(#[0-9a-fA-F]{6});`).FindAllStringSubmatch(string(css), -1) {
		out[m[1]] = strings.ToLower(m[2])
	}
	return out
}

// Kontras token dashboard & auth (T22): teks ≥ 4.5:1 (WCAG AA), tepi kontrol
// form ≥ 3:1 di atas putih.
func TestBrandTokenContrast(t *testing.T) {
	tk := brandTokens(t)
	const white = "#ffffff"
	pairs := []struct {
		fg, bg string
		min    float64
	}{
		{"text", "bg", 4.5}, {"text", "surface", 4.5}, {"text", "subtle", 4.5}, {"text", "primary-soft", 4.5},
		{"muted", "bg", 4.5}, {"muted", "surface", 4.5}, {"muted", "subtle", 4.5},
		{"deep", "bg", 4.5}, {"deep", "surface", 4.5},
		{"primary", "bg", 4.5}, {"primary", "surface", 4.5}, {"primary", "primary-soft", 4.5},
		{"accent-ink", "bg", 4.5}, {"accent-ink", "surface", 4.5},
		{"success", "success-soft", 4.5}, {"success", "surface", 4.5},
		{"warning", "warning-soft", 4.5}, {"warning", "surface", 4.5},
		{"danger", "danger-soft", 4.5}, {"danger", "surface", 4.5},
		{"info", "info-soft", 4.5}, {"info", "surface", 4.5},
		{"control", "surface", 3},
	}
	for _, p := range pairs {
		fg, bg := tk[p.fg], tk[p.bg]
		if fg == "" || bg == "" {
			t.Errorf("token lovoria-%s / lovoria-%s tidak ada di app.css", p.fg, p.bg)
			continue
		}
		if got := theme.Contrast(fg, bg); got < p.min {
			t.Errorf("lovoria-%s (%s) di atas lovoria-%s (%s): kontras %.2f < %.1f", p.fg, fg, p.bg, bg, got, p.min)
		}
	}
	// Teks putih di atas warna pekat (tombol utama, tombol hapus, panel brand).
	for _, bg := range []string{"primary", "primary-hover", "deep", "danger"} {
		if got := theme.Contrast(white, tk[bg]); got < 4.5 {
			t.Errorf("putih di atas lovoria-%s (%s): kontras %.2f < 4.5", bg, tk[bg], got)
		}
	}
}

// migrated: berkas templ yang sudah beralih ke token lovoria-* & kelas ui-*
// (T22). Daftar bertambah tiap PR sampai seluruh dashboard/auth/admin tercakup.
var migrated = []string{
	"form.templ",
	"../../modules/auth/views.templ",
}

var (
	// Kelas palet Tailwind mentah: bg-slate-50, text-red-600, border-amber-500, …
	rawPalette = regexp.MustCompile(`\b(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)-\d{2,3}\b`)
	// Token TEMA UNDANGAN (berubah per wedding) — di dashboard pakai lovoria-*.
	themeToken = regexp.MustCompile(`(?:^|[\s"':])(?:[a-z-]+:)*(?:bg|text|border|ring|outline|accent|divide|from|to|fill|stroke)-(?:primary|surface|ink|on-primary)\b`)
)

func TestNoRawPaletteInMigratedTemplates(t *testing.T) {
	for _, rel := range migrated {
		b, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if m := rawPalette.FindString(line); m != "" {
				t.Errorf("%s:%d: kelas palet mentah %q — pakai token lovoria-* / kelas ui-*", rel, i+1, m)
			}
			if m := themeToken.FindString(line); m != "" {
				t.Errorf("%s:%d: token tema undangan %q di dashboard — pakai lovoria-*", rel, i+1, strings.TrimSpace(m))
			}
		}
	}
}

// Kelas komponen ui-* yang dipakai templ harus terdefinisi di app.css.
func TestUIClassesDefined(t *testing.T) {
	css, err := os.ReadFile(stylesheet)
	if err != nil {
		t.Fatal(err)
	}
	used := regexp.MustCompile(`\bui-[a-z]+(?:-[a-z]+)*\b`)
	for _, rel := range migrated {
		b, err := os.ReadFile(filepath.FromSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		for _, class := range used.FindAllString(string(b), -1) {
			if !strings.Contains(string(css), "."+class+" ") && !strings.Contains(string(css), "."+class+",") && !strings.Contains(string(css), "."+class+"[") {
				t.Errorf("%s memakai .%s yang tidak didefinisikan di app.css", rel, class)
			}
		}
	}
}
