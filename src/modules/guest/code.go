package guest

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// codeAlphabet tanpa karakter ambigu (0 O 1 I L) supaya mudah dibaca/diketik.
const (
	codeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	codeLen      = 7
)

var alphabetSize = big.NewInt(int64(len(codeAlphabet)))

// NewCode menghasilkan kode undangan acak kriptografis (31^7 ≈ 2,75×10¹⁰ kemungkinan).
func NewCode() (string, error) {
	var b strings.Builder
	b.Grow(codeLen)
	for i := 0; i < codeLen; i++ {
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", err
		}
		b.WriteByte(codeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// NormalizeCode menyeragamkan input kode (spasi, huruf kecil) sebelum lookup.
func NormalizeCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// ValidCode memeriksa format kode tanpa menyentuh DB.
func ValidCode(s string) bool {
	if len(s) != codeLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(codeAlphabet, rune(s[i])) {
			return false
		}
	}
	return true
}
