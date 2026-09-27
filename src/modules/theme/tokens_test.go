package theme

import (
	"errors"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

func TestValidateSettings(t *testing.T) {
	ok, err := ValidateSettings(view.Settings{PrimaryColor: " #B76E79 ", FontHeading: "Great Vibes", FontBody: "Lora", Background: "#FFF6F6", CoverImage: "https://pub-x.r2.dev/a.jpg"})
	if err != nil || ok.PrimaryColor != "#b76e79" || ok.Background != "#fff6f6" {
		t.Fatalf("valid: %+v %v", ok, err)
	}
	if _, err := ValidateSettings(view.Settings{}); err != nil {
		t.Errorf("kosong (pakai default) harus valid: %v", err)
	}
	cases := map[string]view.Settings{
		"primary_color": {PrimaryColor: "red"},
		"font_heading":  {FontHeading: "Comic Sans MS"},
		"font_body":     {FontBody: "Great Vibes"}, // script hanya untuk judul
		"background":    {Background: `https://x.com/a.jpg"); color: red; (`},
		"cover_image":   {CoverImage: "javascript:alert(1)"},
	}
	for field, s := range cases {
		_, err := ValidateSettings(s)
		var se SettingsError
		errors.As(err, &se)
		if se[field] == "" {
			t.Errorf("%s harus ditolak, err = %v", field, err)
		}
	}
	for _, bad := range []string{"#12345", "#12345g", "rgb(0,0,0)", "#1234567"} {
		if _, err := ValidateSettings(view.Settings{PrimaryColor: bad}); err == nil {
			t.Errorf("warna %q harus ditolak", bad)
		}
	}
}

func TestTokensCSSAndFonts(t *testing.T) {
	css := TokensCSS("romantic", view.Tokens{Primary: "#b76e79", Surface: "#fff6f6", Ink: "#4a3b3b", FontHeading: "Great Vibes", FontBody: "Lora", Background: "https://pub-x.r2.dev/bg.jpg"})
	for _, want := range []string{`:root[data-theme="romantic"]{`, "--lv-primary:#b76e79;", `--lv-font-heading:"Great Vibes", cursive;`, `url("https://pub-x.r2.dev/bg.jpg")`} {
		if !strings.Contains(css, want) {
			t.Errorf("CSS tidak memuat %q:\n%s", want, css)
		}
	}
	// Nilai tak valid tidak pernah masuk CSS mentah.
	css = TokensCSS("x", view.Tokens{Primary: "red;}body{display:none", Background: `a"); }`})
	if strings.Contains(css, "display:none") || strings.Contains(css, `a")`) {
		t.Errorf("CSS injection lolos:\n%s", css)
	}

	u := GoogleFontsURL("Great Vibes", "Lora", "Lora", "Comic Sans")
	if u != "https://fonts.googleapis.com/css2?family=Great+Vibes:wght@400&family=Lora:wght@400;600&display=swap" {
		t.Errorf("fonts URL = %s", u)
	}
	if GoogleFontsURL("Comic Sans") != "" {
		t.Error("font di luar whitelist tidak boleh dimuat")
	}
}
