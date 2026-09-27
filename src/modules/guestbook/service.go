// Package guestbook: ucapan & doa tamu di halaman undangan (Produk §12).
// Operasi dashboard menerima weddingID yang sudah diotorisasi
// (RequireWeddingOwner); Post & Visible dipakai public site dengan wedding
// hasil resolver.
package guestbook

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	guestbookdb "github.com/khamdanngazis/lovaria/src/modules/guestbook/db"
)

const (
	MaxName    = 100
	MaxMessage = 500
	// PublicPage: jumlah pesan per muatan di halaman undangan ("Muat lebih banyak").
	PublicPage = 10
	// DashboardPage: jumlah pesan per halaman di dashboard.
	DashboardPage = 25
)

var ErrNotFound = errors.New("pesan tidak ditemukan")

// ValidationError memetakan nama field ke pesan error.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

type Entry struct {
	ID        uuid.UUID
	WeddingID uuid.UUID
	GuestID   *uuid.UUID
	Name      string
	Message   string
	Hidden    bool
	Favorite  bool // dipilih pasangan: tampil paling atas saat Kenangan & arsip (T19)
	CreatedAt time.Time
}

// Stats: jumlah pesan di dashboard.
type Stats struct{ Total, Hidden int }

func (s Stats) Visible() int { return s.Total - s.Hidden }

type Service struct {
	q        *guestbookdb.Queries
	filter   *WordFilter
	onChange func(weddingID uuid.UUID) // mis. kosongkan cache halaman publik
}

// OnChange memasang fungsi yang dipanggil setelah pasangan mengubah ucapan
// (sembunyikan, favorit, hapus) — dipakai untuk mengosongkan cache undangan.
func (s *Service) OnChange(f func(weddingID uuid.UUID)) { s.onChange = f }

func (s *Service) changed(weddingID uuid.UUID) {
	if s.onChange != nil {
		s.onChange(weddingID)
	}
}

// NewService: filter boleh nil (tanpa penyaringan kata kasar).
func NewService(pool *pgxpool.Pool, filter *WordFilter) *Service {
	return &Service{q: guestbookdb.New(pool), filter: filter}
}

func toEntry(r guestbookdb.GuestbookEntry) Entry {
	return Entry{ID: r.ID, WeddingID: r.WeddingID, GuestID: r.GuestID, Name: r.GuestName, Message: r.Message, Hidden: r.IsHidden, Favorite: r.IsFavorite, CreatedAt: r.CreatedAt}
}

// Post menyimpan ucapan baru. Pesan yang mengandung kata kasar tetap disimpan
// tetapi otomatis disembunyikan (pasangan bisa menampilkannya di dashboard).
func (s *Service) Post(ctx context.Context, weddingID uuid.UUID, guestID *uuid.UUID, name, message string) (Entry, error) {
	name, message = strings.TrimSpace(name), strings.TrimSpace(message)
	v := ValidationError{}
	switch n := utf8.RuneCountInString(name); {
	case n == 0:
		v["name"] = "Nama wajib diisi"
	case n > MaxName:
		v["name"] = fmt.Sprintf("Nama maksimal %d karakter", MaxName)
	}
	switch n := utf8.RuneCountInString(message); {
	case n == 0:
		v["message"] = "Ucapan wajib diisi"
	case n > MaxMessage:
		v["message"] = fmt.Sprintf("Ucapan maksimal %d karakter", MaxMessage)
	}
	if len(v) > 0 {
		return Entry{}, v
	}
	hidden := s.filter.Match(name) || s.filter.Match(message)
	row, err := s.q.CreateEntry(ctx, guestbookdb.CreateEntryParams{
		ID: db.NewID(), WeddingID: weddingID, GuestID: guestID, GuestName: name, Message: message, IsHidden: hidden,
	})
	if err != nil {
		return Entry{}, fmt.Errorf("guestbook: post: %w", err)
	}
	return toEntry(row), nil
}

