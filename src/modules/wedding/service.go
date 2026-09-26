// Package wedding: entitas pusat platform — wedding, info pasangan, setup wizard,
// dan otorisasi kepemilikan (RequireWeddingOwner) untuk semua modul dashboard.
package wedding

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	weddingdb "github.com/khamdanngazis/lovaria/src/modules/wedding/db"
)

const (
	StatusDraft      = "draft"
	StatusPublished  = "published"
	StatusWeddingDay = "wedding_day"
	StatusMemory     = "memory"
	StatusArchived   = "archived"

	DefaultThemeID  = "elegant"
	DefaultTimezone = "Asia/Jakarta"

	dateLayout = "2006-01-02"
)

var (
	// ErrNotFound: wedding tidak ada ATAU bukan milik user (sengaja tidak dibedakan).
	ErrNotFound = errors.New("wedding tidak ditemukan")
	// ErrQuotaExceeded: upload akan melewati kuota penyimpanan wedding.
	ErrQuotaExceeded = errors.New("kuota penyimpanan wedding sudah penuh")
)

// ValidationError memetakan nama field form ke pesan error.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

type Wedding struct {
	ID           uuid.UUID
	OwnerUserID  uuid.UUID
	Slug         string
	Title        string
	WeddingDate  time.Time
	Description  string
	MainPhotoURL *string
	Status       string
	ThemeID      string
	// Timezone zona waktu IANA acara (jam event disimpan sebagai waktu lokal).
	Timezone string
	// StorageUsedBytes total byte foto wedding di storage (lihat ReserveStorage).
	StorageUsedBytes int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// DashboardURL mengembalikan URL dashboard wedding ini + suffix (mis. "/events").
func (w Wedding) DashboardURL(suffix string) string {
	return "/dashboard/weddings/" + w.ID.String() + suffix
}

// Timezones adalah zona waktu yang bisa dipilih, dengan singkatan tampilannya.
var Timezones = []struct{ ID, Label, Abbr string }{
	{"Asia/Jakarta", "WIB — Waktu Indonesia Barat", "WIB"},
	{"Asia/Makassar", "WITA — Waktu Indonesia Tengah", "WITA"},
	{"Asia/Jayapura", "WIT — Waktu Indonesia Timur", "WIT"},
}

// TimezoneAbbr mengembalikan singkatan zona waktu ("WIB"), atau ID-nya bila tidak dikenal.
func TimezoneAbbr(id string) string {
	for _, tz := range Timezones {
		if tz.ID == id {
			return tz.Abbr
		}
	}
	return id
}

type Couple struct {
	ID               uuid.UUID
	WeddingID        uuid.UUID
	GroomName        string
	BrideName        string
	GroomPhotoURL    *string
	BridePhotoURL    *string
	GroomDescription string
	BrideDescription string
}

// Reader adalah API baca-saja untuk modul lain (public site, guest, theme, ...).
type Reader interface {
	GetWedding(ctx context.Context, id uuid.UUID) (Wedding, error)
	GetWeddingBySlug(ctx context.Context, slug string) (Wedding, error)
	GetCouple(ctx context.Context, weddingID uuid.UUID) (Couple, error)
}

var _ Reader = (*Service)(nil)

// CreateInput adalah isi lengkap setup wizard. Nilai mentah dari form.
type CreateInput struct {
	GroomName   string
	BrideName   string
	Title       string
	WeddingDate string // YYYY-MM-DD
	Description string
}

func (in CreateInput) fields() map[string]string {
	return map[string]string{
		"groom_name": in.GroomName, "bride_name": in.BrideName,
		"title": in.Title, "wedding_date": in.WeddingDate, "description": in.Description,
	}
}

type InfoInput struct {
	Title        string
	WeddingDate  string
	Description  string
	MainPhotoURL string
	Timezone     string // kosong → DefaultTimezone
}

type CoupleInput struct {
	GroomName        string
	BrideName        string
	GroomPhotoURL    string
	BridePhotoURL    string
	GroomDescription string
	BrideDescription string
}

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ---------- Validasi ----------

// ValidateField memvalidasi satu field form wedding/couple ("" bila valid).
func ValidateField(field, value string) string {
	v := strings.TrimSpace(value)
	switch field {
	case "groom_name", "bride_name":
		return text(v, true, 100, map[string]string{"groom_name": "Nama mempelai pria", "bride_name": "Nama mempelai wanita"}[field])
	case "title":
		return text(v, true, 150, "Judul")
	case "description":
		return text(v, false, 2000, "Deskripsi")
	case "groom_description", "bride_description":
		return text(v, false, 1000, "Deskripsi")
	case "wedding_date":
		if v == "" {
			return "Tanggal pernikahan wajib diisi"
		}
		d, err := time.Parse(dateLayout, v)
		if err != nil {
			return "Tanggal tidak valid"
		}
		if d.Year() < 2000 || d.Year() > 2100 {
			return "Tanggal harus antara tahun 2000 dan 2100"
		}
	case "timezone":
		if v == "" {
			return ""
		}
		for _, tz := range Timezones {
			if tz.ID == v {
				return ""
			}
		}
		return "Zona waktu tidak dikenal"
	case "main_photo_url", "groom_photo_url", "bride_photo_url":
		if v == "" {
			return ""
		}
		u, err := url.Parse(v)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || len(v) > 2048 {
			return "URL foto tidak valid (harus diawali https://)"
		}
	}
	return ""
}

func text(v string, required bool, maxLen int, label string) string {
	n := utf8.RuneCountInString(v)
	switch {
	case required && n == 0:
		return label + " wajib diisi"
	case n > maxLen:
		return fmt.Sprintf("%s maksimal %d karakter", label, maxLen)
	}
	return ""
}

// ValidateFields memvalidasi sekumpulan field; nil bila semua valid.
func ValidateFields(fields map[string]string) error {
	v := ValidationError{}
	for f, val := range fields {
		if msg := ValidateField(f, val); msg != "" {
			v[f] = msg
		}
	}
	if len(v) > 0 {
		return v
	}
	return nil
}

func optional(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// ---------- Tulis ----------

// CreateWedding membuat wedding (status draft, tema default) beserta data pasangan.
// Slug dibentuk dari nama pasangan, diberi suffix -2, -3, ... bila bentrok.
func (s *Service) CreateWedding(ctx context.Context, ownerID uuid.UUID, in CreateInput) (Wedding, error) {
	if err := ValidateFields(in.fields()); err != nil {
		return Wedding{}, err
	}
	date, _ := time.Parse(dateLayout, strings.TrimSpace(in.WeddingDate))
	base := baseSlug(in.GroomName, in.BrideName)

	var w weddingdb.Wedding
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = s.repo.inTx(ctx, func(q *weddingdb.Queries) error {
			taken, err := q.ListSlugsWithPrefix(ctx, base)
			if err != nil {
				return err
			}
			w, err = q.CreateWedding(ctx, weddingdb.CreateWeddingParams{
				ID:          db.NewID(),
				OwnerUserID: ownerID,
				Slug:        nextFreeSlug(base, taken),
				Title:       strings.TrimSpace(in.Title),
				WeddingDate: date,
				Description: strings.TrimSpace(in.Description),
			})
			if err != nil {
				return mapErr(err)
			}
			_, err = q.CreateCouple(ctx, weddingdb.CreateCoupleParams{
				ID:        db.NewID(),
				WeddingID: w.ID,
				GroomName: strings.TrimSpace(in.GroomName),
				BrideName: strings.TrimSpace(in.BrideName),
			})
			return err
		})
		if !errors.Is(err, errSlugTaken) { // bentrok karena race → coba lagi
			break
		}
	}
	if err != nil {
		return Wedding{}, fmt.Errorf("wedding: create: %w", err)
	}
	return toWedding(w), nil
}

// UpdateWeddingInfo mengubah judul, tanggal, deskripsi, dan foto utama.
// Pemanggil wajib sudah memastikan kepemilikan (RequireWeddingOwner).
func (s *Service) UpdateWeddingInfo(ctx context.Context, weddingID uuid.UUID, in InfoInput) (Wedding, error) {
	if err := ValidateFields(map[string]string{
		"title": in.Title, "wedding_date": in.WeddingDate, "description": in.Description,
		"main_photo_url": in.MainPhotoURL, "timezone": in.Timezone,
	}); err != nil {
		return Wedding{}, err
	}
	tz := strings.TrimSpace(in.Timezone)
	if tz == "" {
		tz = DefaultTimezone
	}
	date, _ := time.Parse(dateLayout, strings.TrimSpace(in.WeddingDate))
	w, err := s.repo.q.UpdateWeddingInfo(ctx, weddingdb.UpdateWeddingInfoParams{
		ID:           weddingID,
		Title:        strings.TrimSpace(in.Title),
		WeddingDate:  date,
		Description:  strings.TrimSpace(in.Description),
		MainPhotoUrl: optional(in.MainPhotoURL),
		Timezone:     tz,
	})
	if err != nil {
		return Wedding{}, mapErr(err)
	}
	return toWedding(w), nil
}

// UpdateCouple mengubah data mempelai.
func (s *Service) UpdateCouple(ctx context.Context, weddingID uuid.UUID, in CoupleInput) (Couple, error) {
	if err := ValidateFields(map[string]string{
		"groom_name": in.GroomName, "bride_name": in.BrideName,
		"groom_photo_url": in.GroomPhotoURL, "bride_photo_url": in.BridePhotoURL,
		"groom_description": in.GroomDescription, "bride_description": in.BrideDescription,
	}); err != nil {
		return Couple{}, err
	}
	c, err := s.repo.q.UpdateCouple(ctx, weddingdb.UpdateCoupleParams{
		WeddingID:        weddingID,
		GroomName:        strings.TrimSpace(in.GroomName),
		BrideName:        strings.TrimSpace(in.BrideName),
		GroomPhotoUrl:    optional(in.GroomPhotoURL),
		BridePhotoUrl:    optional(in.BridePhotoURL),
		GroomDescription: strings.TrimSpace(in.GroomDescription),
		BrideDescription: strings.TrimSpace(in.BrideDescription),
	})
	if err != nil {
		return Couple{}, mapErr(err)
	}
	return toCouple(c), nil
}

// ---------- Kuota penyimpanan & foto utama (dipakai modul gallery) ----------

// ReserveStorage menambah pemakaian storage secara atomik; ErrQuotaExceeded bila
// pemakaian baru melewati quota. Panggil ReleaseStorage bila upload batal.
func (s *Service) ReserveStorage(ctx context.Context, weddingID uuid.UUID, bytes, quota int64) error {
	_, err := s.repo.q.ReserveStorage(ctx, weddingdb.ReserveStorageParams{ID: weddingID, Bytes: bytes, Quota: quota})
	if errors.Is(mapErr(err), ErrNotFound) {
		// Tidak ada baris yang lolos syarat: wedding tidak ada atau kuota terlampaui.
		if _, gerr := s.GetWedding(ctx, weddingID); gerr != nil {
			return gerr
		}
		return ErrQuotaExceeded
	}
	return err
}

// ReleaseStorage mengurangi pemakaian storage (tidak pernah di bawah 0).
func (s *Service) ReleaseStorage(ctx context.Context, weddingID uuid.UUID, bytes int64) error {
	return s.repo.q.ReleaseStorage(ctx, weddingdb.ReleaseStorageParams{ID: weddingID, Bytes: bytes})
}

// StorageUsage mengembalikan byte yang terpakai wedding.
func (s *Service) StorageUsage(ctx context.Context, weddingID uuid.UUID) (int64, error) {
	w, err := s.GetWedding(ctx, weddingID)
	if err != nil {
		return 0, err
	}
	return w.StorageUsedBytes, nil
}

// SetMainPhotoURL menjadikan URL (mis. foto gallery) sebagai foto utama wedding.
func (s *Service) SetMainPhotoURL(ctx context.Context, weddingID uuid.UUID, url string) error {
	return s.repo.q.SetMainPhotoURL(ctx, weddingdb.SetMainPhotoURLParams{ID: weddingID, MainPhotoUrl: optional(url)})
}

// ---------- Baca ----------

// GetWedding mengambil wedding tanpa cek kepemilikan (untuk modul lain / public site).
func (s *Service) GetWedding(ctx context.Context, id uuid.UUID) (Wedding, error) {
	w, err := s.repo.q.GetWedding(ctx, id)
	if err != nil {
		return Wedding{}, mapErr(err)
	}
	return toWedding(w), nil
}

// GetWeddingForOwner mengambil wedding hanya bila dimiliki ownerID; selain itu ErrNotFound.
func (s *Service) GetWeddingForOwner(ctx context.Context, ownerID, id uuid.UUID) (Wedding, error) {
	w, err := s.repo.q.GetWeddingForOwner(ctx, weddingdb.GetWeddingForOwnerParams{ID: id, OwnerUserID: ownerID})
	if err != nil {
		return Wedding{}, mapErr(err)
	}
	return toWedding(w), nil
}

func (s *Service) GetWeddingBySlug(ctx context.Context, slug string) (Wedding, error) {
	w, err := s.repo.q.GetWeddingBySlug(ctx, strings.ToLower(strings.TrimSpace(slug)))
	if err != nil {
		return Wedding{}, mapErr(err)
	}
	return toWedding(w), nil
}

func (s *Service) GetCouple(ctx context.Context, weddingID uuid.UUID) (Couple, error) {
	c, err := s.repo.q.GetCouple(ctx, weddingID)
	if err != nil {
		return Couple{}, mapErr(err)
	}
	return toCouple(c), nil
}

func (s *Service) ListWeddingsByOwner(ctx context.Context, ownerID uuid.UUID) ([]Wedding, error) {
	rows, err := s.repo.q.ListWeddingsByOwner(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]Wedding, len(rows))
	for i, r := range rows {
		out[i] = toWedding(r)
	}
	return out, nil
}
