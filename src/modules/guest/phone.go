package guest

import (
	"errors"
	"strings"
)

// ErrInvalidPhone dikembalikan NormalizePhone untuk nomor yang tidak valid.
var ErrInvalidPhone = errors.New("nomor HP tidak valid")

// phoneMessage adalah pesan untuk user saat nomor HP tidak valid.
const phoneMessage = "Nomor HP tidak valid (contoh: 0812-3456-7890)"

// NormalizePhone mengubah nomor HP ke format internasional tanpa "+":
// "0812-3456-7890", "+62 812 3456 7890", "812345678" → "628123456789".
// Nomor luar negeri ("+1 555 …") disimpan sebagai digit saja. Kosong → "".
func NormalizePhone(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	intl := strings.HasPrefix(s, "+")
	var d strings.Builder
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			d.WriteRune(r)
		case r == ' ', r == '-', r == '.', r == '(', r == ')', r == '+' && d.Len() == 0:
		default:
			return "", ErrInvalidPhone
		}
	}
	n := d.String()
	switch {
	case intl:
		// sudah berkode negara
	case strings.HasPrefix(n, "62"):
	case strings.HasPrefix(n, "0"):
		n = "62" + n[1:]
	case strings.HasPrefix(n, "8"):
		n = "62" + n
	default:
		return "", ErrInvalidPhone
	}
	if len(n) < 10 || len(n) > 15 {
		return "", ErrInvalidPhone
	}
	if strings.HasPrefix(n, "62") && !strings.HasPrefix(n, "628") {
		return "", ErrInvalidPhone // nomor HP Indonesia selalu 08…
	}
	return n, nil
}
