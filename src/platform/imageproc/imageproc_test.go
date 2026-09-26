package imageproc

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
	"time"
)

// halfImage: kiri merah, kanan biru.
func halfImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 0, 0, 255}
			if x >= w/2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func mustJPEG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withExif menyisipkan segmen APP1 Exif (Orientation + tag GPS palsu) setelah SOI.
func withExif(jpg []byte, orientation uint16) []byte {
	tiff := make([]byte, 0, 64)
	tiff = append(tiff, 'M', 'M', 0, 42, 0, 0, 0, 8) // big-endian, IFD0 di offset 8
	tiff = binary.BigEndian.AppendUint16(tiff, 2)    // 2 entri
	// Orientation (SHORT)
	tiff = binary.BigEndian.AppendUint16(tiff, 0x0112)
	tiff = binary.BigEndian.AppendUint16(tiff, 3)
	tiff = binary.BigEndian.AppendUint32(tiff, 1)
	tiff = binary.BigEndian.AppendUint16(tiff, orientation)
	tiff = append(tiff, 0, 0)
	// GPSInfo pointer (LONG) — cukup sebagai penanda "ada data lokasi".
	tiff = binary.BigEndian.AppendUint16(tiff, 0x8825)
	tiff = binary.BigEndian.AppendUint16(tiff, 4)
	tiff = binary.BigEndian.AppendUint32(tiff, 1)
	tiff = binary.BigEndian.AppendUint32(tiff, 0)
	tiff = binary.BigEndian.AppendUint32(tiff, 0) // next IFD
	payload := append([]byte("Exif\x00\x00"), tiff...)

	seg := []byte{0xFF, 0xE1}
	seg = binary.BigEndian.AppendUint16(seg, uint16(len(payload)+2))
	seg = append(seg, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func decodeJPEG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0xC000 && g < 0x4000 && b < 0x4000
}

func isBlue(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b > 0xC000 && r < 0x4000 && g < 0x4000
}

func TestProcessResizeAndThumb(t *testing.T) {
	res, err := Process(bytes.NewReader(mustJPEG(t, halfImage(3000, 2000))))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 2048 || res.Height != 1365 || res.ThumbWidth != 480 || res.ThumbHeight != 320 {
		t.Errorf("ukuran = %dx%d thumb %dx%d", res.Width, res.Height, res.ThumbWidth, res.ThumbHeight)
	}
	if img := decodeJPEG(t, res.Thumb); img.Bounds().Dx() != 480 {
		t.Errorf("thumb = %v", img.Bounds())
	}
}

func TestProcessSmallImageNotUpscaled(t *testing.T) {
	res, err := Process(bytes.NewReader(mustJPEG(t, halfImage(300, 200))))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 300 || res.Height != 200 || res.ThumbWidth != 300 {
		t.Errorf("tidak boleh diperbesar: %dx%d thumb %d", res.Width, res.Height, res.ThumbWidth)
	}
}

func TestProcessAppliesOrientationAndStripsExif(t *testing.T) {
	src := withExif(mustJPEG(t, halfImage(400, 200)), 6) // 90° searah jarum jam
	if o := exifOrientation(src); o != 6 {
		t.Fatalf("exifOrientation = %d", o)
	}
	res, err := Process(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 200 || res.Height != 400 {
		t.Fatalf("setelah rotasi = %dx%d, want 200x400", res.Width, res.Height)
	}
	img := decodeJPEG(t, res.Main)
	// Kiri (merah) berpindah ke atas, kanan (biru) ke bawah.
	if !isRed(img.At(100, 50)) || !isBlue(img.At(100, 350)) {
		t.Errorf("rotasi salah: atas=%v bawah=%v", img.At(100, 50), img.At(100, 350))
	}
	for _, out := range [][]byte{res.Main, res.Thumb} {
		if bytes.Contains(out, []byte("Exif")) || exifOrientation(out) != 1 {
			t.Error("EXIF (termasuk GPS) harus terbuang")
		}
	}
}

func TestOrientAll(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 2))
	src.Set(0, 0, color.RGBA{255, 0, 0, 255}) // penanda di pojok kiri atas
	want := map[int]image.Point{1: {0, 0}, 2: {2, 0}, 3: {2, 1}, 4: {0, 1}, 5: {0, 0}, 6: {1, 0}, 7: {1, 2}, 8: {0, 2}}
	for o, p := range want {
		out := orient(src, o)
		if !isRed(out.At(p.X, p.Y)) {
			t.Errorf("orientation %d: penanda tidak di %v", o, p)
		}
	}
}

func TestProcessPNGTransparencyAndWebP(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 10, 10)) // transparan penuh
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	res, err := Process(&b)
	if err != nil {
		t.Fatal(err)
	}
	if r, g, bl, _ := decodeJPEG(t, res.Main).At(5, 5).RGBA(); r < 0xF000 || g < 0xF000 || bl < 0xF000 {
		t.Error("transparansi harus menjadi putih")
	}

	// WebP lossless 1x1.
	webpData, _ := base64.StdEncoding.DecodeString("UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA==")
	if res, err := Process(bytes.NewReader(webpData)); err != nil || res.Width != 1 {
		t.Errorf("webp: %v %+v", err, res.Width)
	}
}

func TestProcessRejects(t *testing.T) {
	heic := append([]byte{0, 0, 0, 0x18}, []byte("ftypheic\x00\x00\x00\x00mif1heic")...)
	big := append(mustJPEG(t, halfImage(8, 8)), bytes.Repeat([]byte{0}, MaxBytes)...)

	// JPEG dengan dimensi di header dipalsukan menjadi 10000x10000 (> 40 MP).
	huge := mustJPEG(t, halfImage(16, 16))
	if i := bytes.Index(huge, []byte{0xFF, 0xC0}); i > 0 {
		binary.BigEndian.PutUint16(huge[i+5:], 10000)
		binary.BigEndian.PutUint16(huge[i+7:], 10000)
	}

	cases := map[string]struct {
		data []byte
		want error
	}{
		"teks di-rename .jpg": {[]byte("ini bukan gambar sama sekali"), ErrNotImage},
		"pdf":                 {[]byte("%PDF-1.7\n..."), ErrNotImage},
		"gif":                 {[]byte("GIF89a......"), ErrNotImage},
		"heic":                {heic, ErrHEIC},
		"lebih dari 10MB":     {big, ErrTooLarge},
		"lebih dari 40MP":     {huge, ErrTooManyPx},
		"jpeg terpotong":      {mustJPEG(t, halfImage(64, 64))[:200], ErrCorrupt},
		"kosong":              {nil, ErrNotImage},
	}
	for name, c := range cases {
		if _, err := Process(bytes.NewReader(c.data)); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
		}
	}
}

func TestProcessLargePhotoTime(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("lambat (dan tidak representatif di bawah -race)")
	}
	data := mustJPEG(t, halfImage(6000, 4000)) // 24 MP, seperti foto kamera ponsel
	start := time.Now()
	if _, err := Process(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	t.Logf("24MP diproses dalam %s", time.Since(start))
}
