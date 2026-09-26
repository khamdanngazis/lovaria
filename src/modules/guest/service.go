// Package guest: daftar tamu, kode undangan personal, import/export CSV, dan
// data RSVP. Operasi dashboard menerima weddingID yang sudah diotorisasi
// (RequireWeddingOwner); GetByCode dipakai resolver public site (T09).
package guest

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	guestdb "github.com/khamdanngazis/lovaria/src/modules/guest/db"
)

const (
	StatusPending   = "pending"
	StatusAttending = "attending"
	StatusDeclined  = "declined"

	MaxPax  = 20
	PerPage = 25
)

// Statuses berisi status RSVP beserta labelnya.
var Statuses = []struct{ ID, Label string }{
	{StatusPending, "Belum konfirmasi"},
	{StatusAttending, "Hadir"},
	{StatusDeclined, "Tidak hadir"},
}

func StatusLabel(s string) string {
	for _, x := range Statuses {
		if x.ID == s {
			return x.Label
		}
	}
	return s
}

var ErrNotFound = errors.New("tamu tidak ditemukan")

// ValidationError memetakan nama field ke pesan error.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

type Guest struct {
	ID             uuid.UUID
	WeddingID      uuid.UUID
	Name           string
	Phone          string // 62xxxxxxxxxx atau ""
	Email          string
	GroupName      string
	MaxPax         int
	InvitationCode string
	RSVPStatus     string
	RSVPPax        int
	RSVPMessage    string
	RSVPAt         *time.Time
	Notes          string
	LastOpenedAt   *time.Time
	CreatedAt      time.Time
}

// Input adalah nilai mentah form/CSV.
type Input struct {
	Name      string
	Phone     string
	Email     string
	GroupName string
	MaxPax    string // kosong → 1
	Notes     string
}

// Filter untuk daftar tamu.
type Filter struct {
	Q      string
	Status string
	Group  string
	Page   int // mulai 1
}

type Page struct {
	Guests  []Guest
	Total   int
	Page    int
	PerPage int
}

func (p Page) Pages() int { return max(1, (p.Total+p.PerPage-1)/p.PerPage) }

// Stats adalah ringkasan tamu & pax (dipakai dashboard, T13).
type Stats struct {
	Total, Attending, Declined, Pending int
	PaxInvited, PaxAttending            int
	Opened                              int
}

type Service struct {
	repo    *Repository
	baseURL string
	now     func() time.Time
}

func NewService(repo *Repository, baseURL string) *Service {
	return &Service{repo: repo, baseURL: strings.TrimRight(baseURL, "/"), now: time.Now}
}

// InvitationURL adalah link undangan personal tamu.
func (s *Service) InvitationURL(code string) string { return s.baseURL + "/i/" + code }

// ---------- Validasi ----------

type parsed struct {
	name, phone, group, notes string
	email                     *string
	maxPax                    int16
}

func validate(in Input) (parsed, ValidationError) {
	v := ValidationError{}
	p := parsed{
		name:  strings.TrimSpace(in.Name),
		group: strings.TrimSpace(in.GroupName),
		notes: strings.TrimSpace(in.Notes),
	}
	switch n := utf8.RuneCountInString(p.name); {
	case n == 0:
		v["name"] = "Nama wajib diisi"
	case n > 100:
		v["name"] = "Nama maksimal 100 karakter"
	}
	if utf8.RuneCountInString(p.group) > 50 {
		v["group_name"] = "Grup maksimal 50 karakter"
	}
	if utf8.RuneCountInString(p.notes) > 500 {
		v["notes"] = "Catatan maksimal 500 karakter"
	}
	phone, err := NormalizePhone(in.Phone)
	if err != nil {
		v["phone"] = phoneMessage
	}
	p.phone = phone
	if e := strings.ToLower(strings.TrimSpace(in.Email)); e != "" {
		addr, err := mail.ParseAddress(e)
		if err != nil || addr.Address != e || len(e) > 254 {
			v["email"] = "Format email tidak valid"
		}
		p.email = &e
	}
	p.maxPax = 1
	if m := strings.TrimSpace(in.MaxPax); m != "" {
		n, err := strconv.Atoi(m)
		if err != nil || n < 1 || n > MaxPax {
			v["max_pax"] = fmt.Sprintf("Jumlah orang 1–%d", MaxPax)
		}
		p.maxPax = int16(max(1, min(n, MaxPax))) //nolint:gosec // G115: dibatasi 1..20
	}
	if len(v) > 0 {
		return parsed{}, v
	}
	return p, nil
}

