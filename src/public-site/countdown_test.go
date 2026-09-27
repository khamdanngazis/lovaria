package publicsite

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
)

func TestCountdownTimezones(t *testing.T) {
	day := time.Date(2026, 12, 12, 0, 0, 0, 0, time.UTC)
	resepsi := event.Event{ID: uuid.New(), Date: day, StartTime: "11:00"}
	akad := event.Event{ID: uuid.New(), Date: day, StartTime: "08:00"}
	for _, c := range []struct {
		tz, now    string
		target     string // UTC
		show       bool
		today      bool
		daysLeft   int
		eventFirst bool
	}{
		// WIB (UTC+7): akad 08.00 WIB = 01.00 UTC.
		{"Asia/Jakarta", "2026-12-01T00:00:00Z", "2026-12-12T01:00:00Z", true, false, 11, true},
		{"Asia/Jakarta", "2026-12-11T16:59:00Z", "2026-12-12T01:00:00Z", true, false, 1, true},  // 23.59 WIB tgl 11
		{"Asia/Jakarta", "2026-12-11T17:00:00Z", "2026-12-12T01:00:00Z", true, true, 0, true},   // 00.00 WIB tgl 12
		{"Asia/Jakarta", "2026-12-12T16:59:00Z", "2026-12-12T01:00:00Z", true, true, 0, true},   // masih tgl 12 WIB
		{"Asia/Jakarta", "2026-12-12T17:00:00Z", "2026-12-12T01:00:00Z", false, false, 0, true}, // tgl 13 WIB
		// WIT (UTC+9): akad 08.00 WIT = 23.00 UTC tgl 11.
		{"Asia/Jayapura", "2026-12-11T14:59:00Z", "2026-12-11T23:00:00Z", true, false, 1, true},
		{"Asia/Jayapura", "2026-12-11T15:00:00Z", "2026-12-11T23:00:00Z", true, true, 0, true},
		{"Asia/Jayapura", "2026-12-12T15:00:00Z", "2026-12-11T23:00:00Z", false, false, 0, true},
	} {
		w := wedding.Wedding{WeddingDate: day, Timezone: c.tz}
		now, _ := time.Parse(time.RFC3339, c.now)
		got := countdown([]event.Event{resepsi, akad}, w, now)
		if got.Target.UTC().Format(time.RFC3339) != c.target || got.Show != c.show || got.Today != c.today || got.DaysLeft != c.daysLeft || got.EventID != akad.ID.String() {
			t.Errorf("%s %s: %+v", c.tz, c.now, got)
		}
	}
	// Tanpa acara: menuju tanggal pernikahan 00.00.
	got := countdown(nil, wedding.Wedding{WeddingDate: day, Timezone: "Asia/Jakarta"}, time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC))
	if got.Target.UTC().Format(time.RFC3339) != "2026-12-11T17:00:00Z" || got.DaysLeft != 2 || got.EventID != "" {
		t.Errorf("tanpa acara: %+v", got)
	}
}

func TestPersonalizedInvitation(t *testing.T) {
	f := newFixture(t)
	f.views.Now = func() time.Time { return time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) }
	_, w := f.newWedding(t, "a@example.com", "Khamdan", "Sarah")
	f.publish(w.ID)
	ev, _ := f.events.CreateEvent(ctx, w.ID, event.Input{Name: "Akad Nikah", Type: event.TypeAkad, Date: "2026-12-12", StartTime: "08:00", Venue: "Masjid"})
	g, _ := f.guests.Create(ctx, w.ID, guest.Input{Name: "Budi"})
	if _, err := f.themes.Save(ctx, w.ID, "romantic", view.Settings{
		QuoteText: "Kasih itu sabar", QuoteSource: "1 Korintus 13:4", Greeting: "Teruntuk", Closing: "Sampai jumpa",
		SectionOrder: []string{"quote", "countdown"}, HiddenSections: []string{"gift"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/w/" + w.Slug, "/i/" + g.InvitationCode} {
		body := f.get(p, nil).Body.String()
		for _, want := range []string{"Kasih itu sabar", "Teruntuk", "Sampai jumpa", "11 hari lagi", `data-countdown="2026-12-12T01:00:00Z"`, "/events/" + ev.ID.String() + ".ics"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: tidak ada %q", p, want)
			}
		}
		// Urutan tersimpan sebagian: mempelai tetap di posisi bawaannya (paling atas).
		if strings.Index(body, `id="couple"`) > strings.Index(body, `id="quote"`) || strings.Index(body, `id="quote"`) > strings.Index(body, `id="countdown"`) || strings.Contains(body, `id="gift"`) {
			t.Errorf("%s: urutan bagian salah", p)
		}
		if strings.Contains(body, `id="lv-music"`) {
			t.Errorf("%s: musik belum dipilih", p)
		}
	}
	// Hari H → "Hari ini!"; setelah lewat → tidak tampil.
	f.views.Now = func() time.Time { return time.Date(2026, 12, 12, 3, 0, 0, 0, time.UTC) }
	if body := f.get("/w/"+w.Slug, nil).Body.String(); !strings.Contains(body, "Hari ini!") || strings.Contains(body, "data-countdown") {
		t.Error("hari H")
	}
	f.views.Now = func() time.Time { return time.Date(2026, 12, 13, 3, 0, 0, 0, time.UTC) }
	if strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), `id="countdown"`) {
		t.Error("setelah hari acara hitung mundur tidak tampil")
	}
	// Pengaturan tersimpan langsung terlihat walau halaman di-cache (OnChange).
	f.views.CacheTTL = time.Hour
	f.themes.OnChange(f.views.Invalidate)
	f.get("/w/"+w.Slug, nil)
	if _, err := f.themes.Save(ctx, w.ID, "romantic", view.Settings{QuoteText: "Kutipan baru"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.get("/w/"+w.Slug, nil).Body.String(), "Kutipan baru") {
		t.Error("simpan tema harus mengosongkan cache")
	}
}
