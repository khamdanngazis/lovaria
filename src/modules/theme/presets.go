package theme

import "github.com/khamdanngazis/lovaria/src/modules/theme/view"

// Preset: kombinasi warna kurasi sebuah tema (T33). Struktur, tipografi, dan
// komposisi tema tidak berubah; hanya warna dan aset floral yang mengikuti.
type Preset struct {
	ID        string
	Name      string
	Character string
	Primary   string
	Secondary string // ornamen & dekorasi
	Accent    string
	Surface   string // latar halaman
	Deep      string // judul utama & penekanan
	Ink       string
	Muted     string
	Border    string
	Paper     string // latar kartu
	Soft      string // latar bagian lembut (hadiah)
}

// signaturePresets: preset Lunovia Signature; yang pertama adalah bawaan. Warna
// mengikuti T33 §4 kecuali Primary Blush & Terracotta serta Muted Lavender yang
// digelapkan sedikit agar kontras ≥ 4.5:1 (ditegakkan TestThemeContrast).
var signaturePresets = []Preset{
	{
		ID: "signature-lavender", Name: "Signature Lavender", Character: "Romantis · Elegan · Lembut",
		Primary: "#6b4e71", Secondary: "#a987a8", Accent: "#d9c7d9", Surface: "#fbf6f2",
		Deep: "#49334f", Ink: "#554653", Muted: "#7a6777", Border: "#e7dce4", Paper: "#fffdfc", Soft: "#ead8d8",
	},
	{
		ID: "sage-garden", Name: "Sage Garden", Character: "Natural · Segar · Tenang",
		Primary: "#536b58", Secondary: "#91a58b", Accent: "#dce3d5", Surface: "#faf8f0",
		Deep: "#37483b", Ink: "#46524a", Muted: "#667064", Border: "#dfe4d8", Paper: "#fffefa", Soft: "#e8e6d6",
	},
	{
		ID: "blush-romance", Name: "Blush Romance", Character: "Romantis · Halus · Hangat",
		Primary: "#92666e", Secondary: "#c99a9d", Accent: "#ebd6d3", Surface: "#fff8f4",
		Deep: "#64434a", Ink: "#5b4549", Muted: "#7e6a69", Border: "#efe0dc", Paper: "#fffdfb", Soft: "#f3e3df",
	},
	{
		ID: "midnight-elegance", Name: "Midnight Elegance", Character: "Klasik · Formal · Berkelas",
		Primary: "#293b55", Secondary: "#63758d", Accent: "#d8c7ac", Surface: "#f8f5ef",
		Deep: "#1c2a3e", Ink: "#39465a", Muted: "#616b7a", Border: "#e3ddd1", Paper: "#fffdf9", Soft: "#ede5d7",
	},
	{
		ID: "terracotta-bloom", Name: "Terracotta Bloom", Character: "Hangat · Membumi · Kontemporer",
		Primary: "#a45d46", Secondary: "#c58a6d", Accent: "#e8d2bd", Surface: "#faf5ed",
		Deep: "#6f3d2d", Ink: "#5d473e", Muted: "#7d6458", Border: "#ebddcd", Paper: "#fffcf7", Soft: "#f1e3d3",
	},
}

// apply mengisi token warna dari preset; font & radius tetap milik tema.
func (p Preset) apply(t view.Tokens) view.Tokens {
	t.Preset = p.ID
	t.Primary, t.Secondary, t.Accent = p.Primary, p.Secondary, p.Accent
	t.Surface, t.Background = p.Surface, p.Surface
	t.Deep, t.Ink, t.Muted, t.Border = p.Deep, p.Ink, p.Muted, p.Border
	t.Paper, t.Soft = p.Paper, p.Soft
	return t
}

// Preset: preset tema dengan ID id; cadangan preset pertama.
func (d ThemeDef) Preset(id string) (Preset, bool) {
	for _, p := range d.Presets {
		if p.ID == id {
			return p, true
		}
	}
	if len(d.Presets) > 0 {
		return d.Presets[0], false
	}
	return Preset{}, false
}
