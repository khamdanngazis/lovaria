package theme

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/templates/themes/base"
)

// Kontras teks semua tema bawaan (T21): WCAG AA ≥ 4.5:1.
func TestThemeContrast(t *testing.T) {
	for _, d := range All() {
		tk := d.Tokens
		for _, c := range []struct {
			name   string
			fg, bg string
		}{
			{"Ink di atas Surface", tk.Ink, tk.Surface},
			{"Muted di atas Surface", tk.Muted, tk.Surface},
			{"Primary (judul/tautan) di atas Surface", tk.Primary, tk.Surface},
			{"teks tombol di atas Primary", OnColor(tk.Primary), tk.Primary},
			{"putih di atas Deep", "#ffffff", tk.Deep},
		} {
			if got := Contrast(c.fg, c.bg); got < MinContrast {
				t.Errorf("%s — %s: kontras %.2f < %.1f (%s di atas %s)", d.ID, c.name, got, MinContrast, c.fg, c.bg)
			}
		}
		if OnColor(tk.Primary) != "#ffffff" {
			t.Errorf("%s: Primary bawaan harus cukup gelap untuk teks putih", d.ID)
		}
		for name, v := range map[string]string{"Accent": tk.Accent, "Deep": tk.Deep, "Muted": tk.Muted, "Border": tk.Border} {
			if !hexColor.MatchString(v) {
				t.Errorf("%s: token %s belum diisi (%q)", d.ID, name, v)
			}
		}
	}
}

func TestContrastAndOnColor(t *testing.T) {
	if got := Contrast("#000000", "#ffffff"); got < 20.9 || got > 21.1 {
		t.Errorf("hitam/putih = %.2f, want 21", got)
	}
	if got := Contrast("#777777", "#ffffff"); got < 4.4 || got > 4.6 {
		t.Errorf("#777 di atas putih = %.2f, want ≈ 4.48", got)
	}
	// Primary kustom: gelap → teks putih; terang → teks gelap yang tetap terbaca.
	for bg, want := range map[string]string{"#6b4e71": "#ffffff", "#000000": "#ffffff", "#ffe680": onDark, "#ffffff": onDark, "#c9a88a": onDark} {
		if got := OnColor(bg); got != want {
			t.Errorf("OnColor(%s) = %s, want %s", bg, got, want)
		}
		if c := Contrast(OnColor(bg), bg); c < MinContrast {
			t.Errorf("OnColor(%s): kontras %.2f < 4.5", bg, c)
		}
	}
	css := TokensCSS("signature", resolve(registry["signature"].Tokens, view.Settings{PrimaryColor: "#ffe680"}))
	for _, want := range []string{"--lv-primary:#ffe680;", "--lv-on-primary:" + onDark + ";", "--lv-accent:#c9a88a;", "--lv-deep:#332936;", "--lv-muted:#6b666b;", "--lv-border:#e8dfd9;", "--lv-radius:0.25rem;", "--lv-radius-btn:0.5rem;"} {
		if !strings.Contains(css, want) {
			t.Errorf("TokensCSS tidak memuat %q:\n%s", want, css)
		}
	}
	// Nilai token tak dikenal → cadangan (tidak pernah disisipkan mentah).
	bad := TokensCSS("x", view.Tokens{Accent: "red;}", Radius: "1em;} body{display:none"})
	if strings.Contains(bad, "red") || strings.Contains(bad, "display") {
		t.Errorf("token tidak valid bocor ke CSS: %s", bad)
	}
}

func TestRegistrySignatureDefault(t *testing.T) {
	all := All()
	if len(all) != 5 || all[0].ID != "signature" || DefaultID != "signature" {
		t.Fatalf("tema = %d, pertama %q, bawaan %q", len(all), all[0].ID, DefaultID)
	}
	for _, d := range all {
		if d.Parts.SectionTitle == nil || d.Parts.Hero == nil || d.Parts.Closing == nil {
			t.Errorf("%s: bagian belum lengkap", d.ID)
		}
	}
	if got := GoogleFontsURL("Playfair Display", "Inter"); !strings.Contains(got, "family=Playfair+Display:ital,wght@0,400;0,500;1,400") || !strings.Contains(got, "family=Inter:wght@400;500;600") {
		t.Errorf("font Signature: %s", got)
	}
	// ID tak dikenal → Prepare memakai tema bawaan beserta judul bagiannya.
	if v := Prepare(view.View{ThemeID: "tidak-ada"}); v.ThemeID != "signature" || v.SectionTitle == nil {
		t.Errorf("Prepare: %+v", v.ThemeID)
	}
}

func TestInitials(t *testing.T) {
	for _, c := range []struct{ groom, bride, g, b string }{
		{"Raka Aditya", "Nadia Kirana", "R", "N"},
		{"budi", "siti", "B", "S"},
		{"Élodie", "山田 花子", "É", "山"},
		{"  'Aisyah", "", "A", ""},
		{"", "", "", ""},
	} {
		g, b := base.Initials(view.View{Couple: view.Couple{GroomName: c.groom, BrideName: c.bride}})
		if g != c.g || b != c.b {
			t.Errorf("Initials(%q, %q) = %q, %q", c.groom, c.bride, g, b)
		}
	}
}

var headingTag = regexp.MustCompile(`<h([1-3])[\s>]`)

