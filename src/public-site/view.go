package publicsite

import (
	"context"
	"fmt"
	"strings"
	"time"

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

	// Now: jam sekarang untuk hitung mundur (nil = time.Now; diganti di test).
	Now func() time.Time

	// CacheTTL: umur cache BuildPublic (0 = default 10 detik, < 0 = tanpa cache).
	CacheTTL time.Duration
	cache    viewCache // BuildPublic (halaman undangan publik)
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
		Archived:       w.IsArchived(),
		AllowRSVP:      w.AllowsRSVP(),
		AllowGuestbook: w.AllowsGuestbook(),
	}
	if w.MainPhotoURL != nil {
		v.MainPhoto = *w.MainPhotoURL
	}
	if w.ShowsMemoryLayout() {
		photos, err := b.Gallery.ByCategory(ctx, w.ID, gallery.CategoryWedding, 12)
		if err != nil {
			return view.View{}, fmt.Errorf("view: foto hari-H: %w", err)
		}
		for _, p := range photos {
			v.MemoryPhotos = append(v.MemoryPhotos, view.Photo{URL: p.URL, ThumbURL: p.ThumbURL, Caption: p.Caption, Width: p.Width, Height: p.Height})
		}
		favs, err := b.Guestbook.Favorites(ctx, w.ID, 6)
		if err != nil {
			return view.View{}, fmt.Errorf("view: ucapan favorit: %w", err)
		}
		v.Guestbook.Favorites = guestbookEntries(favs, w.Timezone)
	}
	// Arsip: daftar ucapan tetap tampil (read-only) walau form ditutup.
	if v.AllowGuestbook || w.IsArchived() {
		es, more, err := b.Guestbook.Visible(ctx, w.ID, uuid.Nil, guestbook.PublicPage)
		if err != nil {
			return view.View{}, fmt.Errorf("view: guestbook: %w", err)
		}
		v.Guestbook.Entries = guestbookEntries(es, w.Timezone)
		if more {
			v.Guestbook.MoreBefore = es[len(es)-1].ID.String()
		}
	}
	if w.IsArchived() {
		gifts = nil // tanda kasih tidak relevan lagi setelah diarsipkan
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
	v.Countdown = countdown(evs, w, b.now())
	for _, s := range sts {
		v.Stories = append(v.Stories, view.Story{DateText: s.Date.String(), Title: s.Title, Description: s.Description, PhotoURL: s.PhotoURL})
	}
	cover := settings.CoverImage
	if cover == "" {
		cover = v.MainPhoto
	}
	for _, p := range photos {
		if p.URL == cover && cover != "" {
			v.CoverThumb, v.CoverWidth = p.ThumbURL, p.Width // srcset sampul (T21)
		}
		if p.Category == gallery.CategoryCover {
			continue // foto sampul tampil di pembuka, bukan di galeri
		}
		v.Gallery = append(v.Gallery, view.Photo{URL: p.URL, ThumbURL: p.ThumbURL, Caption: p.Caption, Width: p.Width, Height: p.Height})
	}
	return v, nil
}

func (b *ViewBuilder) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

// countdown: hitung mundur menuju acara pertama (tanggal + jam mulai di zona
// waktu wedding; tanpa acara → tanggal pernikahan 00.00). Sisa hari dihitung per
// hari kalender di zona waktu wedding; setelah hari acara lewat tidak tampil.
func countdown(evs []event.Event, w wedding.Wedding, now time.Time) view.Countdown {
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		loc = time.UTC
	}
	at := func(d time.Time, hhmm string) time.Time {
		h, m := 0, 0
		if t, err := time.Parse("15:04", hhmm); err == nil {
			h, m = t.Hour(), t.Minute()
		}
		return time.Date(d.Year(), d.Month(), d.Day(), h, m, 0, 0, loc)
	}
	c := view.Countdown{Target: at(w.WeddingDate, "")}
	first := true
	for _, e := range evs {
		if t := at(e.Date, e.StartTime); first || t.Before(c.Target) {
			c.Target, c.EventID, first = t, e.ID.String(), false
		}
	}
	day := func(t time.Time) time.Time {
		t = t.In(loc)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	days := int(day(c.Target).Sub(day(now)).Hours() / 24)
	c.Show, c.Today, c.DaysLeft = days >= 0, days == 0, max(days, 0)
	return c
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
