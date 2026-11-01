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

// Preset tema lain (T37): yang pertama sama persis dengan warna bawaan sebelum
// T37, supaya undangan yang sudah ada tidak berubah. Secondary, Ink, Muted,
// Border, Paper, dan Soft preset baru diturunkan dari Primary, Accent, Surface,
// dan Deep lalu diuji kontrasnya (TestThemeContrast, TestPresetTokens).
var elegantPresets = []Preset{
	{
		ID: "champagne-gold", Name: "Champagne Gold", Character: "Klasik · Hangat · Anggun",
		Primary: "#8a6a3c", Secondary: "#aa8963", Accent: "#c9a88a", Surface: "#fbf8f3",
		Deep: "#2e2620", Ink: "#2b2b2b", Muted: "#6a6258", Border: "#e6dccb", Paper: "#fdfcfa", Soft: "#ede2d6",
	},
	{
		ID: "navy-gold", Name: "Navy Gold", Character: "Formal · Berwibawa · Tegas",
		Primary: "#2c3e5c", Secondary: "#777064", Accent: "#c2a36b", Surface: "#f8f6f1",
		Deep: "#1b2638", Ink: "#313b4a", Muted: "#696f7a", Border: "#dbdcdc", Paper: "#fcfbf9", Soft: "#e9dfcb",
	},
	{
		ID: "emerald-classic", Name: "Emerald Classic", Character: "Mewah · Tenang · Berkelas",
		Primary: "#2f5d4e", Secondary: "#7a8261", Accent: "#c4a774", Surface: "#f7f7f2",
		Deep: "#1d3a31", Ink: "#334d44", Muted: "#61746d", Border: "#dbe1db", Paper: "#fcfcfa", Soft: "#e9e1cf",
	},
	{
		ID: "burgundy-royal", Name: "Burgundy Royal", Character: "Megah · Hangat · Dramatis",
		Primary: "#7a2e3a", Secondary: "#a2685a", Accent: "#c9a27a", Surface: "#fbf6f3",
		Deep: "#45161f", Ink: "#572c34", Muted: "#86676c", Border: "#e9dad9", Paper: "#fdfbfa", Soft: "#edded1",
	},
	{
		ID: "silver-monochrome", Name: "Silver Monochrome", Character: "Modern · Bersih · Elegan",
		Primary: "#4a4a4f", Secondary: "#818186", Accent: "#b8b8bd", Surface: "#fafafa",
		Deep: "#222226", Ink: "#38383b", Muted: "#707073", Border: "#e1e1e2", Paper: "#fdfdfd", Soft: "#e8e8e9",
	},
}

var minimalPresets = []Preset{
	{
		ID: "stone-mauve", Name: "Stone Mauve", Character: "Tenang · Lembut · Bersih",
		Primary: "#5f5661", Secondary: "#8c8488", Accent: "#b9b2b0", Surface: "#ffffff",
		Deep: "#1f1f1f", Ink: "#1f1f1f", Muted: "#666666", Border: "#e5e5e5", Paper: "#ffffff", Soft: "#ebe9e9",
	},
	{
		ID: "charcoal", Name: "Charcoal", Character: "Tegas · Sederhana · Kontras",
		Primary: "#333333", Secondary: "#787878", Accent: "#bdbdbd", Surface: "#ffffff",
		Deep: "#111111", Ink: "#292929", Muted: "#757575", Border: "#e2e2e2", Paper: "#ffffff", Soft: "#ededed",
	},
	{
		ID: "sage-linen", Name: "Sage Linen", Character: "Natural · Segar · Teduh",
		Primary: "#56695a", Secondary: "#869686", Accent: "#b7c2b2", Surface: "#fbfbf8",
		Deep: "#1f2a22", Ink: "#353f37", Muted: "#6c746e", Border: "#e4e7e2", Paper: "#fdfdfc", Soft: "#e8ebe4",
	},
	{
		ID: "sand-clay", Name: "Sand Clay", Character: "Hangat · Membumi · Santai",
		Primary: "#8a5a44", Secondary: "#af8c79", Accent: "#d4bfae", Surface: "#fdfbf8",
		Deep: "#2b1f19", Ink: "#40352f", Muted: "#78706c", Border: "#ede4df", Paper: "#fefdfc", Soft: "#f2eae3",
	},
	{
		ID: "ocean-mist", Name: "Ocean Mist", Character: "Sejuk · Jernih · Tenang",
		Primary: "#3f5f73", Secondary: "#7992a1", Accent: "#b3c4cf", Surface: "#fbfcfd",
		Deep: "#1a2a33", Ink: "#303f47", Muted: "#69747a", Border: "#e1e6ea", Paper: "#fdfefe", Soft: "#e7ecf0",
	},
}

