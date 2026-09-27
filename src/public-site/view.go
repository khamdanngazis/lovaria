package publicsite

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/gift"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	"github.com/khamdanngazis/lovaria/src/platform/web"
	"github.com/khamdanngazis/lovaria/static"
)

// ViewBuilder menyusun view.View (kontrak tema) dari service tiap modul.
// Dipakai halaman undangan publik (T09) dan preview tema di dashboard (T08).
type ViewBuilder struct {
	Weddings  *wedding.Service
	Events    *event.Service
	Stories   *story.Service
	Gallery   *gallery.Service
	Themes    *theme.Service
	Guestbook *guestbook.Service
	Gifts     *gift.Service
}

// Build menyusun data undangan wedding w; g boleh nil (akses tanpa kode tamu).
func (b *ViewBuilder) Build(ctx context.Context, w wedding.Wedding, g *guest.Guest) (view.View, error) {
	c, err := b.Weddings.GetCouple(ctx, w.ID)
	if err != nil {
		return view.View{}, fmt.Errorf("view: couple: %w", err)
	}
	evs, err := b.Events.ListEvents(ctx, w.ID)
	if err != nil {
		return view.View{}, fmt.Errorf("view: events: %w", err)
	}
	sts, err := b.Stories.ListStories(ctx, w.ID)
	if err != nil {
		return view.View{}, fmt.Errorf("view: stories: %w", err)
	}
	photos, err := b.Gallery.ListGallery(ctx, w.ID)
	if err != nil {
		return view.View{}, fmt.Errorf("view: gallery: %w", err)
	}
	settings, err := b.Themes.Settings(ctx, w.ID)
	if err != nil {
		return view.View{}, fmt.Errorf("view: theme: %w", err)
	}
	gifts, err := b.Gifts.List(ctx, w.ID)
	if err != nil {
		return view.View{}, fmt.Errorf("view: gifts: %w", err)
	}

	tz := wedding.TimezoneAbbr(w.Timezone)
	v := view.View{
		ThemeID: w.ThemeID, Slug: w.Slug, Title: w.Title, Description: w.Description,
		Date: w.WeddingDate, DateText: web.FormatDateID(w.WeddingDate), TZAbbr: tz,
		Couple: view.Couple{
			GroomName: c.GroomName, BrideName: c.BrideName,
			GroomPhoto: deref(c.GroomPhotoURL), BridePhoto: deref(c.BridePhotoURL),
			GroomDesc: c.GroomDescription, BrideDesc: c.BrideDescription,
		},
		Settings:       settings,
		Memory:         w.InMemory(),
		AllowRSVP:      w.AllowsRSVP(),
		AllowGuestbook: w.AllowsGuestbook(),
	}
	if w.MainPhotoURL != nil {
		v.MainPhoto = *w.MainPhotoURL
	}
	if v.AllowGuestbook {
		es, more, err := b.Guestbook.Visible(ctx, w.ID, uuid.Nil, guestbook.PublicPage)
		if err != nil {
			return view.View{}, fmt.Errorf("view: guestbook: %w", err)
		}
		v.Guestbook.Entries = guestbookEntries(es, w.Timezone)
		if more {
			v.Guestbook.MoreBefore = es[len(es)-1].ID.String()
		}
	}
	for _, a := range gifts {
		v.Gifts = append(v.Gifts, view.Gift{
			Type: a.Type, TypeLabel: gift.TypeLabel(a.Type), Provider: a.Provider,
			AccountNumber: a.AccountNumber, AccountName: a.AccountName, Address: a.Address,
		})
	}
	if g != nil {
		v.Guest = &view.Guest{Name: g.Name, Code: g.InvitationCode, MaxPax: g.MaxPax, RSVPStatus: g.RSVPStatus, RSVPPax: g.RSVPPax, RSVPMessage: g.RSVPMessage}
	}
	for _, e := range evs {
		v.Events = append(v.Events, view.Event{
			ID:   e.ID.String(),
			Name: e.Name, TypeLabel: event.TypeLabel(e.Type), DateText: web.FormatDateID(e.Date),
			TimeText: timeText(e.StartTime, e.EndTime, tz), Venue: e.Venue, Address: e.Address,
			MapsURL: e.MapsURL, Description: e.Description,
		})
	}
	for _, s := range sts {
		v.Stories = append(v.Stories, view.Story{DateText: s.Date.String(), Title: s.Title, Description: s.Description, PhotoURL: s.PhotoURL})
	}
	for _, p := range photos {
		if p.Category == gallery.CategoryCover {
			continue // foto sampul tampil di pembuka, bukan di galeri
		}
		v.Gallery = append(v.Gallery, view.Photo{URL: p.URL, ThumbURL: p.ThumbURL, Caption: p.Caption, Width: p.Width, Height: p.Height})
	}
	return v, nil
}

