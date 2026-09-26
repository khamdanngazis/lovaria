package guest

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	seen := map[string]bool{}
	var prev string
	for i := 0; i < 5000; i++ {
		c, err := NewCode()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidCode(c) {
			t.Fatalf("kode tidak valid: %q", c)
		}
		if strings.ContainsAny(c, "0O1IL") {
			t.Fatalf("kode memuat karakter ambigu: %q", c)
		}
		if seen[c] {
			t.Fatalf("kode duplikat dalam 5000 sampel: %q", c)
		}
		seen[c] = true
		// Tidak berurutan: kode berikutnya tidak berbagi prefiks panjang dengan sebelumnya.
		if prev != "" && c[:5] == prev[:5] {
			t.Fatalf("kode terlihat berurutan: %q setelah %q", c, prev)
		}
		prev = c
	}
	for _, c := range []string{"", "ABC", "ABCDEF0", "abcdefg", "ABCDEFGH", "ABCDEFL"} {
		if ValidCode(c) {
			t.Errorf("%q harus invalid", c)
		}
	}
	if NormalizeCode(" abcdefg ") != "ABCDEFG" {
		t.Error("NormalizeCode")
	}
}

func TestNormalizePhone(t *testing.T) {
	ok := map[string]string{
		"0812-3456-7890":    "6281234567890",
		"+62 812 3456 7890": "6281234567890",
		"62812 3456 7890":   "6281234567890",
		"812.3456.7890":     "6281234567890",
		"(0812) 3456 789":   "628123456789",
		"+1 (555) 123-4567": "15551234567",
		"":                  "",
		"  ":                "",
	}
	for in, want := range ok {
		got, err := NormalizePhone(in)
		if err != nil || got != want {
			t.Errorf("NormalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"12345", "0812abc", "021-555-1234x", "6221555123", "08123456789012345", "+62"} {
		if _, err := NormalizePhone(in); err == nil {
			t.Errorf("%q harus invalid", in)
		}
	}
}
