// Package story: perjalanan cerita pasangan (love story). Sub-modul dari wedding.
// Semua operasi menerima weddingID yang sudah diotorisasi (RequireWeddingOwner).
package story

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/order"

	storydb "github.com/khamdanngazis/lovaria/src/modules/wedding/story/db"
)

var ErrNotFound = errors.New("cerita tidak ditemukan")

type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

// Date adalah tanggal cerita: tahun wajib, bulan & hari opsional.
type Date struct {
	Year  int
	Month int // 0 = tidak diisi
	Day   int // 0 = tidak diisi
}

var bulanID = [...]string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

// MonthName mengembalikan nama bulan Indonesia (1..12).
func MonthName(m int) string {
	if m < 1 || m > 12 {
		return ""
	}
	return bulanID[m-1]
}

// String: "2019", "Maret 2019", atau "12 Maret 2019".
func (d Date) String() string {
	switch {
	case d.Month == 0:
		return strconv.Itoa(d.Year)
	case d.Day == 0:
		return fmt.Sprintf("%s %d", MonthName(d.Month), d.Year)
	default:
		return fmt.Sprintf("%d %s %d", d.Day, MonthName(d.Month), d.Year)
	}
}

// key untuk perbandingan kronologis (bagian kosong dianggap paling awal).
func (d Date) key() int { return d.Year*10000 + d.Month*100 + d.Day }

type Story struct {
	ID          uuid.UUID
	WeddingID   uuid.UUID
	Date        Date
	Title       string
	Description string
	PhotoURL    string
	SortOrder   int
}

// Input adalah nilai mentah dari form.
type Input struct {
	Year        string
	Month       string // "" atau 1..12
	Day         string // "" atau 1..31
	Title       string
	Description string
	PhotoURL    string
}

type parsed struct {
	date               Date
	title, description string
	photoURL           *string
}

// ---------- Validasi ----------

func validate(in Input) (parsed, error) {
	v := ValidationError{}
	p := parsed{title: strings.TrimSpace(in.Title), description: strings.TrimSpace(in.Description)}

	switch n := utf8.RuneCountInString(p.title); {
	case n == 0:
		v["title"] = "Judul wajib diisi"
	case n > 150:
		v["title"] = "Judul maksimal 150 karakter"
	}
	if utf8.RuneCountInString(p.description) > 2000 {
		v["description"] = "Cerita maksimal 2000 karakter"
	}

	year, err := strconv.Atoi(strings.TrimSpace(in.Year))
	if err != nil || year < 1900 || year > 2100 {
		v["year"] = "Tahun wajib diisi (1900–2100)"
	}
	p.date.Year = year

	if m := strings.TrimSpace(in.Month); m != "" {
		month, err := strconv.Atoi(m)
		if err != nil || month < 1 || month > 12 {
			v["month"] = "Bulan tidak valid"
		}
		p.date.Month = month
	}
	if d := strings.TrimSpace(in.Day); d != "" {
		day, err := strconv.Atoi(d)
		switch {
		case p.date.Month == 0:
			v["day"] = "Pilih bulan dulu sebelum mengisi tanggal"
		case err != nil || day < 1 || day > 31:
			v["day"] = "Tanggal tidak valid"
		case v["year"] == "" && v["month"] == "" && !validDay(year, p.date.Month, day):
			v["day"] = fmt.Sprintf("%s %d tidak punya tanggal %d", MonthName(p.date.Month), year, day)
		}
		p.date.Day = day
	}

	if raw := strings.TrimSpace(in.PhotoURL); raw != "" {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(raw) > 2048 {
			v["photo_url"] = "URL foto tidak valid (harus diawali https://)"
		}
		p.photoURL = &raw
	}

	if len(v) > 0 {
		return parsed{}, v
	}
	return p, nil
}

func validDay(year, month, day int) bool {
	t := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return t.Day() == day
}

func int16Ptr(n int) *int16 {
	if n == 0 {
		return nil
	}
	v := int16(n) //nolint:gosec // G115: sudah divalidasi 1..31
	return &v
}

func intOf(p *int16) int {
	if p == nil {
		return 0
	}
	return int(*p)
}

func toStory(r storydb.LoveStory) Story {
	s := Story{
		ID: r.ID, WeddingID: r.WeddingID, Title: r.Title, Description: r.Description,
		Date:      Date{Year: int(r.DateYear), Month: intOf(r.DateMonth), Day: intOf(r.DateDay)},
		SortOrder: int(r.SortOrder),
	}
	if r.PhotoUrl != nil {
		s.PhotoURL = *r.PhotoUrl
	}
	return s
}