// ---------- Tulis ----------

func isCodeConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "guests_invitation_code_key"
}

// Create menambah tamu dengan kode undangan baru (retry bila kode bentrok).
func (s *Service) Create(ctx context.Context, weddingID uuid.UUID, in Input) (Guest, error) {
	p, v := validate(in)
	if v != nil {
		return Guest{}, v
	}
	for attempt := 0; attempt < 5; attempt++ {
		code, err := NewCode()
		if err != nil {
			return Guest{}, err
		}
		row, err := s.repo.q.CreateGuest(ctx, guestdb.CreateGuestParams{
			ID: db.NewID(), WeddingID: weddingID, Name: p.name, Phone: p.phone, Email: p.email,
			GroupName: p.group, MaxPax: p.maxPax, InvitationCode: code, Notes: p.notes,
		})
		if isCodeConflict(err) {
			continue
		}
		if err != nil {
			return Guest{}, fmt.Errorf("guest: create: %w", err)
		}
		return toGuest(row), nil
	}
	return Guest{}, errors.New("guest: gagal membuat kode undangan unik")
}

func (s *Service) Update(ctx context.Context, weddingID, id uuid.UUID, in Input) (Guest, error) {
	p, v := validate(in)
	if v != nil {
		return Guest{}, v
	}
	row, err := s.repo.q.UpdateGuest(ctx, guestdb.UpdateGuestParams{
		ID: id, WeddingID: weddingID, Name: p.name, Phone: p.phone, Email: p.email,
		GroupName: p.group, MaxPax: p.maxPax, Notes: p.notes,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	if err != nil {
		return Guest{}, err
	}
	return toGuest(row), nil
}

func (s *Service) Delete(ctx context.Context, weddingID, id uuid.UUID) error {
	n, err := s.repo.q.DeleteGuest(ctx, guestdb.DeleteGuestParams{ID: id, WeddingID: weddingID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// BulkDelete menghapus beberapa tamu sekaligus; ID milik wedding lain diabaikan.
func (s *Service) BulkDelete(ctx context.Context, weddingID uuid.UUID, ids []uuid.UUID) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	n, err := s.repo.q.DeleteGuests(ctx, guestdb.DeleteGuestsParams{WeddingID: weddingID, Ids: ids})
	return int(n), err
}

// UpdateRSVP menyimpan jawaban RSVP tamu (dipakai T10). Pax disesuaikan: 0 bila
// tidak hadir, 1..max_pax bila hadir.
func (s *Service) UpdateRSVP(ctx context.Context, weddingID, id uuid.UUID, status string, pax int, message string) (Guest, error) {
	g, err := s.Get(ctx, weddingID, id)
	if err != nil {
		return Guest{}, err
	}
	v := ValidationError{}
	switch status {
	case StatusAttending:
		if pax < 1 || pax > g.MaxPax {
			v["pax"] = fmt.Sprintf("Jumlah orang 1–%d", g.MaxPax)
		}
	case StatusDeclined:
		pax = 0
	default:
		v["status"] = "Pilih hadir atau tidak hadir"
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > 500 {
		v["message"] = "Pesan maksimal 500 karakter"
	}
	if len(v) > 0 {
		return Guest{}, v
	}
	now := s.now()
	row, err := s.repo.q.UpdateRSVP(ctx, guestdb.UpdateRSVPParams{
		ID: id, WeddingID: weddingID, RsvpStatus: status, RsvpPax: int16(pax), //nolint:gosec // G115: ≤ max_pax (20)
		RsvpMessage: message, RsvpAt: &now,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	if err != nil {
		return Guest{}, err
	}
	return toGuest(row), nil
}

// MarkOpened mencatat waktu tamu terakhir membuka undangan (dipakai T09).
func (s *Service) MarkOpened(ctx context.Context, weddingID, id uuid.UUID) error {
	now := s.now()
	return s.repo.q.MarkOpened(ctx, guestdb.MarkOpenedParams{ID: id, WeddingID: weddingID, LastOpenedAt: &now})
}

// ---------- Baca ----------

func (s *Service) Get(ctx context.Context, weddingID, id uuid.UUID) (Guest, error) {
	row, err := s.repo.q.GetGuest(ctx, guestdb.GetGuestParams{ID: id, WeddingID: weddingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	if err != nil {
		return Guest{}, err
	}
	return toGuest(row), nil
}

// GetByCode mencari tamu lintas wedding berdasarkan kode undangan (resolver T09).
func (s *Service) GetByCode(ctx context.Context, code string) (Guest, error) {
	code = NormalizeCode(code)
	if !ValidCode(code) {
		return Guest{}, ErrNotFound
	}
	row, err := s.repo.q.GetGuestByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	if err != nil {
		return Guest{}, err
	}
	return toGuest(row), nil
}

func optional(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// phoneQuery: potongan nomor "0812-34" / "+62 812" dicocokkan dengan format
// tersimpan "62812…". Teks biasa dikembalikan apa adanya.
func phoneQuery(q string) string {
	digits := strings.NewReplacer(" ", "", "-", "", ".", "").Replace(q)
	digits = strings.TrimPrefix(digits, "+")
	for _, r := range digits {
		if r < '0' || r > '9' {
			return q
		}
	}
	if strings.HasPrefix(digits, "0") {
		return "62" + digits[1:]
	}
	return digits
}

// escapeLike meloloskan wildcard LIKE supaya input pencarian diperlakukan literal.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// List mengembalikan satu halaman tamu sesuai filter.
func (s *Service) List(ctx context.Context, weddingID uuid.UUID, f Filter) (Page, error) {
	var status *string
	if StatusLabel(f.Status) != f.Status {
		status = &f.Status
	}
	var q *string
	if t := strings.TrimSpace(f.Q); t != "" {
		t = escapeLike(phoneQuery(t))
		q = &t
	}
	group := optional(f.Group)
	page := max(1, f.Page)

	total, err := s.repo.q.CountGuests(ctx, guestdb.CountGuestsParams{WeddingID: weddingID, Status: status, GroupName: group, Q: q})
	if err != nil {
		return Page{}, err
	}
	rows, err := s.repo.q.ListGuests(ctx, guestdb.ListGuestsParams{
		WeddingID: weddingID, Status: status, GroupName: group, Q: q,
		Lim: PerPage, Off: int32((page - 1) * PerPage), //nolint:gosec // G115: halaman kecil
	})
	if err != nil {
		return Page{}, err
	}
	out := Page{Guests: make([]Guest, len(rows)), Total: int(total), Page: page, PerPage: PerPage}
	for i, r := range rows {
		out.Guests[i] = toGuest(r)
	}
	return out, nil
}

// All mengembalikan semua tamu (untuk export).
func (s *Service) All(ctx context.Context, weddingID uuid.UUID) ([]Guest, error) {
	rows, err := s.repo.q.ListAllGuests(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	out := make([]Guest, len(rows))
	for i, r := range rows {
		out[i] = toGuest(r)
	}
	return out, nil
}

func (s *Service) Groups(ctx context.Context, weddingID uuid.UUID) ([]string, error) {
	return s.repo.q.ListGroups(ctx, weddingID)
}

// Stats menghitung ringkasan tamu & pax wedding.
func (s *Service) Stats(ctx context.Context, weddingID uuid.UUID) (Stats, error) {
	r, err := s.repo.q.GuestStats(ctx, weddingID)
	if err != nil {
		return Stats{}, err
	}
	return Stats{
		Total: int(r.Total), Attending: int(r.Attending), Declined: int(r.Declined), Pending: int(r.Pending),
		PaxInvited: int(r.PaxInvited), PaxAttending: int(r.PaxAttending), Opened: int(r.Opened),
	}, nil
}
