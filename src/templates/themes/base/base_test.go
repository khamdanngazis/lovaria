package base

import "testing"

// Tiap baris grid (2 kolom) harus terisi penuh: tidak ada foto kotak yang sendirian.
func TestGalleryHasNoHoles(t *testing.T) {
	for n := 1; n <= 12; n++ {
		col := 0
		for i := 0; i < n; i++ {
			if wide(i, n) {
				if col != 0 {
					t.Fatalf("n=%d: foto lebar %d mulai di tengah baris", n, i)
				}
				continue
			}
			col = 1 - col
		}
		if col != 0 {
			t.Errorf("n=%d: baris terakhir berlubang", n)
		}
	}
}
