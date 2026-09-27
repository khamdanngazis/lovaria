package theme

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

// Font adalah font yang diizinkan (Google Fonts) beserta cadangannya.
type Font struct {
	Name     string // nama tampilan & nilai tersimpan
	Weights  string // mis. "400;600"
	Fallback string // stack CSS cadangan
	Script   bool   // font tulisan tangan (hanya cocok untuk judul)
}

// Fonts adalah whitelist font. Nilai di luar daftar ini ditolak.
var Fonts = []Font{
	{Name: "Cormorant Garamond", Weights: "400;600", Fallback: "Georgia, serif"},
	{Name: "Playfair Display", Weights: "400;600", Fallback: "Georgia, serif"},
	{Name: "Cinzel", Weights: "400;600", Fallback: "Georgia, serif"},
	{Name: "Lora", Weights: "400;600", Fallback: "Georgia, serif"},
	{Name: "Great Vibes", Weights: "400", Fallback: "cursive", Script: true},
	{Name: "Dancing Script", Weights: "400;600", Fallback: "cursive", Script: true},
	{Name: "Montserrat", Weights: "400;600", Fallback: "system-ui, sans-serif"},
	{Name: "Josefin Sans", Weights: "400;600", Fallback: "system-ui, sans-serif"},
	{Name: "Poppins", Weights: "400;600", Fallback: "system-ui, sans-serif"},
	{Name: "Inter", Weights: "400;600", Fallback: "system-ui, sans-serif"},
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
		families = append(families, "family="+strings.ReplaceAll(f.Name, " ", "+")+":wght@"+f.Weights)
	}
	if len(families) == 0 {
		return ""
	}
	return "https://fonts.googleapis.com/css2?" + strings.Join(families, "&") + "&display=swap"
}
