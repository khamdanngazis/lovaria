package event

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

type fixture struct {
	svc      *Service
	weddings *wedding.Service
	auth     *auth.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return fixture{
		svc:      NewService(NewRepository(pool)),
		weddings: wedding.NewService(wedding.NewRepository(pool)),
		auth:     auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
	}
}

// newWedding membuat user + wedding baru dan mengembalikan (ownerID, wedding).
func (f fixture) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func input(name, date, start string) Input {
	return Input{Name: name, Type: TypeReception, Date: date, StartTime: start, Venue: "Gedung"}
}

func names(t *testing.T, f fixture, weddingID uuid.UUID) []string {
	t.Helper()
	evs, err := f.svc.ListEvents(ctx, weddingID)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Name
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCreateEventFields(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")

	e, err := f.svc.CreateEvent(ctx, w.ID, Input{
		Name: " Akad Nikah ", Type: TypeAkad, Date: "2026-12-12", StartTime: "08:00", EndTime: "10:30",
		Venue: "Masjid Istiqlal", Address: "Jl. Taman Wijaya Kusuma", Description: "Keluarga inti",
		MapsURL: "https://www.google.com/maps/place/x/@-6.1701,106.8310,17z",
	})
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Akad Nikah" || e.StartTime != "08:00" || e.EndTime != "10:30" || e.Date.Format(dateLayout) != "2026-12-12" {
		t.Errorf("event = %+v", e)
	}
	if e.Latitude == nil || *e.Latitude != -6.1701 || *e.Longitude != 106.8310 {
		t.Errorf("koordinat = %v %v", e.Latitude, e.Longitude)
	}
	if e.WeddingID != w.ID {
		t.Error("wedding_id salah")
	}
}

func TestEventValidation(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	_, err := f.svc.CreateEvent(ctx, w.ID, Input{Type: "pesta", Date: "2026-13-01", StartTime: "25:00", EndTime: "x", MapsURL: "https://waze.com/x"})
	var v ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("err = %v", err)
	}
	for _, k := range []string{"name", "venue", "type", "date", "start_time", "end_time", "maps_url"} {
		if v[k] == "" {
			t.Errorf("field %s harus error", k)
		}
	}
	_, err = f.svc.CreateEvent(ctx, w.ID, Input{Name: "x", Venue: "y", Date: "2026-12-12", StartTime: "10:00", EndTime: "09:00"})
	if !errors.As(err, &v) || v["end_time"] != "Jam selesai harus setelah jam mulai" {
		t.Errorf("end < start: %v", err)
	}
}

func TestEventOrdering(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	mk := func(name, date, start string) Event {
		e, err := f.svc.CreateEvent(ctx, w.ID, input(name, date, start))
		if err != nil {
			t.Fatal(err)
		}
		return e
	}

	// Default: disisipkan kronologis.
	resepsi := mk("Resepsi", "2026-12-12", "11:00")
	akad := mk("Akad", "2026-12-12", "08:00")
	lamaran := mk("Lamaran", "2026-06-01", "10:00")
	if got := names(t, f, w.ID); !eq(got, []string{"Lamaran", "Akad", "Resepsi"}) {
		t.Fatalf("kronologis: %v", got)
	}

	// Manual: Resepsi naik dua kali → paling atas.
	for i := 0; i < 2; i++ {
		if err := f.svc.MoveEvent(ctx, w.ID, resepsi.ID, true); err != nil {
			t.Fatal(err)
		}
	}
	if got := names(t, f, w.ID); !eq(got, []string{"Resepsi", "Lamaran", "Akad"}) {
		t.Fatalf("manual: %v", got)
	}
	// Naik lagi saat sudah paling atas → tidak berubah.
	_ = f.svc.MoveEvent(ctx, w.ID, resepsi.ID, true)

	// Acara baru disisipkan sebelum item kronologis berikutnya, urutan manual lain tetap.
	mk("Siraman", "2026-12-11", "09:00")
	// Item pertama yang lebih akhir dari Siraman (11 Des) adalah Resepsi (12 Des) di posisi 0.
	if got := names(t, f, w.ID); !eq(got, []string{"Siraman", "Resepsi", "Lamaran", "Akad"}) {
		t.Fatalf("sisip: %v", got)
	}

	// Edit tanggal tidak mengubah urutan; reset lewat SortEventsByDate.
	if _, err := f.svc.UpdateEvent(ctx, w.ID, akad.ID, input("Akad", "2027-01-01", "08:00")); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SortEventsByDate(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	if got := names(t, f, w.ID); !eq(got, []string{"Lamaran", "Siraman", "Resepsi", "Akad"}) {
		t.Fatalf("sort by date: %v", got)
	}

	// Urutan persisten & konsisten: sort_order 0..n-1 tanpa duplikat.
	evs, _ := f.svc.ListEvents(ctx, w.ID)
	for i, e := range evs {
		if e.SortOrder != i {
			t.Errorf("%s sort_order = %d, want %d", e.Name, e.SortOrder, i)
		}
	}

	if err := f.svc.DeleteEvent(ctx, w.ID, lamaran.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.DeleteEvent(ctx, w.ID, lamaran.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("hapus 2x: %v", err)
	}
	if got := names(t, f, w.ID); len(got) != 3 {
		t.Errorf("setelah hapus: %v", got)
	}
}

// Semua query memfilter wedding_id: ID acara wedding lain diperlakukan tidak ada.
func TestEventWeddingIsolation(t *testing.T) {
	f := newFixture(t)
	_, wa := f.newWedding(t, "a@example.com")
	_, wb := f.newWedding(t, "b@example.com")
	e, err := f.svc.CreateEvent(ctx, wa.ID, input("Akad", "2026-12-12", "08:00"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.svc.GetEvent(ctx, wb.ID, e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := f.svc.UpdateEvent(ctx, wb.ID, e.ID, input("Dibajak", "2026-12-12", "08:00")); !errors.Is(err, ErrNotFound) {
		t.Errorf("update: %v", err)
	}
	if err := f.svc.DeleteEvent(ctx, wb.ID, e.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete: %v", err)
	}
	if err := f.svc.MoveEvent(ctx, wb.ID, e.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("move: %v", err)
	}
	if got := names(t, f, wb.ID); len(got) != 0 {
		t.Errorf("wedding B melihat acara A: %v", got)
	}
	if got, _ := f.svc.GetEvent(ctx, wa.ID, e.ID); got.Name != "Akad" {
		t.Error("acara A berubah")
	}
}