// ---------- Service ----------

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateStory menambah cerita, disisipkan sesuai kronologi tanpa mengubah urutan manual lainnya.
func (s *Service) CreateStory(ctx context.Context, weddingID uuid.UUID, in Input) (Story, error) {
	p, err := validate(in)
	if err != nil {
		return Story{}, err
	}
	var id uuid.UUID
	err = s.repo.inTx(ctx, func(q *storydb.Queries) error {
		existing, err := q.ListStoriesForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		pos := order.ChronoPosition(len(existing), func(i int) bool {
			return toStory(existing[i]).Date.key() > p.date.key()
		})
		row, err := q.CreateStory(ctx, storydb.CreateStoryParams{
			ID: db.NewID(), WeddingID: weddingID,
			DateYear: int16(p.date.Year), DateMonth: int16Ptr(p.date.Month), DateDay: int16Ptr(p.date.Day), //nolint:gosec // G115: tahun divalidasi 1900..2100
			Title: p.title, Description: p.description, PhotoUrl: p.photoURL,
		})
		if err != nil {
			return err
		}
		id = row.ID
		ids := make([]uuid.UUID, len(existing))
		for i, e := range existing {
			ids[i] = e.ID
		}
		return saveOrder(ctx, q, weddingID, order.InsertAt(ids, pos, id))
	})
	if err != nil {
		return Story{}, fmt.Errorf("story: create: %w", err)
	}
	return s.GetStory(ctx, weddingID, id)
}

// UpdateStory mengubah cerita. Urutan tidak berubah (pakai SortStoriesByDate untuk reset).
func (s *Service) UpdateStory(ctx context.Context, weddingID, id uuid.UUID, in Input) (Story, error) {
	p, err := validate(in)
	if err != nil {
		return Story{}, err
	}
	row, err := s.repo.q.UpdateStory(ctx, storydb.UpdateStoryParams{
		ID: id, WeddingID: weddingID,
		DateYear: int16(p.date.Year), DateMonth: int16Ptr(p.date.Month), DateDay: int16Ptr(p.date.Day), //nolint:gosec // G115: tahun divalidasi 1900..2100
		Title: p.title, Description: p.description, PhotoUrl: p.photoURL,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Story{}, ErrNotFound
	}
	if err != nil {
		return Story{}, fmt.Errorf("story: update: %w", err)
	}
	return toStory(row), nil
}

func (s *Service) DeleteStory(ctx context.Context, weddingID, id uuid.UUID) error {
	n, err := s.repo.q.DeleteStory(ctx, storydb.DeleteStoryParams{ID: id, WeddingID: weddingID})
	if err != nil {
		return fmt.Errorf("story: delete: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MoveStory menggeser cerita satu posisi ke atas/bawah.
func (s *Service) MoveStory(ctx context.Context, weddingID, id uuid.UUID, up bool) error {
	return s.repo.inTx(ctx, func(q *storydb.Queries) error {
		rows, err := q.ListStoriesForUpdate(ctx, weddingID)
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

// SortStoriesByDate mengatur ulang urutan berdasarkan tanggal.
func (s *Service) SortStoriesByDate(ctx context.Context, weddingID uuid.UUID) error {
	return s.repo.inTx(ctx, func(q *storydb.Queries) error {
		rows, err := q.ListStoriesForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		for i := 1; i < len(rows); i++ { // insertion sort stabil
			for j := i; j > 0 && toStory(rows[j]).Date.key() < toStory(rows[j-1]).Date.key(); j-- {
				rows[j], rows[j-1] = rows[j-1], rows[j]
			}
		}
		ids := make([]uuid.UUID, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
		}
		return saveOrder(ctx, q, weddingID, ids)
	})
}

func (s *Service) GetStory(ctx context.Context, weddingID, id uuid.UUID) (Story, error) {
	row, err := s.repo.q.GetStory(ctx, storydb.GetStoryParams{ID: id, WeddingID: weddingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Story{}, ErrNotFound
	}
	if err != nil {
		return Story{}, err
	}
	return toStory(row), nil
}

// ListStories mengembalikan cerita sesuai urutan tampil (dipakai public site, T09).
func (s *Service) ListStories(ctx context.Context, weddingID uuid.UUID) ([]Story, error) {
	rows, err := s.repo.q.ListStories(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	out := make([]Story, len(rows))
	for i, r := range rows {
		out[i] = toStory(r)
	}
	return out, nil
}

// RebaseMediaURLs mengganti basis URL foto cerita (lihat wedding.Service.RebaseMediaURLs).
func (s *Service) RebaseMediaURLs(ctx context.Context, oldPrefix, newPrefix string, apply bool) (int64, error) {
	if !apply {
		return s.repo.q.CountStoryPhotoPrefix(ctx, oldPrefix)
	}
	return s.repo.q.RebaseStoryPhotoURL(ctx, storydb.RebaseStoryPhotoURLParams{OldPrefix: oldPrefix, NewPrefix: newPrefix})
}
