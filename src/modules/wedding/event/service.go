// Package event: detail acara wedding (akad, resepsi, dll.). Sub-modul dari wedding.
// Semua operasi menerima weddingID yang sudah diotorisasi (RequireWeddingOwner).
package event

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/order"

	eventdb "github.com/khamdanngazis/lovaria/src/modules/wedding/event/db"
)

const (
	TypeAkad       = "akad"
	TypeReception  = "reception"
	TypeEngagement = "engagement"
	TypeOther      = "other"

	dateLayout = "2006-01-02"
	timeLayout = "15:04"
)

// Types berisi jenis acara beserta labelnya, urut untuk pilihan form.
var Types = []struct{ ID, Label string }{
	{TypeAkad, "Akad nikah"},
	{TypeReception, "Resepsi"},
	{TypeEngagement, "Lamaran"},
	{TypeOther, "Lainnya"},
}

// TypeLabel mengembalikan label jenis acara.
func TypeLabel(t string) string {
	for _, x := range Types {
		if x.ID == t {
			return x.Label
		}
	}
	return t
}

var ErrNotFound = errors.New("acara tidak ditemukan")

type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

// Event adalah satu acara. Waktu adalah waktu lokal di zona waktu wedding.
type Event struct {
	ID          uuid.UUID
	WeddingID   uuid.UUID
	Name        string
	Type        string
	Date        time.Time
	StartTime   string // "HH:MM"
	EndTime     string // "" bila tidak diisi
	Venue       string
	Address     string
	MapsURL     string
	Latitude    *float64
	Longitude   *float64
	Description string
	SortOrder   int
}

// Input adalah nilai mentah dari form.
type Input struct {
	Name        string
	Type        string
	Date        string // YYYY-MM-DD
	StartTime   string // HH:MM
	EndTime     string // HH:MM, opsional
	Venue       string
	Address     string
	MapsURL     string
	Description string
}

// parsed adalah Input yang sudah divalidasi.
type parsed struct {
	name, typ, venue, address, description string
	date                                   time.Time
	start, end                             pgtype.Time
	mapsURL                                *string
	lat, lng                               *float64
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---------- Validasi ----------

func validate(in Input) (parsed, error) {
	v := ValidationError{}
	p := parsed{
		name:        strings.TrimSpace(in.Name),
		typ:         strings.TrimSpace(in.Type),
		venue:       strings.TrimSpace(in.Venue),
		address:     strings.TrimSpace(in.Address),
		description: strings.TrimSpace(in.Description),
	}
	textField(v, "name", p.name, true, 100, "Nama acara")
	textField(v, "venue", p.venue, true, 150, "Tempat")
	textField(v, "address", p.address, false, 500, "Alamat")
	textField(v, "description", p.description, false, 1000, "Keterangan")

	if p.typ == "" {
		p.typ = TypeOther
	}
	if TypeLabel(p.typ) == p.typ {
		v["type"] = "Jenis acara tidak dikenal"
	}

	d, err := time.Parse(dateLayout, strings.TrimSpace(in.Date))
	switch {
	case strings.TrimSpace(in.Date) == "":
		v["date"] = "Tanggal wajib diisi"
	case err != nil || d.Year() < 2000 || d.Year() > 2100:
		v["date"] = "Tanggal tidak valid"
	}
	p.date = d

	var ok bool
	if p.start, ok = parseClock(in.StartTime); !ok {
		v["start_time"] = "Jam mulai wajib diisi (format JJ:MM)"
	}
	if strings.TrimSpace(in.EndTime) != "" {
		if p.end, ok = parseClock(in.EndTime); !ok {
			v["end_time"] = "Jam selesai tidak valid (format JJ:MM)"
		} else if p.start.Valid && p.end.Microseconds <= p.start.Microseconds {
			v["end_time"] = "Jam selesai harus setelah jam mulai"
		}
	}

	if raw := strings.TrimSpace(in.MapsURL); raw != "" {
		lat, lng, err := ParseMapsURL(raw)
		if err != nil {
			v["maps_url"] = err.Error()
		} else if len(raw) > 2048 {
			v["maps_url"] = "URL terlalu panjang"
		} else {
			p.mapsURL, p.lat, p.lng = &raw, lat, lng
		}
	}

	if len(v) > 0 {
		return parsed{}, v
	}
	return p, nil
}

func textField(v ValidationError, key, val string, required bool, maxLen int, label string) {
	n := utf8.RuneCountInString(val)
	switch {
	case required && n == 0:
		v[key] = label + " wajib diisi"
	case n > maxLen:
		v[key] = fmt.Sprintf("%s maksimal %d karakter", label, maxLen)
	}
}

func parseClock(s string) (pgtype.Time, bool) {
	t, err := time.Parse(timeLayout, strings.TrimSpace(s))
	if err != nil {
		return pgtype.Time{}, false
	}
	us := int64(t.Hour()*3600+t.Minute()*60) * 1_000_000
	return pgtype.Time{Microseconds: us, Valid: true}, true
}

func formatClock(t pgtype.Time) string {
	if !t.Valid {
		return ""
	}
	mins := t.Microseconds / 60_000_000
	return fmt.Sprintf("%02d:%02d", mins/60, mins%60)
}

func toEvent(e eventdb.Event) Event {
	out := Event{
		ID: e.ID, WeddingID: e.WeddingID, Name: e.Name, Type: e.Type, Date: e.EventDate,
		StartTime: formatClock(e.StartTime), EndTime: formatClock(e.EndTime),
		Venue: e.Venue, Address: e.Address, Latitude: e.Latitude, Longitude: e.Longitude,
		Description: e.Description, SortOrder: int(e.SortOrder),
	}
	if e.MapsUrl != nil {
		out.MapsURL = *e.MapsUrl
	}
	return out
}

// chronoKey membandingkan event secara kronologis.
func chronoKey(date time.Time, start pgtype.Time) int64 {
	return date.Unix()*1_000_000 + start.Microseconds
}

// ---------- Tulis ----------

// CreateEvent menambah acara, disisipkan sesuai urutan kronologis tanpa mengubah
// urutan manual acara lain.
func (s *Service) CreateEvent(ctx context.Context, weddingID uuid.UUID, in Input) (Event, error) {
	p, err := validate(in)
	if err != nil {
		return Event{}, err
	}
	var created eventdb.Event
	err = s.repo.inTx(ctx, func(q *eventdb.Queries) error {
		existing, err := q.ListEventsForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		key := chronoKey(p.date, p.start)
		pos := order.ChronoPosition(len(existing), func(i int) bool {
			return chronoKey(existing[i].EventDate, existing[i].StartTime) > key
		})
		created, err = q.CreateEvent(ctx, eventdb.CreateEventParams{
			ID: db.NewID(), WeddingID: weddingID, Name: p.name, Type: p.typ,
			EventDate: p.date, StartTime: p.start, EndTime: p.end,
			Venue: p.venue, Address: p.address, MapsUrl: p.mapsURL,
			Latitude: p.lat, Longitude: p.lng, Description: p.description,
		})
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(existing))
		for i, e := range existing {
			ids[i] = e.ID
		}
		return saveOrder(ctx, q, weddingID, order.InsertAt(ids, pos, created.ID))
	})
	if err != nil {
		return Event{}, fmt.Errorf("event: create: %w", err)
	}
	return s.GetEvent(ctx, weddingID, created.ID)
}

