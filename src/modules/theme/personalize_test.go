package theme

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

func TestValidatePersonalization(t *testing.T) {
	long := func(n int) string { return strings.Repeat("a", n) }
	for _, c := range []struct {
		name  string
		in    view.Settings
		field string // "" = valid
	}{
		{"kosong", view.Settings{}, ""},
		{"kutipan pas 500", view.Settings{QuoteText: long(500)}, ""},
		{"kutipan 501", view.Settings{QuoteText: long(501)}, "quote_text"},
		{"sumber 101", view.Settings{QuoteText: "x", QuoteSource: long(101)}, "quote_source"},
		{"sapaan 101", view.Settings{Greeting: long(101)}, "greeting"},
		{"penutup 501", view.Settings{Closing: long(501)}, "closing"},
		{"karakter multibyte dihitung per huruf", view.Settings{QuoteText: strings.Repeat("é", 500)}, ""},
		{"sembunyikan bagian dikenal", view.Settings{HiddenSections: []string{"story", "gift"}}, ""},
		{"bagian tak dikenal", view.Settings{HiddenSections: []string{"script"}}, "hidden_sections"},
		{"RSVP tidak bisa disembunyikan", view.Settings{HiddenSections: []string{"rsvp"}}, "hidden_sections"},
		{"acara tidak bisa disembunyikan", view.Settings{HiddenSections: []string{"events"}}, "hidden_sections"},
		{"urutan tak dikenal", view.Settings{SectionOrder: []string{"gift", "<b>"}}, "section_order"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := ValidateSettings(c.in)
			var se SettingsError
			switch {
			case c.field == "" && err != nil:
				t.Errorf("harus valid: %v", err)
			case c.field != "" && (!errors.As(err, &se) || se[c.field] == ""):
				t.Errorf("harus error di %s: %v", c.field, err)
			}
		})
	}

	got, err := ValidateSettings(view.Settings{
		QuoteText: "  Kasih itu sabar\r\n ", QuoteSource: " 1 Kor 13:4 ", Greeting: " Yth. ",
		HiddenSections: []string{"gift", "gift"}, SectionOrder: []string{"quote", "couple", "quote"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.QuoteText != "Kasih itu sabar" || got.QuoteSource != "1 Kor 13:4" || got.Greeting != "Yth." ||
		!reflect.DeepEqual(got.HiddenSections, []string{"gift"}) || !reflect.DeepEqual(got.SectionOrder, []string{"quote", "couple"}) {
		t.Errorf("normalisasi: %+v", got)
	}
	// Sumber tanpa kutipan tidak disimpan.
	if got, _ := ValidateSettings(view.Settings{QuoteSource: "QS"}); got.QuoteSource != "" {
		t.Error("sumber tanpa kutipan harus dikosongkan")
	}
}

func ids(ds []SectionDef) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.ID
	}
	return out
}

func TestOrderedAndMoveSections(t *testing.T) {
	if got := ids(OrderedSections(view.Settings{})); !reflect.DeepEqual(got, SectionIDs) {
		t.Errorf("bawaan = %v", got)
	}
	// Urutan sebagian: sisanya disisipkan di posisi bawaannya, tanpa duplikat.
	got := ids(OrderedSections(view.Settings{SectionOrder: []string{"gift", "couple", "gift", "nope"}}))
	want := []string{"gift", "couple", "countdown", "quote", "events", "story", "gallery", "rsvp", "checkin", "guestbook"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sebagian = %v", got)
	}
	st := view.Settings{}
	if got := MoveSection(st, "quote", true); got[1] != "quote" || got[2] != "countdown" {
		t.Errorf("naik = %v", got)
	}
	if got := MoveSection(st, "couple", true); !reflect.DeepEqual(got, SectionIDs) {
		t.Errorf("bagian teratas tidak bisa naik: %v", got)
	}
	if got := MoveSection(st, "gift", false); !reflect.DeepEqual(got, SectionIDs) {
		t.Errorf("bagian terbawah tidak bisa turun: %v", got)
	}
	if !SectionHidden(view.Settings{HiddenSections: []string{"story"}}, "story") || SectionHidden(view.Settings{HiddenSections: []string{"rsvp"}}, "rsvp") {
		t.Error("SectionHidden hanya untuk bagian yang bisa disembunyikan")
	}
}

func TestRenderPersonalization(t *testing.T) {
	for _, d := range All() {
		t.Run(d.ID, func(t *testing.T) {
			v := sample()
			v.ThemeID = d.ID
			v.AllowGuestbook = true
			v.Countdown = view.Countdown{Show: true, DaysLeft: 12, Target: time.Date(2026, 12, 12, 1, 0, 0, 0, time.UTC), CalendarURL: "/w/x/events/1.ics"}
			v.Settings = view.Settings{
				QuoteText: `Kasih itu sabar <script>alert("q")</script>`, QuoteSource: "1 Korintus 13:4",
				Greeting: `Yth. <img src=x onerror=alert(1)>`, Closing: "Sampai jumpa di hari bahagia kami",
				MusicURL: "https://media.test/music/a.mp3", MusicEnabled: true,
				SectionOrder:   []string{"gallery", "quote", "couple"},
				HiddenSections: []string{"story", "guestbook"},
			}
			html := render(t, v)
			for _, bad := range []string{`<script>alert("q")`, `<img src=x`, `id="story"`, `id="guestbook"`} {
				if strings.Contains(html, bad) {
					t.Errorf("tidak boleh ada %q", bad)
				}
			}
			for _, want := range []string{"Kasih itu sabar &lt;script&gt;", "1 Korintus 13:4", "Yth. &lt;img", "Sampai jumpa di hari bahagia kami",
				`id="lv-music"`, `src="https://media.test/music/a.mp3"`, `preload="none"`, "12 hari lagi", `data-countdown="2026-12-12T01:00:00Z"`, `href="/w/x/events/1.ics"`, `href="#undangan"`} {
				if !strings.Contains(html, want) {
					t.Errorf("tidak ada %q", want)
				}
			}
			// Urutan: galeri → kutipan → (acara: disisipkan setelah kutipan, posisi
			// bawaannya) → mempelai → hitung mundur.
			pos := func(id string) int { return strings.Index(html, `id="`+id+`"`) }
			if !slices.IsSorted([]int{pos("undangan"), pos("gallery"), pos("quote"), pos("events"), pos("couple"), pos("countdown")}) {
				t.Errorf("urutan salah: undangan=%d gallery=%d quote=%d couple=%d countdown=%d events=%d", pos("undangan"), pos("gallery"), pos("quote"), pos("couple"), pos("countdown"), pos("events"))
			}
		})
	}

	// Bawaan: tanpa kutipan, musik, atau hitung mundur.
	html := render(t, sample())
	for _, bad := range []string{`id="quote"`, `id="lv-music"`, `id="countdown"`} {
		if strings.Contains(html, bad) {
			t.Errorf("bawaan tidak boleh ada %q", bad)
		}
	}
	// Musik nonaktif / tanpa URL → tidak dirender; hari H → "Hari ini!"; kenangan → tanpa hitung mundur.
	v := sample()
	v.Settings = view.Settings{MusicURL: "https://media.test/music/a.mp3"}
	v.Countdown = view.Countdown{Show: true, Today: true}
	if html := render(t, v); strings.Contains(html, `id="lv-music"`) || !strings.Contains(html, "Hari ini!") || strings.Contains(html, "data-countdown") {
		t.Error("musik nonaktif / hari H salah")
	}
	v.Memory = true
	if strings.Contains(render(t, v), `id="countdown"`) {
		t.Error("hitung mundur tidak tampil saat kenangan")
	}
}
