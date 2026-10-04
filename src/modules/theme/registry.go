// Package theme: registry tema undangan, token tampilan per wedding, dan
// Render — satu-satunya pintu masuk merender halaman undangan (Arsitektur §6).
// Tidak boleh ada logic "tema X pakai layout Y" di luar paket ini.
package theme

import (
	"sort"

	"github.com/a-h/templ"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/templates/themes/base"
	"github.com/khamdanngazis/lovaria/src/templates/themes/elegant"
	"github.com/khamdanngazis/lovaria/src/templates/themes/minimal"
	"github.com/khamdanngazis/lovaria/src/templates/themes/modern"
	"github.com/khamdanngazis/lovaria/src/templates/themes/romantic"
	"github.com/khamdanngazis/lovaria/src/templates/themes/signature"
)

// Part adalah satu bagian halaman undangan.
type Part func(v view.View) templ.Component

// Parts adalah komponen sebuah tema. Bagian yang nil diisi dari tema base.
// Layout membungkus bagian lain (templ children).
type Parts struct {
	Layout, Hero, Couple, LoveStory, Events, Gallery, Closing Part
	// SectionTitle: judul bagian milik tema — dipakai juga oleh bagian bersama
	// (hitung mundur, RSVP, ucapan, hadiah, kenangan) lewat view.SectionTitle.
	SectionTitle func(title, subtitle string) templ.Component
}

// ThemeDef mendefinisikan satu tema.
type ThemeDef struct {
	ID          string
	Name        string
	Description string
	Tokens      view.Tokens
	Parts       Parts
	// Islands: komponen interaktif berat (Svelte) yang dimuat tema ini — kosong di MVP.
	Islands []string
	order   int
}

// DefaultID adalah tema bila theme_id wedding tidak dikenal.
const DefaultID = "signature"

var registry = map[string]ThemeDef{}

// register menambah tema. Menambah tema baru = folder templates/themes/<id> +
// satu panggilan register di init() di bawah (lihat doc/themes.md).
func register(d ThemeDef) {
	b := d.Parts
	fill := func(p *Part, def Part) {
		if *p == nil {
			*p = def
		}
	}
	fill(&b.Layout, base.Layout)
	fill(&b.Hero, base.Hero)
	fill(&b.Couple, base.Couple)
	fill(&b.LoveStory, base.LoveStory)
	fill(&b.Events, base.Events)
	fill(&b.Gallery, base.Gallery)
	fill(&b.Closing, base.Closing)
	if b.SectionTitle == nil {
		b.SectionTitle = base.SectionTitle
	}
	d.Parts = b
	d.order = len(registry)
	registry[d.ID] = d
}

// Warna bawaan dipilih agar kontras teks ≥ 4.5:1 (WCAG AA): Ink & Muted di atas
// Surface, putih di atas Primary dan Deep (ditegakkan TestThemeContrast).
// Accent (champagne) hanya untuk ornamen/garis, bukan teks isi atau tombol.
func init() {
	register(ThemeDef{
		ID: "signature", Name: "Lovoria Signature", Description: "Editorial, hangat, dan premium — wajah Lovoria.",
		Tokens: view.Tokens{
			Primary: "#6b4e71", Surface: "#faf7f5", Ink: "#292529", Accent: "#c9a88a", Deep: "#332936", Muted: "#6b666b", Border: "#e8dfd9",
			FontHeading: "Playfair Display", FontBody: "Inter", Radius: "0.25rem", ButtonRadius: "0.5rem",
		},
		Parts: Parts{
			Hero: signature.Hero, Couple: signature.Couple, LoveStory: signature.LoveStory, Events: signature.Events,
			Gallery: signature.Gallery, Closing: signature.Closing, SectionTitle: signature.SectionTitle,
		},
	})
	register(ThemeDef{
		ID: "elegant", Name: "Elegan", Description: "Klasik dengan aksen emas, serif, dan bingkai tipis.",
		Tokens: view.Tokens{
			Primary: "#8a6a3c", Surface: "#fbf8f3", Ink: "#2b2b2b", Accent: "#c9a88a", Deep: "#2e2620", Muted: "#6a6258", Border: "#e6dccb",
			FontHeading: "Cormorant Garamond", FontBody: "Lato",
		},
		Parts: Parts{Hero: elegant.Hero, Couple: elegant.Couple, Events: elegant.Events},
	})
	register(ThemeDef{
		ID: "minimal", Name: "Minimalis", Description: "Bersih dan lega, huruf kapital, tanpa ornamen.",
		Tokens: view.Tokens{
			Primary: "#707070", Surface: "#ffffff", Ink: "#1f1f1f", Accent: "#b9b2b0", Deep: "#1f1f1f", Muted: "#666666", Border: "#e5e5e5",
			FontHeading: "Josefin Sans", FontBody: "Inter",
		},
		Parts: Parts{Hero: minimal.Hero, Couple: minimal.Couple, Closing: minimal.Closing},
	})
	register(ThemeDef{
		ID: "romantic", Name: "Romantis", Description: "Lembut dengan warna merah muda, tulisan tangan, foto melengkung.",
		Tokens: view.Tokens{
			Primary: "#a85a67", Surface: "#fff6f6", Ink: "#4a3b3b", Accent: "#c9a88a", Deep: "#5a3540", Muted: "#735e5e", Border: "#f0dcdc",
			FontHeading: "Great Vibes", FontBody: "Lora",
		},
		Parts: Parts{Hero: romantic.Hero, Couple: romantic.Couple, Closing: romantic.Closing},
	})
	register(ThemeDef{
		ID: "modern", Name: "Modern", Description: "Tegas dengan blok warna penuh dan huruf sans tebal.",
		Tokens: view.Tokens{
			Primary: "#2f7d6d", Surface: "#f4f6f5", Ink: "#16201e", Accent: "#c9a88a", Deep: "#16201e", Muted: "#55605d", Border: "#dde3e1",
			FontHeading: "Montserrat", FontBody: "Poppins",
		},
		Parts: Parts{Hero: modern.Hero, Couple: modern.Couple, Events: modern.Events},
	})
}

// Get mengembalikan tema; ok=false bila ID tidak terdaftar.
func Get(id string) (ThemeDef, bool) {
	d, ok := registry[id]
	return d, ok
}

// All mengembalikan semua tema terurut sesuai pendaftaran.
func All() []ThemeDef {
	out := make([]ThemeDef, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].order < out[j].order })
	return out
}

// Exists: apakah ID tema terdaftar.
func Exists(id string) bool {
	_, ok := registry[id]
	return ok
}