// Semua tema benar pada data minim maupun penuh: satu h1, urutan heading tidak
// melompat (h1 → h2 → h3), sampul & monogram selalu ada.
func TestThemesWithMinimalAndFullData(t *testing.T) {
	long := "Muhammad Raka Aditya Pratama Wijayakusuma" // ≥ 30 karakter
	event := func(i int) view.Event {
		return view.Event{Name: fmt.Sprintf("Acara %d", i), TypeLabel: "Resepsi", DateText: "Sabtu, 12 Desember 2026", TimeText: "08.00 WIB", Venue: "Gedung"}
	}
	cases := map[string]func(*view.View){
		"minim: tanpa foto, cerita, galeri; satu acara": func(v *view.View) {
			v.Stories, v.Gallery, v.MainPhoto = nil, nil, ""
			v.Couple.GroomPhoto, v.Couple.BridePhoto = "", ""
			v.Events = []view.Event{event(1)}
		},
		"nama panjang, empat acara, foto lengkap": func(v *view.View) {
			v.Couple.GroomName, v.Couple.BrideName = long, long+" Putri"
			v.Guest = &view.Guest{Name: "Bapak " + long + " beserta Keluarga Besar"}
			v.MainPhoto = "https://pub-x.r2.dev/cover.jpg"
			v.CoverThumb, v.CoverWidth = "https://pub-x.r2.dev/cover_thumb.jpg", 1600
			v.Couple.GroomPhoto, v.Couple.BridePhoto = "https://pub-x.r2.dev/g.jpg", "https://pub-x.r2.dev/b.jpg"
			v.Events = []view.Event{event(1), event(2), event(3), event(4)}
			v.AllowRSVP, v.AllowGuestbook = true, true
			v.Gifts = []view.Gift{{Type: "bank", Provider: "BCA", AccountNumber: "123", AccountName: "A"}}
			v.Countdown = view.Countdown{Show: true, DaysLeft: 3, Target: time.Date(2026, 12, 12, 1, 0, 0, 0, time.UTC)}
			v.Settings.QuoteText = "Kasih itu sabar"
		},
		"tanpa acara sama sekali": func(v *view.View) { v.Events = nil },
	}
	for _, d := range All() {
		for name, mutate := range cases {
			t.Run(d.ID+"/"+name, func(t *testing.T) {
				v := sample()
				v.ThemeID = d.ID
				mutate(&v)
				html := render(t, v)
				if n := strings.Count(html, "<h1"); n != 1 {
					t.Errorf("h1 = %d, want 1", n)
				}
				prev := 0
				for _, m := range headingTag.FindAllStringSubmatch(html, -1) {
					level := int(m[1][0] - '0')
					if level > prev+1 {
						t.Fatalf("heading melompat dari h%d ke h%d", prev, level)
					}
					prev = level
				}
				for _, want := range []string{`id="opening"`, `id="closing"`, `data-open="lock"`, `href="#undangan"`, "<aside"} {
					if !strings.Contains(html, want) {
						t.Errorf("tidak ada %q", want)
					}
				}
				// Foto sampul di panel desktop selalu lazy (tidak diunduh di ponsel).
				if v.MainPhoto != "" && !strings.Contains(html, `loading="lazy"`) {
					t.Error("foto panel desktop harus lazy")
				}
				// Sampul yang memakai base.CoverImage (Signature & base; tema lama
				// menyusul di PR berikutnya): prioritas tinggi + srcset.
				if v.CoverThumb != "" && strings.Contains(html, "lv-kenburns") {
					if !strings.Contains(html, `fetchpriority="high"`) || !strings.Contains(html, "cover_thumb.jpg 480w, https://pub-x.r2.dev/cover.jpg 1600w") {
						t.Error("foto sampul: fetchpriority / srcset")
					}
				}
			})
		}
	}
}

// Signature meng-override semua bagian visual & judul bagian bersama memakai
// ornamen tema; tema lain tetap memakai judulnya sendiri.
func TestSignatureThemeAndSharedTitles(t *testing.T) {
	v := sample()
	v.ThemeID = "signature"
	v.MainPhoto = "https://pub-x.r2.dev/cover.jpg"
	v.AllowRSVP, v.AllowGuestbook = true, true
	v.Gifts = []view.Gift{{Type: "bank", Provider: "BCA", AccountNumber: "123", AccountName: "A"}}
	html := render(t, v)
	section := func(id string) string {
		i := strings.Index(html, `id="`+id+`"`)
		if i < 0 {
			t.Fatalf("bagian %s tidak ada", id)
		}
		return html[i : i+strings.Index(html[i:], "</section>")]
	}
	// Bagian bersama: judul Signature (bintang SVG + h2 besar), bukan judul base.
	for _, id := range []string{"rsvp", "guestbook", "gift", "couple", "story", "events", "gallery"} {
		if s := section(id); !strings.Contains(s, "<svg") || !strings.Contains(s, "text-4xl leading-tight") {
			t.Errorf("bagian %s belum memakai judul tema Signature", id)
		}
	}
	for _, want := range []string{"bg-deep", "Samuel &amp; Sarah", ">K<", ">S<", "Save the date", "12", `class="lv-btn`, "lv-card", "lv-input"} {
		if !strings.Contains(html, want) {
			t.Errorf("Signature: tidak ada %q", want)
		}
	}
	if !strings.Contains(section("closing"), "bg-deep") || !strings.Contains(section("gallery"), "bg-deep") {
		t.Error("penutup & galeri Signature memakai latar Deep")
	}
	// Preview dashboard: sampul tidak dikunci; Kenangan: tombol menuju #memories.
	v.Preview = true
	if strings.Contains(render(t, v), `data-open="lock"`) {
		t.Error("preview tidak boleh mengunci sampul")
	}
	v.Preview, v.Memory = false, true
	v.MemoryPhotos = []view.Photo{{URL: "https://pub-x.r2.dev/m.jpg", ThumbURL: "https://pub-x.r2.dev/m_thumb.jpg"}}
	if html := render(t, v); !strings.Contains(html, `href="#memories" data-open="lock"`) || !strings.Contains(html, "Lihat Kenangan") {
		t.Error("kenangan: tombol pembuka harus menuju #memories")
	}
}
