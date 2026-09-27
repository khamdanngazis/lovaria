package dashboard

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/a-h/templ"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
)

// Home menyusun widget beranda wedding (T13). Dashboard hanya agregator:
// semua angka diambil dari service modul masing-masing, tanpa query sendiri.
type Home struct {
	Weddings  *wedding.Service
	Events    *event.Service
	Stories   *story.Service
	Gallery   *gallery.Service
	Themes    *theme.Service
	Guests    *guest.Service
	Guestbook *guestbook.Service
	BaseURL   string
}

// step adalah satu langkah checklist onboarding.
type step struct {
	Label, Hint, URL string
	Done             bool
}

type homeData struct {
	W          wedding.Wedding
	Link       string // link undangan umum (/w/slug)
	Guests     guest.Stats
	Messages   []guestbook.Entry
	Gallery    gallery.Summary
	Onboarding []step
}

func (d homeData) done() int {
	n := 0
	for _, s := range d.Onboarding {
		if s.Done {
			n++
		}
	}
	return n
}

// rate: persentase a dari b (0 bila b = 0).
func rate(a, b int) int {
	if b == 0 {
		return 0
	}
	return a * 100 / b
}

func (d homeData) openRate() int { return rate(d.Guests.Opened, d.Guests.Total) }
func (d homeData) rsvpRate() int {
	return rate(d.Guests.Attending+d.Guests.Declined, d.Guests.Total)
}

func (d homeData) storagePercent() int {
	u := d.Gallery.Usage
	if u.QuotaBytes <= 0 {
		return 0
	}
	return int(min(100, u.UsedBytes*100/u.QuotaBytes))
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d KB", (n+1023)/1024)
	}
}

// Widgets memenuhi wedding.HomeWidgets.
func (h *Home) Widgets(ctx context.Context, w wedding.Wedding) (templ.Component, error) {
	d, err := h.load(ctx, w)
	if err != nil {
		return nil, err
	}
	return widgets(d), nil
}

func (h *Home) load(ctx context.Context, w wedding.Wedding) (homeData, error) {
	d := homeData{W: w, Link: h.BaseURL + "/w/" + w.Slug}
	var err error
	if d.Guests, err = h.Guests.Stats(ctx, w.ID); err != nil {
		return d, fmt.Errorf("dashboard: tamu: %w", err)
	}
	if d.Messages, err = h.Guestbook.Recent(ctx, w.ID, 5); err != nil {
		return d, fmt.Errorf("dashboard: ucapan: %w", err)
	}
	if d.Gallery, err = h.Gallery.Summary(ctx, w.ID, 6); err != nil {
		return d, fmt.Errorf("dashboard: galeri: %w", err)
	}
	couple, err := h.Weddings.GetCouple(ctx, w.ID)
	if err != nil {
		return d, fmt.Errorf("dashboard: pasangan: %w", err)
	}
	events, err := h.Events.CountEvents(ctx, w.ID)
	if err != nil {
		return d, fmt.Errorf("dashboard: acara: %w", err)
	}
	stories, err := h.Stories.ListStories(ctx, w.ID)
	if err != nil {
		return d, fmt.Errorf("dashboard: cerita: %w", err)
	}
	themed, err := h.Themes.Configured(ctx, w.ID)
	if err != nil {
		return d, fmt.Errorf("dashboard: tema: %w", err)
	}
	profile := func(photo *string, desc string) bool { return (photo != nil && *photo != "") || desc != "" }
	d.Onboarding = []step{
		{"Profil pasangan", "Foto atau deskripsi kedua mempelai", w.DashboardURL("/couple"),
			profile(couple.GroomPhotoURL, couple.GroomDescription) && profile(couple.BridePhotoURL, couple.BrideDescription)},
		{"Acara", "Akad, resepsi, dan lokasinya", w.DashboardURL("/events"), events > 0},
		{"Cerita cinta", "Perjalanan kalian berdua", w.DashboardURL("/stories"), len(stories) > 0},
		{"Galeri foto", "Foto prewedding & momen favorit", w.DashboardURL("/gallery"), d.Gallery.Count > 0},
		{"Tema", "Pilih tampilan undangan", w.DashboardURL("/theme"), themed},
		{"Daftar tamu", "Tambah tamu untuk link undangan pribadi", w.DashboardURL("/guests"), d.Guests.Total > 0},
		{"Publikasikan", "Terbitkan saat semua siap", w.DashboardURL(""), !w.IsDraft()},
	}
	return d, nil
}

// copyData: ekspresi x-data komponen Alpine copyText (string di-encode JSON).
func copyData(text string) string {
	b, _ := json.Marshal(text)
	return "copyText(" + string(b) + ")"
}