// UpdateEvent mengubah acara. Urutan tidak berubah (pakai SortEventsByDate untuk reset).
func (s *Service) UpdateEvent(ctx context.Context, weddingID, id uuid.UUID, in Input) (Event, error) {
	p, err := validate(in)
	if err != nil {
		return Event{}, err
	}
	e, err := s.repo.q.UpdateEvent(ctx, eventdb.UpdateEventParams{
		ID: id, WeddingID: weddingID, Name: p.name, Type: p.typ,
		EventDate: p.date, StartTime: p.start, EndTime: p.end,
		Venue: p.venue, Address: p.address, MapsUrl: p.mapsURL,
		Latitude: p.lat, Longitude: p.lng, Description: p.description,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("event: update: %w", err)
	}
	return toEvent(e), nil
}

func (s *Service) DeleteEvent(ctx context.Context, weddingID, id uuid.UUID) error {
	n, err := s.repo.q.DeleteEvent(ctx, eventdb.DeleteEventParams{ID: id, WeddingID: weddingID})
	if err != nil {
		return fmt.Errorf("event: delete: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MoveEvent menggeser acara satu posisi ke atas/bawah.
func (s *Service) MoveEvent(ctx context.Context, weddingID, id uuid.UUID, up bool) error {
	return s.repo.inTx(ctx, func(q *eventdb.Queries) error {
		rows, err := q.ListEventsForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(rows))
		found := false
		for i, r := range rows {
			ids[i] = r.ID
			found = found || r.ID == id
		}
		if !found {
			return ErrNotFound
		}
		if next, changed := order.Move(ids, id, up); changed {
			return saveOrder(ctx, q, weddingID, next)
		}
		return nil
	})
}

// SortEventsByDate mengatur ulang urutan berdasarkan tanggal & jam mulai.
func (s *Service) SortEventsByDate(ctx context.Context, weddingID uuid.UUID) error {
	return s.repo.inTx(ctx, func(q *eventdb.Queries) error {
		rows, err := q.ListEventsForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		sortChrono(rows)
		ids := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
		return saveOrder(ctx, q, weddingID, ids)
	})
}

func sortChrono(rows []eventdb.Event) {
	// Insertion sort stabil: jumlah acara kecil.
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && chronoKey(rows[j].EventDate, rows[j].StartTime) < chronoKey(rows[j-1].EventDate, rows[j-1].StartTime); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

// ---------- Baca ----------

func (s *Service) GetEvent(ctx context.Context, weddingID, id uuid.UUID) (Event, error) {
	e, err := s.repo.q.GetEvent(ctx, eventdb.GetEventParams{ID: id, WeddingID: weddingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	if err != nil {
		return Event{}, err
	}
	return toEvent(e), nil
}

// CountEvents menghitung acara wedding (dipakai checklist publikasi, T12).
func (s *Service) CountEvents(ctx context.Context, weddingID uuid.UUID) (int, error) {
	n, err := s.repo.q.CountEvents(ctx, weddingID)
	return int(n), err
}

// ListEvents mengembalikan acara wedding sesuai urutan tampil (dipakai public site, T09).
func (s *Service) ListEvents(ctx context.Context, weddingID uuid.UUID) ([]Event, error) {
	rows, err := s.repo.q.ListEvents(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = toEvent(r)
	}
	return out, nil
}
