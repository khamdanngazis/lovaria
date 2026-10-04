package theme

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

// Font adalah font yang diizinkan (Google Fonts) beserta cadangannya.
type Font struct {
	Name     string // nama tampilan & nilai tersimpan
	Weights  string // mis. "400;600"
	Fallback string // stack CSS cadangan
	Script   bool   // font tulisan tangan (hanya cocok untuk judul)
	Italic   bool   // muat juga gaya miring 400 (judul editorial)
}

// Fonts adalah whitelist font. Nilai di luar daftar ini ditolak.
var Fonts = []Font{
	{Name: "Cormorant Garamond", Weights: "400;600", Fallback: "Georgia, serif", Italic: true},
	{Name: "Playfair Display", Weights: "400;500", Fallback: "Georgia, serif", Italic: true},
	{Name: "Cinzel", Weights: "400;600", Fallback: "Georgia, serif"},
	{Name: "Lora", Weights: "400;600", Fallback: "Georgia, serif"},
	{Name: "Great Vibes", Weights: "400", Fallback: "cursive", Script: true},
	{Name: "Dancing Script", Weights: "400;600", Fallback: "cursive", Script: true},
	{Name: "Montserrat", Weights: "400;600", Fallback: "system-ui, sans-serif"},
	{Name: "Josefin Sans", Weights: "400;600", Fallback: "system-ui, sans-serif"},
	{Name: "Poppins", Weights: "400;600", Fallback: "system-ui, sans-serif"},
	{Name: "Inter", Weights: "400;500;600", Fallback: "system-ui, sans-serif"},
	{Name: "Lato", Weights: "400;700", Fallback: "system-ui, sans-serif"},
	{Name: "Nunito", Weights: "400;600", Fallback: "system-ui, sans-serif"},
}

