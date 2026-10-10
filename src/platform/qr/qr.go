// Package qr membuat gambar kode QR (PNG) di server — tanpa layanan pihak
// ketiga dan tanpa script di halaman (T31: QR kehadiran tamu).
package qr

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// PNG mengembalikan gambar QR hitam di atas putih berukuran size×size piksel,
// lengkap dengan tepi kosong (quiet zone) supaya terbaca di latar apa pun.
// Tingkat koreksi Medium: tetap terbaca dari layar ponsel yang redup atau
// sedikit tergores.
func PNG(content string, size int) ([]byte, error) {
	if content == "" {
		return nil, fmt.Errorf("qr: isi kosong")
	}
	if size < 128 || size > 2048 {
		return nil, fmt.Errorf("qr: ukuran %d di luar 128–2048", size)
	}
	b, err := qrcode.Encode(content, qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("qr: %w", err)
	}
	return b, nil
}