var romanticPresets = []Preset{
	{
		ID: "rose-blush", Name: "Rose Blush", Character: "Manis · Hangat · Romantis",
		Primary: "#a85a67", Secondary: "#b88178", Accent: "#c9a88a", Surface: "#fff6f6",
		Deep: "#5a3540", Ink: "#4a3b3b", Muted: "#735e5e", Border: "#f0dcdc", Paper: "#fffbfb", Soft: "#f0e0d8",
	},
	{
		ID: "peach-bloom", Name: "Peach Bloom", Character: "Ceria · Hangat · Segar",
		Primary: "#a85a42", Secondary: "#c6876e", Accent: "#e4b49b", Surface: "#fff7f2",
		Deep: "#5b3326", Ink: "#6b473a", Muted: "#89695f", Border: "#f3e1d9", Paper: "#fffcfa", Soft: "#f7e4da",
	},
	{
		ID: "lilac-dream", Name: "Lilac Dream", Character: "Lembut · Anggun · Melamun",
		Primary: "#7d5a8c", Secondary: "#a285ae", Accent: "#c7b0d1", Surface: "#fbf7fc",
		Deep: "#46304f", Ink: "#584460", Muted: "#7b6b82", Border: "#e9e1ec", Paper: "#fdfcfe", Soft: "#ece3f0",
	},
	{
		ID: "sage-romance", Name: "Sage Romance", Character: "Natural · Teduh · Taman",
		Primary: "#5e7a63", Secondary: "#90a48e", Accent: "#c2cfb8", Surface: "#f8faf5",
		Deep: "#33453a", Ink: "#47574d", Muted: "#66746b", Border: "#e2e8e1", Paper: "#fcfdfb", Soft: "#e9eee4",
	},
	{
		ID: "dusty-blue", Name: "Dusty Blue", Character: "Tenang · Halus · Klasik",
		Primary: "#4e6a8a", Secondary: "#8499b1", Accent: "#b9c8d8", Surface: "#f6f9fc",
		Deep: "#2b3d52", Ink: "#3f5063", Muted: "#637181", Border: "#dee5ec", Paper: "#fbfdfe", Soft: "#e5ebf2",
	},
}

var modernPresets = []Preset{
	{
		ID: "teal", Name: "Teal", Character: "Segar · Tegas · Muda",
		Primary: "#2f7d6d", Secondary: "#7c927c", Accent: "#c9a88a", Surface: "#f4f6f5",
		Deep: "#1d4a43", Ink: "#16201e", Muted: "#55605d", Border: "#dde3e1", Paper: "#fbfbfb", Soft: "#e8e0d7",
	},
	{
		ID: "midnight-navy", Name: "Midnight Navy", Character: "Tegas · Elegan · Malam",
		Primary: "#2b4c7e", Secondary: "#86806d", Accent: "#e0b45c", Surface: "#f3f5f8",
		Deep: "#18294a", Ink: "#2e3d5b", Muted: "#626e84", Border: "#d7dde7", Paper: "#fafbfc", Soft: "#eee3cc",
	},
	{
		ID: "terracotta-pop", Name: "Terracotta Pop", Character: "Hangat · Berani · Energik",
		Primary: "#b04a2f", Secondary: "#d08655", Accent: "#f0c27b", Surface: "#f8f4f1",
		Deep: "#5c2618", Ink: "#6c3b2e", Muted: "#8c655a", Border: "#eedcd6", Paper: "#fcfbf9", Soft: "#f6e6d0",
	},
	{
		ID: "plum", Name: "Plum", Character: "Dramatis · Hangat · Unik",
		Primary: "#6b3a6e", Secondary: "#a67093", Accent: "#e2a6b8", Surface: "#f6f3f6",
		Deep: "#3a1d3c", Ink: "#4d324f", Muted: "#7b677c", Border: "#e3d9e3", Paper: "#fbfafb", Soft: "#f0dde5",
	},
	{
		ID: "graphite", Name: "Graphite", Character: "Urban · Bersih · Netral",
		Primary: "#3a3f44", Secondary: "#81888d", Accent: "#c8d0d6", Surface: "#f4f5f6",
		Deep: "#1c1f22", Ink: "#323437", Muted: "#6c6e70", Border: "#dadcdd", Paper: "#fbfbfb", Soft: "#e8ebed",
	},
}

