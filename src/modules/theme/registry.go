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
	"github.com/khamdanngazis/lovaria/src/templates/themes/regional"
	"github.com/khamdanngazis/lovaria/src/templates/themes/romantic"
	"github.com/khamdanngazis/lovaria/src/templates/themes/signature"
	"github.com/khamdanngazis/lovaria/static"
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
	// Pitch: uraian lebih panjang untuk halaman etalase tema (/tema/<id>, T27).
	Pitch string
	// Region: daerah yang menginspirasi tema (Koleksi Daerah, T28); kosong =
	// koleksi utama.
	Region string
	Tokens view.Tokens
	Parts  Parts
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
		ID: "signature", Name: "Lunovia Signature", Description: "Editorial, hangat, dan premium — wajah Lunovia.",
		Pitch: "Lunovia Signature adalah tema andalan kami: sampul foto potret layar penuh, judul serif besar bergaya majalah, dan garis champagne tipis di atas warna plum yang hangat. Cocok untuk pasangan yang ingin undangan terasa premium dan modern tanpa terlihat ramai — foto prewedding menjadi pusat perhatian, sementara detail acara tersaji rapi dan mudah dibaca.",
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
		Pitch: "Tema Elegan membawa nuansa undangan cetak klasik ke layar ponsel: bingkai ganda dengan ornamen sudut, huruf serif Cormorant Garamond, dan tata letak serba rata tengah. Pilihan yang pas untuk akad dan resepsi formal, pernikahan adat, atau pasangan yang menyukai kesan anggun dan tak lekang waktu dengan sentuhan warna emas.",
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
		Pitch: "Tema Minimalis mengutamakan ruang kosong dan keterbacaan: teks rata kiri, garis tipis, angka tanggal besar, dan galeri grid tanpa sudut membulat. Cocok untuk pasangan yang menyukai desain bersih ala Skandinavia atau Jepang, dan ingin informasi acara langsung terbaca tanpa ornamen berlebih.",
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
		Pitch: "Tema Romantis terasa lembut dan personal: foto berbentuk lengkung, ornamen ranting bunga, judul tulisan tangan, dan warna merah muda yang hangat. Cocok untuk pernikahan taman, intimate wedding, atau pasangan yang ingin undangan terasa manis dan penuh cerita.",
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
		ID: "modern", Name: "Modern", Description: "Tegas dan segar: blok warna penuh, huruf sangat besar, kartu acara berwarna.",
		Pitch: "Tema Modern tampil tegas dan segar: nama pasangan dengan huruf kapital sangat besar, blok warna penuh, dan kartu acara berwarna hijau toska. Cocok untuk pasangan muda yang menyukai tampilan berani dan kontemporer, dengan foto sampul sebagai latar pembuka.",
		Tokens: view.Tokens{
			Primary: "#2f7d6d", Surface: "#f4f6f5", Ink: "#16201e", Accent: "#c9a88a", Deep: "#1d4a43", Muted: "#55605d", Border: "#dde3e1",
			FontHeading: "Montserrat", FontBody: "Poppins", Radius: "1.25rem", ButtonRadius: "0.75rem",
		},
		Parts: Parts{
			Hero: modern.Hero, Couple: modern.Couple, LoveStory: modern.LoveStory, Events: modern.Events,
			Gallery: modern.Gallery, Closing: modern.Closing, SectionTitle: modern.SectionTitle,
		},
	})
	// Koleksi Daerah (T28): budaya daerah dengan rasa modern. Motif hanya
	// sebagai pita/bingkai/detail; tiap tema punya 1–2 penanda budaya.
	regionalTheme := func(id, name, region, desc, pitch string, tk view.Tokens, hero Part, k regional.Kit) {
		register(ThemeDef{
			ID: id, Name: name, Region: region, Description: desc, Pitch: pitch, Tokens: tk,
			Parts: Parts{Hero: hero, Couple: k.Couple, LoveStory: k.LoveStory, Events: k.Events, Gallery: k.Gallery, Closing: k.Closing, SectionTitle: k.SectionTitle},
		})
	}
	regionalTheme("jawa", "Javanese Heritage", "Jawa",
		"Keanggunan Jawa yang tenang: pita batik kawung, warna sogan, dan foto medali lonjong.",
		"Javanese Heritage membawa nuansa keraton ke undangan digital dengan cara yang tenang dan modern. Motif batik kawung hanya hadir sebagai pita tipis di tepi sampul dan kartu, dipadukan warna sogan, gading, dan emas champagne serta huruf serif yang klasik. Cocok untuk akad, panggih, dan resepsi adat Jawa — atau pasangan yang ingin membawa sentuhan Jawa tanpa terlihat ramai.",
		view.Tokens{Primary: "#7a4a26", Surface: "#faf5ec", Ink: "#2e2118", Accent: "#c9a063", Deep: "#3a2718", Muted: "#6b5a4a", Border: "#e6d9c4", FontHeading: "Cormorant Garamond", FontBody: "Lora", Radius: "0.25rem", ButtonRadius: "0.25rem"},
		regional.JawaHero, regional.Jawa)
	regionalTheme("sunda", "Sundanese Romance", "Sunda",
		"Lembut dan alami: hijau sage, pucuk daun, dan perbukitan Priangan.",
		"Sundanese Romance terasa ringan dan lapang, seperti pagi di tanah Priangan. Warna hijau sage dan gading, ornamen pucuk daun, siluet perbukitan di sampul, dan nama pasangan bertulisan tangan memberi kesan romantis yang alami. Cocok untuk pernikahan adat Sunda, pesta kebun, atau pasangan yang ingin undangan terasa hangat dan bersahaja.",
		view.Tokens{Primary: "#5d7256", Surface: "#f8f6ef", Ink: "#2f352c", Accent: "#b9a77c", Deep: "#36452f", Muted: "#5f6659", Border: "#e2e4d6", FontHeading: "Dancing Script", FontBody: "Nunito", Radius: "1.5rem"},
		regional.SundaHero, regional.Sunda)
	regionalTheme("minang", "Minang Heritage", "Minangkabau",
		"Megah dan berkarakter: gonjong Rumah Gadang, pita songket, marun dan emas.",
		"Minang Heritage adalah ungkapan modern dari kemegahan Minangkabau. Siluet gonjong Rumah Gadang menjadi penanda utama di sampul dan kartu acara, dengan pita songket emas di atas warna marun tua. Huruf kapital yang tegas memberi kesan megah tanpa berlebihan. Cocok untuk baralek, akad, dan resepsi adat Minang.",
		view.Tokens{Primary: "#7b1e2b", Surface: "#fbf6ee", Ink: "#2a1a1a", Accent: "#c9a24b", Deep: "#4a0f1b", Muted: "#6b5555", Border: "#ead9c8", FontHeading: "Cinzel", FontBody: "Lato", Radius: "0.125rem", ButtonRadius: "0.125rem"},
		regional.MinangHero, regional.Minang)
	regionalTheme("bali", "Balinese Elegance", "Bali",
		"Artistik dan hangat: siluet gapura berundak, bunga kamboja, terakota dan emas redup.",
		"Balinese Elegance menghadirkan keanggunan Bali yang tak lekang waktu dengan sentuhan modern. Foto pasangan dibingkai siluet gapura berundak, ditemani bunga kamboja dan pita sulur patra dalam warna terakota, cokelat tanah, dan emas redup. Fokusnya tetap pada keanggunan pernikahan — bukan suasana resor tropis. Cocok untuk pawiwahan maupun resepsi di Bali.",
		view.Tokens{Primary: "#9a4a2f", Surface: "#faf4ea", Ink: "#2f2319", Accent: "#b8975a", Deep: "#3d2a1c", Muted: "#6a5a4b", Border: "#e8dac6", FontHeading: "Playfair Display", FontBody: "Lora", Radius: "0.375rem", ButtonRadius: "0.375rem"},
		regional.BaliHero, regional.Bali)
	regionalTheme("bugis", "Bugis Royal", "Bugis",
		"Megah dan tegas: pita lipa sabbe, motif sulapa eppa, merah anggur dan emas.",
		"Bugis Royal merayakan warisan Bugis dengan rasa mewah yang terkendali. Pita kotak-kotak lipa sabbe membingkai sampul, belah ketupat sulapa eppa menjadi ornamen pembatas, dan foto pasangan tampil dalam bingkai segi enam berwarna emas di atas merah anggur tua. Cocok untuk mappacci, akad, dan resepsi adat Bugis-Makassar.",
		view.Tokens{Primary: "#6e1630", Surface: "#fbf5ee", Ink: "#24161a", Accent: "#d1a94a", Deep: "#3a0d1c", Muted: "#675257", Border: "#ead8cf", FontHeading: "Playfair Display", FontBody: "Montserrat", Radius: "0.125rem", ButtonRadius: "0.125rem"},
		regional.BugisHero, regional.Bugis)
}

// Featured: tema unggulan etalase ("Pilihan Lunovia") — tema bawaan.
func (d ThemeDef) Featured() bool { return d.ID == DefaultID }

// Thumb: URL thumbnail tangkapan layar tema (static/img/themes/<id>.webp,
// dibuat `make theme-thumbs`); "" bila berkasnya tidak ada → kartu warna.
func (d ThemeDef) Thumb() string {
	name := "img/themes/" + d.ID + ".webp"
	if !static.Exists(name) {
		return ""
	}
	return static.URL(name)
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