// Preview menyusun view untuk preview dashboard: bagian yang masih kosong diisi
// data contoh supaya tampilan tema terlihat utuh.
func (b *ViewBuilder) Preview(ctx context.Context, w wedding.Wedding) (view.View, error) {
	v, err := b.Build(ctx, w, nil)
	if err != nil {
		return view.View{}, err
	}
	v.Preview = true
	// Preview dashboard menampilkan semua bagian supaya tampilan tema terlihat utuh.
	v.AllowRSVP, v.AllowGuestbook = true, true
	FillSample(&v)
	return v, nil
}

// FillSample mengisi bagian kosong dengan data contoh (menandai v.Sample).
func FillSample(v *view.View) {
	img := func(name string) string { return static.URL("img/sample-" + name + ".svg") }
	if v.Guest == nil {
		v.Guest = &view.Guest{Name: "Nama Tamu", MaxPax: 2}
	}
	if v.MainPhoto == "" && v.Settings.CoverImage == "" {
		v.MainPhoto, v.Sample = img("cover"), true
	}
	if len(v.Events) == 0 {
		v.Sample = true
		v.Events = []view.Event{
			{Name: "Akad Nikah", TypeLabel: "Akad nikah", DateText: v.DateText, TimeText: "08.00–10.00 " + v.TZAbbr, Venue: "Masjid Agung", Address: "Jl. Contoh No. 1", MapsURL: "https://maps.google.com"},
			{Name: "Resepsi", TypeLabel: "Resepsi", DateText: v.DateText, TimeText: "11.00–14.00 " + v.TZAbbr, Venue: "Gedung Serbaguna", Address: "Jl. Contoh No. 2", MapsURL: "https://maps.google.com"},
		}
	}
	if len(v.Stories) == 0 {
		v.Sample = true
		v.Stories = []view.Story{
			{DateText: "2019", Title: "Pertama bertemu", Description: "Cerita singkat awal perjumpaan kalian."},
			{DateText: "2025", Title: "Lamaran", Description: "Momen berharga sebelum hari bahagia."},
		}
	}
	if len(v.Guestbook.Entries) == 0 {
		v.Sample = true
		v.Guestbook.Entries = []view.GuestbookEntry{
			{Name: "Rina", Message: "Selamat menempuh hidup baru! Semoga menjadi keluarga sakinah, mawaddah, warahmah.", DateText: v.DateText},
			{Name: "Andi & keluarga", Message: "Bahagia selalu untuk kalian berdua.", DateText: v.DateText},
		}
	}
	if len(v.Gifts) == 0 {
		v.Sample = true
		v.Gifts = []view.Gift{{Type: "bank", TypeLabel: "Rekening bank", Provider: "Bank Contoh", AccountNumber: "1234567890", AccountName: v.Couple.GroomName}}
	}
	if len(v.Gallery) == 0 {
		v.Sample = true
		for i := 1; i <= 4; i++ {
			u := img(fmt.Sprintf("photo-%d", i))
			v.Gallery = append(v.Gallery, view.Photo{URL: u, ThumbURL: u})
		}
	}
}

func timeText(start, end, tz string) string {
	s := strings.ReplaceAll(start, ":", ".")
	if end != "" {
		s += "–" + strings.ReplaceAll(end, ":", ".")
	} else {
		s += " – selesai"
	}
	return s + " " + tz
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
