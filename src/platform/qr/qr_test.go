package qr

import (
	"bytes"
	"image/png"
	"testing"
)

func TestPNG(t *testing.T) {
	b, err := PNG("https://lunovia.id/i/ABCDEFG", 512)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("bukan PNG: %v", err)
	}
	if r := img.Bounds(); r.Dx() != 512 || r.Dy() != 512 {
		t.Errorf("ukuran %v", r)
	}
	// Sudut kiri-atas adalah tepi kosong (putih); isi berbeda → gambar berbeda.
	if r, g, bl, _ := img.At(0, 0).RGBA(); r != 0xffff || g != 0xffff || bl != 0xffff {
		t.Error("tepi kosong harus putih")
	}
	other, _ := PNG("https://lunovia.id/i/HJKMNPQ", 512)
	if bytes.Equal(b, other) {
		t.Error("isi berbeda harus menghasilkan QR berbeda")
	}
	for _, bad := range []struct {
		s    string
		size int
	}{{"", 512}, {"x", 10}, {"x", 5000}} {
		if _, err := PNG(bad.s, bad.size); err == nil {
			t.Errorf("PNG(%q, %d) harus ditolak", bad.s, bad.size)
		}
	}
}
