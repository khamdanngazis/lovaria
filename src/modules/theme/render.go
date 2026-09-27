package theme

import (
	"context"
	"io"

	"github.com/a-h/templ"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/templates/shared"
)

// Render merender halaman undangan lengkap untuk v memakai tema v.ThemeID
// (fallback DefaultID) + pengaturan v.Settings. Satu-satunya pintu masuk (T09).
func Render(v view.View) templ.Component {
	def, ok := Get(v.ThemeID)
	if !ok {
		def, _ = Get(DefaultID)
	}
	v.ThemeID = def.ID
	v.Tokens = resolve(def.Tokens, v.Settings)
	v.CSS = TokensCSS(def.ID, v.Tokens)
	v.FontsURL = GoogleFontsURL(v.Tokens.FontHeading, v.Tokens.FontBody)

	p := def.Parts
	// Urutan bagian sesuai Produk §7: Opening → Couple (+Date) → Love Story →
	// Events → Gallery → RSVP → Guestbook → Gift → Closing.
	sections := []templ.Component{
		p.Hero(v), p.Couple(v), p.LoveStory(v), p.Events(v), p.Gallery(v),
		shared.RSVPSection(v), shared.GuestbookSection(v), shared.GiftSection(v),
		p.Closing(v),
	}
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
