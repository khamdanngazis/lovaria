package imageproc

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestSpansWeightsSumToOne(t *testing.T) {
	for _, c := range [][2]int{{4000, 2048}, {3000, 1536}, {1365, 480}, {7, 3}, {10, 10}} {
		for i, sp := range spans(c[0], c[1]) {
			var sum float64
			for _, w := range sp.weights {
				sum += float64(w)
			}
			if math.Abs(sum-1) > 1e-4 {
				t.Fatalf("%v: span %d jumlah bobot = %f", c, i, sum)
			}
		}
	}
}

func TestResizeAreaUniformAndAntiAlias(t *testing.T) {
	// Warna seragam tetap sama.
	u := image.NewRGBA(image.Rect(0, 0, 301, 199))
	for i := range u.Pix {
		u.Pix[i] = []byte{200, 100, 50, 255}[i%4]
	}
	out := resizeArea(u, 97, 64)
	if c := out.RGBAAt(40, 30); c != (color.RGBA{200, 100, 50, 255}) {
		t.Errorf("seragam: %v", c)
	}

	// Papan catur 1px hitam-putih diperkecil 2× → abu-abu (bukan moiré/aliasing).
	cb := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			v := uint8(0)
			if (x+y)%2 == 0 {
				v = 255
			}
			cb.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	out = resizeArea(cb, 100, 100)
	for _, p := range []image.Point{{0, 0}, {50, 50}, {99, 99}} {
		if c := out.RGBAAt(p.X, p.Y); c.R < 120 || c.R > 135 {
			t.Errorf("papan catur %v: %v, want ~128", p, c)
		}
	}
}

// TestResizeRealPhoto: jalankan dengan PHOTO=/path/foto-12mp.jpg untuk mengukur.
func TestResizeRealPhoto(t *testing.T) {
	path := os.Getenv("PHOTO")
	if path == "" {
		t.Skip("set PHOTO untuk mengukur")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	start := time.Now()
	if _, err := Process(bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&m1)
	t.Logf("Process: %s, total alokasi %d MB", time.Since(start).Round(time.Millisecond), (m1.TotalAlloc-m0.TotalAlloc)>>20)
}