var jawaPresets = []Preset{
	{
		ID: "sogan-gading", Name: "Sogan Gading", Character: "Klasik · Tenang · Keraton",
		Primary: "#7a4a26", Secondary: "#a27544", Accent: "#c9a063", Surface: "#faf5ec",
		Deep: "#3a2718", Ink: "#2e2118", Muted: "#6b5a4a", Border: "#e6d9c4", Paper: "#fdfbf7", Soft: "#ecddc6",
	},
	{
		ID: "indigo-lurik", Name: "Indigo Lurik", Character: "Teduh · Bersahaja · Dalam",
		Primary: "#2e4566", Secondary: "#7c7264", Accent: "#c9a063", Surface: "#f7f5ef",
		Deep: "#1a2840", Ink: "#303c52", Muted: "#666f7d", Border: "#dbdcdc", Paper: "#fcfbf9", Soft: "#eaddc8",
	},
	{
		ID: "hijau-keraton", Name: "Hijau Keraton", Character: "Anggun · Tenang · Berwibawa",
		Primary: "#3f5a3a", Secondary: "#847d4e", Accent: "#c9a063", Surface: "#f8f6ee",
		Deep: "#23331f", Ink: "#384634", Muted: "#677161", Border: "#dee0d5", Paper: "#fcfbf8", Soft: "#ebdec7",
	},
	{
		ID: "merah-manggis", Name: "Merah Manggis", Character: "Hangat · Megah · Matang",
		Primary: "#7b2f2f", Secondary: "#a66a47", Accent: "#d0a45f", Surface: "#faf4ee",
		Deep: "#431818", Ink: "#552e2d", Muted: "#856765", Border: "#e8d8d3", Paper: "#fdfbf8", Soft: "#eedec6",
	},
	{
		ID: "ireng-prada", Name: "Ireng Prada", Character: "Formal · Mewah · Kontras",
		Primary: "#3a3330", Secondary: "#826a4a", Accent: "#c9a063", Surface: "#f6f2ea",
		Deep: "#1e1a18", Ink: "#34302d", Muted: "#706c67", Border: "#dcd7d0", Paper: "#fbfaf7", Soft: "#e9dbc4",
	},
}