// Visible mengembalikan pesan yang tampil, terbaru dulu, sesudah entri before
// (uuid.Nil = dari awal). more = masih ada pesan berikutnya.
func (s *Service) Visible(ctx context.Context, weddingID, before uuid.UUID, limit int) (entries []Entry, more bool, err error) {
	var b *uuid.UUID
	if before != uuid.Nil {
		b = &before
	}
	rows, err := s.q.ListVisible(ctx, guestbookdb.ListVisibleParams{WeddingID: weddingID, BeforeID: b, Lim: int32(limit + 1)}) //nolint:gosec // G115: limit kecil
	if err != nil {
		return nil, false, err
	}
	if len(rows) > limit {
		rows, more = rows[:limit], true
	}
	entries = make([]Entry, len(rows))
	for i, r := range rows {
		entries[i] = toEntry(r)
	}
	return entries, more, nil
}

// List: semua pesan (dashboard), filter hidden opsional, halaman mulai 1.
func (s *Service) List(ctx context.Context, weddingID uuid.UUID, hidden *bool, page int) ([]Entry, error) {
	page = max(1, page)
	rows, err := s.q.ListEntries(ctx, guestbookdb.ListEntriesParams{
		WeddingID: weddingID, Hidden: hidden, Lim: DashboardPage, Off: int32((page - 1) * DashboardPage), //nolint:gosec // G115: halaman kecil
	})
	if err != nil {
		return nil, err
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = toEntry(r)
	}
	return out, nil
}

func (s *Service) Stats(ctx context.Context, weddingID uuid.UUID) (Stats, error) {
	r, err := s.q.EntryStats(ctx, weddingID)
	return Stats{Total: int(r.Total), Hidden: int(r.Hidden)}, err
}

func (s *Service) SetHidden(ctx context.Context, weddingID, id uuid.UUID, hidden bool) (Entry, error) {
	row, err := s.q.SetHidden(ctx, guestbookdb.SetHiddenParams{ID: id, WeddingID: weddingID, IsHidden: hidden})
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	if err != nil {
		return Entry{}, err
	}
	s.changed(weddingID)
	return toEntry(row), nil
}

func (s *Service) Delete(ctx context.Context, weddingID, id uuid.UUID) error {
	n, err := s.q.DeleteEntry(ctx, guestbookdb.DeleteEntryParams{ID: id, WeddingID: weddingID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	s.changed(weddingID)
	return nil
}

// Recent: n pesan terbaru (termasuk yang disembunyikan) untuk beranda dashboard.
func (s *Service) Recent(ctx context.Context, weddingID uuid.UUID, n int) ([]Entry, error) {
	rows, err := s.q.ListEntries(ctx, guestbookdb.ListEntriesParams{WeddingID: weddingID, Lim: int32(n)}) //nolint:gosec // G115: n kecil
	if err != nil {
		return nil, err
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = toEntry(r)
	}
	return out, nil
}

// SetFavorite menandai / melepas ucapan favorit (T19).
func (s *Service) SetFavorite(ctx context.Context, weddingID, id uuid.UUID, favorite bool) (Entry, error) {
	row, err := s.q.SetFavorite(ctx, guestbookdb.SetFavoriteParams{ID: id, WeddingID: weddingID, IsFavorite: favorite})
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	if err != nil {
		return Entry{}, err
	}
	s.changed(weddingID)
	return toEntry(row), nil
}

// Favorites: ucapan favorit yang tampil (tidak disembunyikan), terbaru dulu.
func (s *Service) Favorites(ctx context.Context, weddingID uuid.UUID, n int) ([]Entry, error) {
	rows, err := s.q.ListFavorites(ctx, guestbookdb.ListFavoritesParams{WeddingID: weddingID, Lim: int32(n)}) //nolint:gosec // G115: n kecil
	if err != nil {
		return nil, err
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		out[i] = toEntry(r)
	}
	return out, nil
}