// FontByName mengembalikan font dari whitelist.
func FontByName(name string) (Font, bool) {
	for _, f := range Fonts {
		if f.Name == name {
			return f, true
		}
	}
	return Font{}, false
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// SettingsError memetakan field pengaturan ke pesan error.
type SettingsError map[string]string

func (e SettingsError) Error() string {
	parts := make([]string, 0, len(e))
	for k, v := range e {
		parts = append(parts, k+": "+v)
	}
	sort.Strings(parts)
	return "pengaturan tema tidak valid: " + strings.Join(parts, "; ")
}

// ValidateSettings memeriksa & menormalkan pengaturan (warna huruf kecil, spasi dibuang).
func ValidateSettings(s view.Settings) (view.Settings, error) {
	e := SettingsError{}
	s.PrimaryColor = strings.ToLower(strings.TrimSpace(s.PrimaryColor))
	if s.PrimaryColor != "" && !hexColor.MatchString(s.PrimaryColor) {
		e["primary_color"] = "Warna harus format hex, mis. #b76e79"
	}
	for key, name := range map[string]*string{"font_heading": &s.FontHeading, "font_body": &s.FontBody} {
		*name = strings.TrimSpace(*name)
		if *name == "" {
			continue
		}
		f, ok := FontByName(*name)
		switch {
		case !ok:
			e[key] = "Font tidak tersedia"
		case f.Script && key == "font_body":
			e[key] = "Font tulisan tangan hanya untuk judul"
		}
	}
	s.Background = strings.TrimSpace(s.Background)
	if s.Background != "" {
		if strings.HasPrefix(s.Background, "#") {
			s.Background = strings.ToLower(s.Background)
			if !hexColor.MatchString(s.Background) {
				e["background"] = "Warna latar harus format hex"
			}
		} else if !safeImageURL(s.Background) {
			e["background"] = "Latar harus warna hex atau URL gambar https://"
		}
	}
	s.CoverImage = strings.TrimSpace(s.CoverImage)
	if s.CoverImage != "" && !safeImageURL(s.CoverImage) {
		e["cover_image"] = "URL foto sampul harus diawali https://"
	}
	validatePersonalization(&s, e)
	if len(e) > 0 {
		return view.Settings{}, e
	}
	return s, nil
}

// safeImageURL: http(s) absolut atau path lokal /media/..., tanpa karakter yang
// bisa keluar dari url("...") di CSS.
func safeImageURL(raw string) bool {
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, "\"'()\\<>\n\r\t ") {
		return false
	}
	if strings.HasPrefix(raw, "/media/") {
		return true // driver storage lokal (development)
	}
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// resolve menggabungkan token default tema dengan pengaturan wedding.
func resolve(def view.Tokens, s view.Settings) view.Tokens {
	t := def
	if s.PrimaryColor != "" {
		t.Primary = s.PrimaryColor
	}
	if s.FontHeading != "" {
		t.FontHeading = s.FontHeading
	}
	if s.FontBody != "" {
		t.FontBody = s.FontBody
	}
	if s.Background != "" {
		t.Background = s.Background
	}
	if s.CoverImage != "" {
		t.CoverImage = s.CoverImage
	}
	return t
}

func fontStack(name string) string {
	f, ok := FontByName(name)
	if !ok {
		return "system-ui, sans-serif"
	}
	return fmt.Sprintf("%q, %s", f.Name, f.Fallback)
}

// TokensCSS membuat blok CSS variable untuk tema. Semua nilai sudah divalidasi
// (hex/whitelist/URL aman), sehingga aman disisipkan ke <style>.
func TokensCSS(themeID string, t view.Tokens) string {
	var b strings.Builder
	fmt.Fprintf(&b, `:root[data-theme=%q]{`, themeID)
	fmt.Fprintf(&b, "--lv-primary:%s;", colorOr(t.Primary, "#b76e79"))
	fmt.Fprintf(&b, "--lv-surface:%s;", colorOr(t.Surface, "#ffffff"))
	fmt.Fprintf(&b, "--lv-ink:%s;", colorOr(t.Ink, "#222222"))
	// Token desain tema (T21). Cadangan memakai palet brand Lunovia.
	fmt.Fprintf(&b, "--lv-accent:%s;", colorOr(t.Accent, "#c9a88a"))
	fmt.Fprintf(&b, "--lv-deep:%s;", colorOr(t.Deep, "#332936"))
	fmt.Fprintf(&b, "--lv-muted:%s;", colorOr(t.Muted, "#6b666b"))
	fmt.Fprintf(&b, "--lv-border:%s;", colorOr(t.Border, "#e8dfd9"))
	fmt.Fprintf(&b, "--lv-on-primary:%s;", OnColor(colorOr(t.Primary, "#b76e79")))
	fmt.Fprintf(&b, "--lv-radius:%s;", lengthOr(t.Radius, "1rem"))
	fmt.Fprintf(&b, "--lv-radius-btn:%s;", lengthOr(t.ButtonRadius, "9999px"))
	fmt.Fprintf(&b, "--lv-font-heading:%s;", fontStack(t.FontHeading))
	fmt.Fprintf(&b, "--lv-font-body:%s;", fontStack(t.FontBody))
	switch {
	case hexColor.MatchString(t.Background):
		fmt.Fprintf(&b, "--lv-background:%s;", t.Background)
	case safeImageURL(t.Background):
		fmt.Fprintf(&b, `--lv-background:%s url("%s") center/cover fixed;`, colorOr(t.Surface, "#ffffff"), t.Background)
	default:
		fmt.Fprintf(&b, "--lv-background:%s;", colorOr(t.Surface, "#ffffff"))
	}
	b.WriteString("}")
	return b.String()
}

var cssLength = regexp.MustCompile(`^(0|[0-9]+(\.[0-9]+)?(rem|px))$`)

func lengthOr(l, def string) string {
	if cssLength.MatchString(l) {
		return l
	}
	return def
}

// ---------- Kontras (WCAG 2.x) ----------

// luminance: luminansi relatif warna #rrggbb (0 hitam … 1 putih).
func luminance(hex string) float64 {
	if !hexColor.MatchString(hex) {
		return 0
	}
	var rgb [3]float64
	for i := range rgb {
		n, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
		c := float64(n) / 255
		if c <= 0.03928 {
			rgb[i] = c / 12.92
		} else {
			rgb[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
}

// Contrast: rasio kontras dua warna #rrggbb (1 … 21).
func Contrast(a, b string) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// Warna teks di atas warna pekat (tombol, bagian Primary).
const (
	onLight = "#ffffff"
	onDark  = "#1f1a20"
	// MinContrast: kontras teks minimum (WCAG AA untuk teks normal).
	MinContrast = 4.5
)

// OnColor: warna teks yang terbaca di atas bg — putih bila kontrasnya cukup,
// selain itu gelap. Dipakai untuk --lv-on-primary supaya Primary kustom yang
// terang (mis. kuning muda) tidak membuat teks tombol hilang.
func OnColor(bg string) string {
	if Contrast(onLight, bg) >= MinContrast || Contrast(onLight, bg) >= Contrast(onDark, bg) {
		return onLight
	}
	return onDark
}

func colorOr(c, def string) string {
	if hexColor.MatchString(c) {
		return c
	}
	return def
}

// GoogleFontsURL memuat hanya font yang dipakai (judul & isi), tanpa duplikat.
func GoogleFontsURL(fonts ...string) string {
	seen := map[string]bool{}
	var families []string
	for _, name := range fonts {
		f, ok := FontByName(name)
		if !ok || seen[f.Name] {
			continue
		}
		seen[f.Name] = true
		axis := ":wght@" + f.Weights
		if f.Italic {
			// ital,wght@0,400;0,500;1,400 — tegak semua bobot + miring 400.
			axis = ":ital,wght@0," + strings.ReplaceAll(f.Weights, ";", ";0,") + ";1,400"
		}
		families = append(families, "family="+strings.ReplaceAll(f.Name, " ", "+")+axis)
	}
	if len(families) == 0 {
		return ""
	}
	return "https://fonts.googleapis.com/css2?" + strings.Join(families, "&") + "&display=swap"
}
