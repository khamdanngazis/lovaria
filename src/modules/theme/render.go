package theme

import (
	"context"
	"io"

	"github.com/a-h/templ"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/templates/shared"
)

// Prepare memasang bagian milik tema ke view: ID tema final (cadangan DefaultID)
// dan judul bagian tema. Dipanggil Render; public site memanggilnya juga untuk
// fragmen bagian bersama (RSVP / ucapan lewat htmx) supaya judulnya tetap
// bergaya tema setelah di-swap.
func Prepare(v view.View) view.View {
	def, ok := Get(v.ThemeID)
	if !ok {
		def, _ = Get(DefaultID)
	}
	v.ThemeID = def.ID
	v.SectionTitle = def.Parts.SectionTitle
	return v
}

// Render merender halaman undangan lengkap untuk v memakai tema v.ThemeID
// (fallback DefaultID) + pengaturan v.Settings. Satu-satunya pintu masuk (T09).
func Render(v view.View) templ.Component {
	v = Prepare(v)
	def, _ := Get(v.ThemeID)
	v.Tokens = resolve(def.Tokens, v.Settings)
	v.CSS = TokensCSS(def.ID, v.Tokens)
	v.FontsURL = GoogleFontsURL(v.Tokens.FontHeading, v.Tokens.FontBody)

	v.Quote = view.Quote{Text: v.Settings.QuoteText, Source: v.Settings.QuoteSource}
	v.Greeting, v.Closing = v.Settings.Greeting, v.Settings.Closing
	v.Music = view.Music{URL: v.Settings.MusicURL, Enabled: v.Settings.MusicEnabled}

	p := def.Parts
	// Urutan bawaan sesuai Produk §7 (Opening → Couple → … → Gift → Closing);
	// bagian tengah bisa diurutkan & disembunyikan pasangan (T20, registry
	// Sections). Setelah hari H, kenangan (foto hari-H + ucapan favorit) tampil
	// tepat setelah pembuka (T19).
	parts := map[string]templ.Component{
		"couple": p.Couple(v), "countdown": shared.CountdownSection(v), "quote": shared.QuoteSection(v),
		"events": p.Events(v), "story": p.LoveStory(v), "gallery": p.Gallery(v),
		"rsvp": shared.RSVPSection(v), "checkin": shared.CheckinSection(v), "guestbook": shared.GuestbookSection(v), "gift": shared.GiftSection(v),
	}
	sections := []templ.Component{p.Hero(v), shared.MemorySection(v), shared.ContentAnchor()}
	for _, d := range OrderedSections(v.Settings) {
		if !SectionHidden(v.Settings, d.ID) {
			sections = append(sections, parts[d.ID])
		}
	}
	sections = append(sections, p.Closing(v), shared.MusicButton(v))
	body := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		for _, s := range sections {
			if err := s.Render(ctx, w); err != nil {
				return err
			}
		}
		return nil
	})
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return p.Layout(v).Render(templ.WithChildren(ctx, body), w)
	})
}
