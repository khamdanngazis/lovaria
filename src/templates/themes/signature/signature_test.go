package signature

import (
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

func TestNameSize(t *testing.T) {
	large, medium, small := nameSize("Arya", "Kirana"), nameSize("Muhammad", "Sari"), nameSize("Wijayakusuma", "Sari")
	if large == medium || medium == small {
		t.Errorf("tingkat ukuran harus berbeda: %q, %q, %q", large, medium, small)
	}
	// Dihitung per huruf, bukan byte: tujuh huruf non-ASCII tetap tingkat terbesar.
	if got := nameSize("Élodièé", "山田花子"); got != large {
		t.Errorf("nama non-ASCII: %q", got)
	}
	// Nama terpanjang yang menentukan, dari sisi mana pun.
	if got := nameSize("Sari", "Muhammad"); got != medium {
		t.Errorf("mempelai wanita lebih panjang: %q", got)
	}
}

func TestStoryDate(t *testing.T) {
	for in, want := range map[string][2]string{
		"2019":          {"2019", ""},
		"Maret 2019":    {"2019", "Maret"},
		"12 Maret 2019": {"2019", "12 Maret"},
		"":              {"", ""},
	} {
		if y, r := storyDate(in); y != want[0] || r != want[1] {
			t.Errorf("storyDate(%q) = %q, %q", in, y, r)
		}
	}
}

func TestGallerySrcset(t *testing.T) {
	p := view.Photo{URL: "a.jpg", ThumbURL: "a_thumb.jpg", Width: 1600, Height: 1000}
	if got := gallerySrcset(p); got != "a_thumb.jpg 480w, a.jpg 1600w" {
		t.Errorf("srcset = %q", got)
	}
	for _, p := range []view.Photo{{URL: "a.jpg", ThumbURL: "a_thumb.jpg"}, {URL: "a.jpg", ThumbURL: "a_thumb.jpg", Width: 400}, {URL: "a.jpg", ThumbURL: "a.jpg", Width: 1600}} {
		if got := gallerySrcset(p); got != "" {
			t.Errorf("%+v: srcset = %q", p, got)
		}
	}
	if big, small := gallerySizes(galleryCell{Index: 0}), gallerySizes(galleryCell{Index: 4, Class: "aspect-[3/4]"}); big == small {
		t.Errorf("sizes harus berbeda: %q", big)
	}
}

func TestGalleryBlocks(t *testing.T) {
	land := func(id string) view.Photo { return view.Photo{URL: id, Width: 1200, Height: 800} }
	port := func(id string) view.Photo { return view.Photo{URL: id, Width: 800, Height: 1200} }
	if galleryBlocks(nil) != nil {
		t.Error("tanpa foto: tanpa blok")
	}
	// Semua jumlah foto: setiap foto tampil tepat sekali, foto pertama foto utama.
	for n := 1; n <= 14; n++ {
		for name, mk := range map[string]func(string) view.Photo{"lanskap": land, "potret": port, "tanpa ukuran": func(id string) view.Photo { return view.Photo{URL: id} }} {
			photos := make([]view.Photo, n)
			for i := range photos {
				photos[i] = mk(string(rune('a' + i)))
			}
			blocks := galleryBlocks(photos)
			seen := map[string]bool{}
			for _, b := range blocks {
				for _, c := range b.Cells {
					if seen[c.Photo.URL] {
						t.Errorf("%s/%d: foto %s ganda", name, n, c.Photo.URL)
					}
					seen[c.Photo.URL] = true
					if photos[c.Index].URL != c.Photo.URL {
						t.Errorf("%s/%d: indeks %d salah", name, n, c.Index)
					}
				}
			}
			if len(seen) != n {
				t.Errorf("%s/%d: tampil %d foto", name, n, len(seen))
			}
			if blocks[0].Cells[0].Photo.URL != "a" {
				t.Errorf("%s/%d: foto utama bukan foto pertama", name, n)
			}
		}
	}
	// Enam foto: blok utama + kolase, dan foto lanskap menempati slot lebar.
	blocks := galleryBlocks([]view.Photo{land("a"), land("b"), land("c"), port("d"), land("e"), port("f")})
	if len(blocks) != 2 || len(blocks[0].Cells) != 3 || len(blocks[1].Cells) != 3 {
		t.Fatalf("blok = %+v", blocks)
	}
	for _, c := range blocks[1].Cells {
		if wide := strings.Contains(c.Class, "col-span-2"); wide != (c.Photo.URL == "e") {
			t.Errorf("foto %s: slot %q", c.Photo.URL, c.Class)
		}
	}
	// Foto utama potret mendapat blok tegak, bukan dipotong jadi lanskap.
	if b := galleryBlocks([]view.Photo{port("a"), land("b"), land("c")})[0]; !strings.Contains(b.Cells[0].Class, "aspect-[4/5]") {
		t.Errorf("foto utama potret: %q", b.Cells[0].Class)
	}
	// Kolase berikutnya tidak mengulang bentuk yang sama.
	ps := []view.Photo{land("a"), land("b"), land("c"), land("d"), land("e"), land("f"), land("g"), land("h"), land("i")}
	if b := galleryBlocks(ps); b[1].Class == b[2].Class {
		t.Errorf("kolase berulang: %q", b[1].Class)
	}
}