var sundaPresets = []Preset{
	{
		ID: "sage-priangan", Name: "Sage Priangan", Character: "Alami · Ringan · Romantis",
		Primary: "#5d7256", Secondary: "#8b8c69", Accent: "#b9a77c", Surface: "#f8f6ef",
		Deep: "#36452f", Ink: "#2f352c", Muted: "#5f6659", Border: "#e2e4d6", Paper: "#fcfbf9", Soft: "#e6e0cf",
	},
	{
		ID: "biru-situ", Name: "Biru Situ", Character: "Sejuk · Tenang · Jernih",
		Primary: "#4a6a7a", Secondary: "#8098a2", Accent: "#b7c6c9", Surface: "#f6f8f7",
		Deep: "#2a3e48", Ink: "#3e515a", Muted: "#64737a", Border: "#dee4e6", Paper: "#fbfcfc", Soft: "#e4eaea",
	},
	{
		ID: "kembang-kopi", Name: "Kembang Kopi", Character: "Hangat · Bersahaja · Membumi",
		Primary: "#7a5a44", Secondary: "#a08467", Accent: "#c7ad8a", Surface: "#faf6f0",
		Deep: "#43311f", Ink: "#554534", Muted: "#7a6d5e", Border: "#e8e0d8", Paper: "#fdfbf9", Soft: "#ece2d3",
	},
	{
		ID: "rose-mojang", Name: "Rose Mojang", Character: "Manis · Lembut · Muda",
		Primary: "#9a5f66", Secondary: "#bc8c8d", Accent: "#ddb9b4", Surface: "#fdf7f5",
		Deep: "#573338", Ink: "#68474b", Muted: "#85696c", Border: "#efe2e1", Paper: "#fefcfb", Soft: "#f4e6e3",
	},
	{
		ID: "lumut-tua", Name: "Lumut Tua", Character: "Teduh · Dalam · Alami",
		Primary: "#3f5e4f", Secondary: "#748c6f", Accent: "#a9b98f", Surface: "#f5f6ef",
		Deep: "#243a30", Ink: "#394d43", Muted: "#63736a", Border: "#dce1d9", Paper: "#fbfbf9", Soft: "#e0e5d4",
	},
}

var minangPresets = []Preset{
	{
		ID: "marun-emas", Name: "Marun Emas", Character: "Megah · Tegas · Berkarakter",
		Primary: "#7b1e2b", Secondary: "#a2603b", Accent: "#c9a24b", Surface: "#fbf6ee",
		Deep: "#4a0f1b", Ink: "#2a1a1a", Muted: "#6b5555", Border: "#ead9c8", Paper: "#fdfbf8", Soft: "#eddec0",
	},
	{
		ID: "hitam-songket", Name: "Hitam Songket", Character: "Formal · Mewah · Kontras",
		Primary: "#2b2523", Secondary: "#7a6437", Accent: "#c9a24b", Surface: "#f8f4ec",
		Deep: "#161210", Ink: "#2d2926", Muted: "#716d68", Border: "#dbd7d0", Paper: "#fcfbf7", Soft: "#ebddbf",
	},
	{
		ID: "merah-marawa", Name: "Merah Marawa", Character: "Berani · Meriah · Hangat",
		Primary: "#a3262a", Secondary: "#be6b3b", Accent: "#d8b04c", Surface: "#fcf6ee",
		Deep: "#5a1214", Ink: "#6a292a", Muted: "#936361", Border: "#f0d9d3", Paper: "#fefbf8", Soft: "#f2e2c1",
	},
	{
		ID: "hijau-bukit", Name: "Hijau Bukit", Character: "Teduh · Anggun · Tenang",
		Primary: "#2f5a43", Secondary: "#7c7e47", Accent: "#c9a24b", Surface: "#f7f6ee",
		Deep: "#1a3527", Ink: "#30483b", Muted: "#617367", Border: "#dbe0d6", Paper: "#fcfbf8", Soft: "#eadec0",
	},
	{
		ID: "biru-samudra", Name: "Biru Samudra", Character: "Sejuk · Berwibawa · Dalam",
		Primary: "#24466b", Secondary: "#76745b", Accent: "#c9a24b", Surface: "#f6f6f2",
		Deep: "#142a43", Ink: "#2b3e54", Muted: "#62717f", Border: "#d9dddf", Paper: "#fbfbfa", Soft: "#e9dec3",
	},
}

