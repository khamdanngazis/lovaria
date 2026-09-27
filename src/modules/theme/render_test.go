package theme

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

func sample() view.View {
	return view.View{
		Title: "Pernikahan", Date: time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC), DateText: "Sabtu, 12 Desember 2026",
		Couple:  view.Couple{GroomName: "Khamdan Ngazis", BrideName: "Sarah", GroomDesc: "Putra Bpk. A"},
		Guest:   &view.Guest{Name: `Budi <script>alert(1)</script>`},
		Events:  []view.Event{{Name: "Akad Nikah", TypeLabel: "Akad", DateText: "Sabtu", TimeText: "08.00 WIB", Venue: "Masjid", MapsURL: "https://maps.google.com/?q=1,2"}},
		Stories: []view.Story{{DateText: "2019", Title: "Pertama bertemu"}},
		Gallery: []view.Photo{{URL: "https://pub-x.r2.dev/a.jpg", ThumbURL: "https://pub-x.r2.dev/a_thumb.jpg"}},
	}
}

func render(t *testing.T, v view.View) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(v).Render(context.Background(), &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestRenderAllThemes(t *testing.T) {
	for _, d := range All() {
		t.Run(d.ID, func(t *testing.T) {
			v := sample()
			v.ThemeID = d.ID
			html := render(t, v)
			for _, want := range []string{
				`data-theme="` + d.ID + `"`,
				`:root[data-theme="` + d.ID + `"]{--lv-primary:` + d.Tokens.Primary,
				"Khamdan", "Sarah", "Sabtu, 12 Desember 2026", "Akad Nikah", "Pertama bertemu", "a_thumb.jpg",
				`id="opening"`, `id="couple"`, `id="events"`, `id="story"`, `id="gallery"`, `id="closing"`,
				"family=" + strings.ReplaceAll(d.Tokens.FontHeading, " ", "+"),
			} {
				if !strings.Contains(html, want) {
					t.Errorf("tidak memuat %q", want)
				}
			}
			if strings.Contains(html, "<script>alert(1)</script>") {
				t.Error("nama tamu tidak di-escape")
			}
			if strings.Contains(html, `id="rsvp"`) {
				t.Error("placeholder RSVP hanya boleh tampil saat preview")
			}
			// Hanya font judul & isi tema ini yang dimuat (dihitung di URL Google Fonts).
			i := strings.Index(html, "https://fonts.googleapis.com/css2?")
			if i < 0 {
				t.Fatal("URL Google Fonts tidak ada")
			}
			fontsURL := html[i : i+strings.Index(html[i:], `"`)]
			if n := strings.Count(fontsURL, "family="); n > 2 {
				t.Errorf("memuat %d font, maksimal 2: %s", n, fontsURL)
			}
			if strings.Contains(html, `<link rel="stylesheet" href="https://fonts.googleapis.com`) && !strings.Contains(html, "<noscript>") {
				t.Error("stylesheet font tidak boleh memblokir render")
			}
		})
	}
}

func TestRenderSettingsOverrideAndFallback(t *testing.T) {
	v := sample()
	v.ThemeID = "tidak-ada"
	v.Settings = view.Settings{PrimaryColor: "#123456", FontHeading: "Cinzel", CoverImage: "https://pub-x.r2.dev/cover.jpg"}
	v.Preview = true
	html := render(t, v)
	for _, want := range []string{`data-theme="elegant"`, "--lv-primary:#123456;", "family=Cinzel", "cover.jpg", `id="rsvp"`, "Preview"} {
		if !strings.Contains(html, want) {
			t.Errorf("tidak memuat %q", want)
		}
	}
	if strings.Contains(html, "family=Cormorant") {
		t.Error("font default tidak boleh dimuat saat diganti")
	}
}

func TestEmptySectionsHidden(t *testing.T) {
	v := sample()
	v.Events, v.Stories, v.Gallery, v.Guest = nil, nil, nil, nil
	html := render(t, v)
	for _, absent := range []string{`id="events"`, `id="story"`, `id="gallery"`} {
		if strings.Contains(html, absent) {
			t.Errorf("bagian kosong %s tidak boleh tampil", absent)
		}
	}
	if !strings.Contains(html, "Tamu Undangan") {
		t.Error("tanpa tamu → sapaan umum")
	}
}
