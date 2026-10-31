package signature

import (
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

func TestGalleryWide(t *testing.T) {
	land, port := view.Photo{Width: 1200, Height: 800}, view.Photo{Width: 800, Height: 1200}
	for name, photos := range map[string][]view.Photo{
		"potret tunggal di tengah": {land, port, land, land},
		"potret tunggal di akhir":  {land, land, port},
		"semua lanskap":            {land, land, land, land, land, land},
		"semua potret":             {port, port, port, port},
		"tanpa ukuran":             {{}, {}, {}, {}},
		"campuran":                 {land, port, port, land, port},
		"satu foto":                {land},
	} {
		t.Run(name, func(t *testing.T) {
			wide := galleryWide(photos)
			narrow := 0
			for _, w := range wide {
				if !w {
					narrow++
				}
			}
			if narrow%2 != 0 {
				t.Errorf("grid berlubang: %v", wide)
			}
			if len(photos) >= 3 && narrow == 0 {
				t.Errorf("grid seragam: %v", wide)
			}
		})
	}
	// Potret tidak dilebarkan bila orientasi foto campuran.
	if wide := galleryWide([]view.Photo{land, port, land}); wide[1] {
		t.Errorf("potret terpotong jadi lanskap: %v", wide)
	}
}