var baliPresets = []Preset{
	{
		ID: "terakota-kamboja", Name: "Terakota Kamboja", Character: "Artistik · Hangat · Anggun",
		Primary: "#9a4a2f", Secondary: "#a97044", Accent: "#b8975a", Surface: "#faf4ea",
		Deep: "#3d2a1c", Ink: "#2f2319", Muted: "#6a5a4b", Border: "#e8dac6", Paper: "#fdfbf7", Soft: "#e8dac2",
	},
	{
		ID: "hitam-batu-candi", Name: "Hitam Batu Candi", Character: "Formal · Megah · Kontras",
		Primary: "#2f2a27", Secondary: "#746040", Accent: "#b8975a", Surface: "#f6f2eb",
		Deep: "#181513", Ink: "#2e2b29", Muted: "#6f6c69", Border: "#dad6d0", Paper: "#fbfaf7", Soft: "#e5d9c2",
	},
	{
		ID: "hijau-subak", Name: "Hijau Subak", Character: "Alami · Teduh · Segar",
		Primary: "#4a6a3f", Secondary: "#868752", Accent: "#c2a465", Surface: "#f7f6ee",
		Deep: "#283c22", Ink: "#3d4f36", Muted: "#65735e", Border: "#dfe2d6", Paper: "#fcfbf8", Soft: "#e8dfc8",
	},
	{
		ID: "merah-bata", Name: "Merah Bata", Character: "Berani · Hangat · Meriah",
		Primary: "#a3372c", Secondary: "#bc7045", Accent: "#d4a95e", Surface: "#fbf4ee",
		Deep: "#5a1b15", Ink: "#6a312b", Muted: "#90645e", Border: "#efdad3", Paper: "#fdfbf8", Soft: "#f0dfc6",
	},
	{
		ID: "biru-samudra", Name: "Biru Samudra", Character: "Sejuk · Tenang · Lapang",
		Primary: "#2f5a6e", Secondary: "#7c8574", Accent: "#c9b07a", Surface: "#f5f7f6",
		Deep: "#1a3441", Ink: "#304853", Muted: "#60727b", Border: "#d9e1e3", Paper: "#fbfcfb", Soft: "#e9e3d3",
	},
}

var bugisPresets = []Preset{
	{
		ID: "merah-anggur", Name: "Merah Anggur", Character: "Megah · Tegas · Mewah",
		Primary: "#6e1630", Secondary: "#a0603d", Accent: "#d1a94a", Surface: "#fbf5ee",
		Deep: "#3a0d1c", Ink: "#24161a", Muted: "#675257", Border: "#ead8cf", Paper: "#fdfbf8", Soft: "#efe0c0",
	},
	{
		ID: "hijau-lipa-sabbe", Name: "Hijau Lipa Sabbe", Character: "Anggun · Tenang · Berkelas",
		Primary: "#1f5a48", Secondary: "#788249", Accent: "#d1a94a", Surface: "#f6f6f0",
		Deep: "#10352a", Ink: "#27483e", Muted: "#5c746b", Border: "#d8e0d8", Paper: "#fbfbf9", Soft: "#ece0c2",
	},
	{
		ID: "ungu-senja", Name: "Ungu Senja", Character: "Dramatis · Hangat · Unik",
		Primary: "#4f2a5e", Secondary: "#906a54", Accent: "#d1a94a", Surface: "#f8f4f8",
		Deep: "#2b1435", Ink: "#402a48", Muted: "#78697f", Border: "#e0d8e2", Paper: "#fcfbfc", Soft: "#eddfc7",
	},
	{
		ID: "biru-pinisi", Name: "Biru Pinisi", Character: "Berwibawa · Sejuk · Dalam",
		Primary: "#1f3f66", Secondary: "#787458", Accent: "#d1a94a", Surface: "#f5f6f8",
		Deep: "#11243e", Ink: "#283951", Muted: "#647081", Border: "#d7dce4", Paper: "#fbfbfc", Soft: "#ebe0c7",
	},
	{
		ID: "hitam-emas", Name: "Hitam Emas", Character: "Formal · Mewah · Kontras",
		Primary: "#2a2224", Secondary: "#7e6637", Accent: "#d1a94a", Surface: "#f7f3ee",
		Deep: "#151012", Ink: "#2c2728", Muted: "#726d6c", Border: "#dad6d2", Paper: "#fcfaf8", Soft: "#ecdec0",
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
