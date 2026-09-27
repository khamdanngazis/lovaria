package theme

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

// ---------- Pustaka musik bawaan (T20) ----------

// Track: satu lagu bebas royalti di pustaka bawaan. Berkasnya disimpan di bucket
// foto dengan key "music/<File>" (diunggah operator, lihat doc/themes.md).
type Track struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Duration string `json:"duration"` // "3:12"
	File     string `json:"file"`
	License  string `json:"license"` // wajib: lisensi yang mengizinkan pemakaian komersial
	Source   string `json:"source"`  // URL halaman asal lagu
}

//go:embed music_library.json
var musicLibraryJSON []byte

// MusicLibrary: pustaka lagu bawaan (manifest music_library.json).
var MusicLibrary = mustTracks(musicLibraryJSON)

func mustTracks(b []byte) []Track {
	var ts []Track
	if err := json.Unmarshal(b, &ts); err != nil {
		panic("theme: music_library.json: " + err.Error())
	}
	for _, t := range ts {
		if t.ID == "" || t.File == "" || t.License == "" || strings.ContainsAny(t.File, "/\\") {
			panic(fmt.Sprintf("theme: lagu %q di music_library.json tidak lengkap (id, file, license wajib)", t.ID))
		}
	}
	return ts
}

// MusicKey: key storage lagu bawaan.
func MusicKey(t Track) string { return "music/" + t.File }

// ---------- Contoh kutipan (T20) ----------

// QuoteSample: kutipan siap pakai yang bisa dipilih lalu diedit pasangan. Tidak
// ada yang dipakai otomatis — bagian kutipan kosong sampai pasangan memilih.
type QuoteSample struct {
	Label  string
	Text   string
	Source string
}

var QuoteSamples = []QuoteSample{
	{
		Label:  "Islam — QS. Ar-Rum: 21",
		Text:   "Dan di antara tanda-tanda kekuasaan-Nya ialah Dia menciptakan untukmu pasangan hidup dari jenismu sendiri, supaya kamu cenderung dan merasa tenteram kepadanya, dan dijadikan-Nya di antaramu rasa kasih dan sayang.",
		Source: "QS. Ar-Rum: 21",
	},
	{
		Label:  "Kristen — Matius 19:6",
		Text:   "Demikianlah mereka bukan lagi dua, melainkan satu. Karena itu, apa yang telah dipersatukan Allah, tidak boleh diceraikan manusia.",
		Source: "Matius 19:6",
	},
	{
		Label:  "Kristen — 1 Korintus 13:4",
		Text:   "Kasih itu sabar; kasih itu murah hati; ia tidak cemburu. Ia tidak memegahkan diri dan tidak sombong.",
		Source: "1 Korintus 13:4",
	},
	{
		Label:  "Umum — Antoine de Saint-Exupéry",
		Text:   "Cinta bukan tentang saling memandang, melainkan memandang bersama ke arah yang sama.",
		Source: "Antoine de Saint-Exupéry",
	},
	{
		Label: "Umum — tanpa sumber",
		Text:  "Dua hati, satu tujuan. Dengan memohon rahmat dan restu, kami mengundang Anda untuk menjadi bagian dari awal perjalanan kami.",
	},
}

// ---------- Validasi ----------

// Batas panjang teks personalisasi (karakter).
const (
	MaxQuote       = 500
	MaxQuoteSource = 100
	MaxGreeting    = 100
	MaxClosing     = 500
)

// validatePersonalization memeriksa bagian T20 dari pengaturan (teks & bagian).
// URL musik diperiksa di Service (butuh daftar lagu & unggahan milik wedding).
func validatePersonalization(s *view.Settings, e SettingsError) {
	for _, f := range []struct {
		key  string
		val  *string
		max  int
		name string
	}{
		{"quote_text", &s.QuoteText, MaxQuote, "Kutipan"},
		{"quote_source", &s.QuoteSource, MaxQuoteSource, "Sumber kutipan"},
		{"greeting", &s.Greeting, MaxGreeting, "Teks sapaan"},
		{"closing", &s.Closing, MaxClosing, "Kalimat penutup"},
	} {
		*f.val = strings.TrimSpace(strings.ReplaceAll(*f.val, "\r\n", "\n"))
		if utf8.RuneCountInString(*f.val) > f.max {
			e[f.key] = fmt.Sprintf("%s maksimal %d karakter", f.name, f.max)
		}
	}
	if s.QuoteText == "" {
		s.QuoteSource = ""
	}
	s.MusicURL = strings.TrimSpace(s.MusicURL)

	var hidden []string
	for _, id := range s.HiddenSections {
		d, ok := SectionByID(id)
		switch {
		case !ok:
			e["hidden_sections"] = "Bagian tidak dikenal"
		case !d.Hideable:
			e["hidden_sections"] = d.Label + " tidak bisa disembunyikan"
		default:
			hidden = append(hidden, id)
		}
	}
	s.HiddenSections = dedupe(hidden)
	order := make([]string, 0, len(s.SectionOrder))
	for _, id := range s.SectionOrder {
		if _, ok := SectionByID(id); !ok {
			e["section_order"] = "Bagian tidak dikenal"
			continue
		}
		order = append(order, id)
	}
	s.SectionOrder = dedupe(order)
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	var out []string // nil bila kosong (pengaturan bawaan)
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
