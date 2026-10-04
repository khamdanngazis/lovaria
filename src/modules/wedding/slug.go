package wedding

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const maxSlugLen = 60

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedSlugs tidak boleh dipakai karena bentrok dengan route aplikasi.
var reservedSlugs = map[string]bool{
	"admin": true, "api": true, "i": true, "w": true, "dashboard": true, "login": true,
	"logout": true, "register": true, "static": true, "healthz": true, "readyz": true,
	"new": true, "www": true, "lovoria": true,
}

// ValidateSlug mengembalikan pesan error ("" bila valid). Slug hanya [a-z0-9-].
func ValidateSlug(s string) string {
	switch {
	case s == "":
		return "Slug wajib diisi"
	case len(s) > maxSlugLen:
		return "Slug maksimal 60 karakter"
	case !slugRe.MatchString(s):
		return "Slug hanya boleh huruf kecil a-z, angka, dan tanda hubung (-)"
	case reservedSlugs[s]:
		return "Slug ini tidak bisa dipakai"
	}
	return ""
}

// Slugify mengubah teks bebas menjadi slug: "Samuel & Sárah" → "samuel-sarah".
func Slugify(s string) string {
	// Buang diakritik: é → e.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	if out, _, err := transform.String(t, s); err == nil {
		s = out
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case b.Len() > 0 && !dash:
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > maxSlugLen {
		out = strings.TrimRight(out[:maxSlugLen], "-")
	}
	return out
}

// baseSlug membentuk slug dasar dari nama pasangan: "samuel-sarah".
// Fallback "wedding-<acak>" bila nama tidak menghasilkan karakter latin.
func baseSlug(groom, bride string) string {
	s := Slugify(firstWord(groom) + " " + firstWord(bride))
	if s == "" || reservedSlugs[s] {
		b := make([]byte, 3)
		_, _ = rand.Read(b)
		return "wedding-" + hex.EncodeToString(b)
	}
	return s
}

func firstWord(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return ""
}

// nextFreeSlug memilih base, atau base-2, base-3, ... yang belum dipakai.
func nextFreeSlug(base string, taken []string) string {
	used := make(map[string]bool, len(taken))
	for _, t := range taken {
		used[t] = true
	}
	if !used[base] {
		return base
	}
	for n := 2; ; n++ {
		cand := base + "-" + strconv.Itoa(n)
		if !used[cand] {
			return cand
		}
	}
}
