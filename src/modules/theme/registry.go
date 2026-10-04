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
		ID: "elegant", Name: "Elegan", Description: "Klasik formal: bingkai ganda, ornamen sudut, serba rata tengah.",
		Tokens: view.Tokens{
			Primary: "#8a6a3c", Surface: "#fbf8f3", Ink: "#2b2b2b", Accent: "#c9a88a", Deep: "#2e2620", Muted: "#6a6258", Border: "#e6dccb",
			FontHeading: "Cormorant Garamond", FontBody: "Lato", Radius: "0.125rem", ButtonRadius: "0.125rem",
		},
		Parts: Parts{
			Hero: elegant.Hero, Couple: elegant.Couple, LoveStory: elegant.LoveStory, Events: elegant.Events,
			Gallery: elegant.Gallery, Closing: elegant.Closing, SectionTitle: elegant.SectionTitle,
		},
	})
	register(ThemeDef{
		ID: "minimal", Name: "Minimalis", Description: "Lega dan tenang: rata kiri, garis tipis, angka tanggal besar.",
		Tokens: view.Tokens{
			Primary: "#5f5661", Surface: "#ffffff", Ink: "#1f1f1f", Accent: "#b9b2b0", Deep: "#1f1f1f", Muted: "#666666", Border: "#e5e5e5",
			FontHeading: "Josefin Sans", FontBody: "Inter", Radius: "0", ButtonRadius: "0",
		},
		Parts: Parts{
			Hero: minimal.Hero, Couple: minimal.Couple, LoveStory: minimal.LoveStory, Events: minimal.Events,
			Gallery: minimal.Gallery, Closing: minimal.Closing, SectionTitle: minimal.SectionTitle,
		},
	})
	register(ThemeDef{
		ID: "romantic", Name: "Romantis", Description: "Lembut dan personal: foto melengkung, ornamen bunga, tulisan tangan.",
		Tokens: view.Tokens{
			Primary: "#a85a67", Surface: "#fff6f6", Ink: "#4a3b3b", Accent: "#c9a88a", Deep: "#5a3540", Muted: "#735e5e", Border: "#f0dcdc",
			FontHeading: "Great Vibes", FontBody: "Lora", Radius: "1.5rem",
		},
		Parts: Parts{
			Hero: romantic.Hero, Couple: romantic.Couple, LoveStory: romantic.LoveStory, Events: romantic.Events,
			Gallery: romantic.Gallery, Closing: romantic.Closing, SectionTitle: romantic.SectionTitle,
		},
	})
	register(ThemeDef{
		ID: "modern", Name: "Modern", Description: "Tegas dan kontras: pembuka gelap, blok warna penuh, huruf sangat besar.",
		Tokens: view.Tokens{
			Primary: "#332936", Surface: "#f6f3f1", Ink: "#191519", Accent: "#c9a88a", Deep: "#191519", Muted: "#5c565c", Border: "#e2dcd8",
			FontHeading: "Montserrat", FontBody: "Poppins", Radius: "1.25rem", ButtonRadius: "0.75rem",
		},
		Parts: Parts{
			Hero: modern.Hero, Couple: modern.Couple, LoveStory: modern.LoveStory, Events: modern.Events,
			Gallery: modern.Gallery, Closing: modern.Closing, SectionTitle: modern.SectionTitle,
		},
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
