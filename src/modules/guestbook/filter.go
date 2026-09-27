package guestbook

import (
	"strings"
	"unicode"
)

// DefaultBlockedWords: daftar awal kata kasar (Indonesia, Jawa, Inggris).
// Ditambah lewat config GUESTBOOK_BLOCKED_WORDS. Pesan yang cocok hanya
// disembunyikan (bukan ditolak), jadi salah deteksi bisa dipulihkan pasangan.
var DefaultBlockedWords = []string{
	"anjing", "anjir", "asu", "babi", "bajingan", "bangsat", "bego", "brengsek", "goblok", "goblog",
	"jancok", "jancuk", "jembut", "kampret", "keparat", "kontol", "lonte", "memek", "ngentot",
	"pelacur", "pepek", "perek", "tai", "taik", "tolol", "bitch", "fuck", "fucking", "shit", "asshole",
}

// WordFilter mencocokkan kata utuh (bukan potongan kata) setelah normalisasi:
// huruf kecil, angka "leet" (4→a, 1→i, 0→o, 3→e, 5→s, 7→t), dan huruf yang
// diulang ("anjiiing" → "anjing").
type WordFilter struct {
	words map[string]bool
}

func NewWordFilter(words []string) *WordFilter {
	f := &WordFilter{words: map[string]bool{}}
	for _, w := range words {
		if w = normalizeWord(strings.TrimSpace(w)); w != "" {
			f.words[w] = true
		}
	}
	return f
}

var leet = strings.NewReplacer("4", "a", "@", "a", "1", "i", "!", "i", "0", "o", "3", "e", "5", "s", "$", "s", "7", "t")

func normalizeWord(w string) string {
	w = leet.Replace(strings.ToLower(w))
	var b strings.Builder
	var prev rune
	for _, r := range w {
		if !unicode.IsLetter(r) || r == prev {
			continue
		}
		b.WriteRune(r)
		prev = r
	}
	return b.String()
}

// Match: teks mengandung salah satu kata terlarang.
func (f *WordFilter) Match(text string) bool {
	if f == nil || len(f.words) == 0 {
		return false
	}
	// Pisah per spasi/tanda baca, tapi pertahankan karakter leet di dalam kata.
	for _, tok := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("@!$", r)
	}) {
		// "!" / "$" di ujung kata adalah tanda baca, bukan huruf leet ("bangsat!!").
		if f.words[normalizeWord(strings.Trim(tok, "!$"))] {
			return true
		}
	}
	return false
}
