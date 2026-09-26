// Package imageproc memvalidasi & memproses foto upload: deteksi format dari
// magic bytes, batas ukuran & piksel, koreksi orientasi EXIF, resize, thumbnail,
// dan encode ulang ke JPEG (metadata EXIF/GPS ikut terbuang).
package imageproc

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/webp"
)

const (
	MaxBytes   = 10 << 20 // 10 MB per file
	MaxPixels  = 40_000_000
	MainSize   = 2048 // sisi terpanjang foto utama
	ThumbSize  = 480  // sisi terpanjang thumbnail
	mainJPEGQ  = 82
	thumbJPEGQ = 78
)

var (
	ErrTooLarge    = errors.New("ukuran file maksimal 10 MB")
	ErrNotImage    = errors.New("file bukan gambar JPG, PNG, atau WebP")
	ErrHEIC        = errors.New("format HEIC/HEIF belum didukung — ubah ke JPG dulu (di iPhone: Pengaturan › Kamera › Format › Paling Kompatibel)")
	ErrTooManyPx   = errors.New("resolusi gambar terlalu besar (maksimal 40 megapiksel)")
	ErrCorrupt     = errors.New("gambar rusak atau tidak bisa dibaca")
	errUnsupported = errors.New("format tidak didukung")
)

// Result adalah hasil pemrosesan: foto utama & thumbnail dalam JPEG.
type Result struct {
	Main, Thumb             []byte
	Width, Height           int
	ThumbWidth, ThumbHeight int
}

// ContentType hasil selalu JPEG.
const ContentType = "image/jpeg"

// sem membatasi decode/resize bersamaan di seluruh proses (hemat RAM).
var sem = make(chan struct{}, 2)

// Detect mengenali format dari magic bytes: "jpeg", "png", "webp".
func Detect(head []byte) (string, error) {
	switch {
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return "jpeg", nil
	case len(head) >= 8 && bytes.Equal(head[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "png", nil
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return "webp", nil
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		switch string(head[8:12]) {
		case "heic", "heix", "hevc", "heim", "heis", "hevm", "hevs", "mif1", "msf1":
			return "", ErrHEIC
		}
	}
	return "", ErrNotImage
}

// Process membaca maksimal MaxBytes dari r lalu menghasilkan foto utama & thumbnail.
func Process(r io.Reader) (Result, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxBytes+1))
	if err != nil {
		return Result{}, err
	}
	if len(data) > MaxBytes {
		return Result{}, ErrTooLarge
	}
	format, err := Detect(data)
	if err != nil {
		return Result{}, err
	}

	cfg, err := decodeConfig(format, data)
	if err != nil {
		return Result{}, ErrCorrupt
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return Result{}, ErrCorrupt
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return Result{}, ErrTooManyPx
	}

	sem <- struct{}{}
	defer func() { <-sem }()

	src, err := decode(format, data)
	if err != nil {
		return Result{}, ErrCorrupt
	}
	orientation := 1
	if format == "jpeg" {
		orientation = exifOrientation(data)
	}

	main := orient(fit(src, MainSize), orientation)
	thumb := fit(main, ThumbSize)

	var res Result
	if res.Main, err = encodeJPEG(main, mainJPEGQ); err != nil {
		return Result{}, err
	}
	if res.Thumb, err = encodeJPEG(thumb, thumbJPEGQ); err != nil {
		return Result{}, err
	}
	res.Width, res.Height = main.Bounds().Dx(), main.Bounds().Dy()
	res.ThumbWidth, res.ThumbHeight = thumb.Bounds().Dx(), thumb.Bounds().Dy()
	return res, nil
}

func decodeConfig(format string, data []byte) (image.Config, error) {
	rd := bytes.NewReader(data)
	switch format {
	case "jpeg":
		return jpeg.DecodeConfig(rd)
	case "png":
		return png.DecodeConfig(rd)
	case "webp":
		return webp.DecodeConfig(rd)
	}
	return image.Config{}, errUnsupported
}

func decode(format string, data []byte) (image.Image, error) {
	rd := bytes.NewReader(data)
	switch format {
	case "jpeg":
		return jpeg.Decode(rd)
	case "png":
		return png.Decode(rd)
	case "webp":
		return webp.Decode(rd)
	}
	return nil, errUnsupported
}

// fit memperkecil gambar supaya sisi terpanjang ≤ max (tidak pernah memperbesar).
// Hasil selalu *image.RGBA dengan latar putih (transparansi PNG/WebP diratakan).
func fit(src image.Image, maxSide int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxSide || h > maxSide {
		// Dibulatkan ke terdekat supaya rasio tetap (1365×480/2048 → 320, bukan 319).
		if w >= h {
			h = max(1, (h*maxSide+w/2)/w)
			w = maxSide
		} else {
			w = max(1, (w*maxSide+h/2)/h)
			h = maxSide
		}
	}
	return resizeArea(src, w, h)
}

func encodeJPEG(img image.Image, q int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return nil, fmt.Errorf("imageproc: encode: %w", err)
	}
	return buf.Bytes(), nil
}
