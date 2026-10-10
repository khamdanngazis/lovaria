package guest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	guestdb "github.com/khamdanngazis/lovaria/src/modules/guest/db"
)

// Check-in tamu dengan QR di hari H (T31): link penerima tamu, pencatatan
// kehadiran, dan tamu tambahan (tanpa undangan).

// Cara check-in dicatat.
const (
	ViaScan   = "scan"   // QR dipindai penerima tamu
	ViaManual = "manual" // dicari manual oleh penerima tamu
	ViaOwner  = "owner"  // ditandai pasangan dari dashboard
)

var (
	// ErrCheckinLink: token link penerima tamu salah, kedaluwarsa, atau dicabut.
	ErrCheckinLink = errors.New("link penerima tamu tidak berlaku")
	// ErrAlreadyCheckedIn: undangan ini sudah dipakai check-in.
	ErrAlreadyCheckedIn = errors.New("tamu sudah check-in")
)

// SetCheckinSecret memasang kunci HMAC (APP_SECRET) penanda tangan token link
// penerima tamu. Tanpa kunci, link tidak bisa dibuat maupun dipakai.
func (s *Service) SetCheckinSecret(secret []byte) { s.checkinSecret = secret }

// checkinToken: id link + tanda tangan HMAC — tidak ada rahasia yang disimpan
// di database, dan link yang sama bisa ditampilkan lagi ke pasangan kapan saja.
func (s *Service) checkinToken(linkID uuid.UUID) string {
	mac := hmac.New(sha256.New, s.checkinSecret)
	mac.Write([]byte("checkin:" + linkID.String()))
	return base64.RawURLEncoding.EncodeToString(linkID[:]) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:18])
}

// CheckinLink: link penerima tamu yang aktif untuk sebuah wedding.
type CheckinLink struct {
	URL       string
	CreatedAt time.Time
}

func (s *Service) linkURL(id uuid.UUID) string { return s.baseURL + "/checkin/" + s.checkinToken(id) }

// ActiveCheckinLink mengembalikan link aktif; ok=false bila belum dibuat.
func (s *Service) ActiveCheckinLink(ctx context.Context, weddingID uuid.UUID) (CheckinLink, bool, error) {
	if len(s.checkinSecret) == 0 {
		return CheckinLink{}, false, nil
	}
	row, err := s.repo.q.GetActiveCheckinLink(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return CheckinLink{}, false, nil
	}
	if err != nil {
		return CheckinLink{}, false, fmt.Errorf("guest: link check-in: %w", err)
	}
	return CheckinLink{URL: s.linkURL(row.ID), CreatedAt: row.CreatedAt}, true, nil
}

// NewCheckinLink membuat link baru dan mencabut yang lama (link lama langsung
// tidak berlaku).
func (s *Service) NewCheckinLink(ctx context.Context, weddingID uuid.UUID) (CheckinLink, error) {
	if len(s.checkinSecret) == 0 {
		return CheckinLink{}, errors.New("guest: kunci link check-in belum dipasang")
	}
	now := s.now()
	var row guestdb.CheckinLink
	err := s.repo.inTx(ctx, func(q *guestdb.Queries) error {
		if err := q.RevokeCheckinLinks(ctx, guestdb.RevokeCheckinLinksParams{WeddingID: weddingID, RevokedAt: &now}); err != nil {
			return err
		}
		var err error
		row, err = q.CreateCheckinLink(ctx, guestdb.CreateCheckinLinkParams{ID: db.NewID(), WeddingID: weddingID, CreatedAt: now})
		return err
	})
	if err != nil {
		return CheckinLink{}, fmt.Errorf("guest: buat link check-in: %w", err)
	}
	return CheckinLink{URL: s.linkURL(row.ID), CreatedAt: row.CreatedAt}, nil
}

// RevokeCheckinLink mencabut link penerima tamu.
func (s *Service) RevokeCheckinLink(ctx context.Context, weddingID uuid.UUID) error {
	now := s.now()
	return s.repo.q.RevokeCheckinLinks(ctx, guestdb.RevokeCheckinLinksParams{WeddingID: weddingID, RevokedAt: &now})
}

// WeddingByCheckinToken memeriksa token link penerima tamu dan mengembalikan
// wedding yang dilayaninya. Token salah / dicabut → ErrCheckinLink.
func (s *Service) WeddingByCheckinToken(ctx context.Context, token string) (uuid.UUID, error) {
	idPart, _, ok := strings.Cut(token, ".")
	raw, err := base64.RawURLEncoding.DecodeString(idPart)
	if !ok || err != nil || len(raw) != 16 || len(s.checkinSecret) == 0 {
		return uuid.Nil, ErrCheckinLink
	}
	id, _ := uuid.FromBytes(raw)
	if !hmac.Equal([]byte(token), []byte(s.checkinToken(id))) {
		return uuid.Nil, ErrCheckinLink
	}
	row, err := s.repo.q.GetCheckinLinkByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrCheckinLink
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("guest: cari link check-in: %w", err)
	}
	return row.WeddingID, nil
}

// codeInText: kode undangan di dalam isi QR — alamat ".../i/<KODE>" (domain
// Lunovia maupun custom domain) atau kode itu sendiri.
var codeInText = regexp.MustCompile(`(?i)(?:^|/i/)([2-9A-HJKMNP-Z]{7})(?:[/?#]|$)`)

// ParseScannedCode mengambil kode undangan dari hasil pindai QR; "" bila
// bukan QR undangan.
func ParseScannedCode(text string) string {
	m := codeInText.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return ""
	}
	return NormalizeCode(m[1])
}

// GuestForCheckin mencari tamu wedding ini dari hasil pindai / kode. Tamu
// wedding lain tidak pernah ditemukan (ErrNotFound).
func (s *Service) GuestForCheckin(ctx context.Context, weddingID uuid.UUID, scanned string) (Guest, error) {
	code := ParseScannedCode(scanned)
	if code == "" {
		return Guest{}, ErrNotFound
	}
	row, err := s.repo.q.GetGuestByCodeInWedding(ctx, guestdb.GetGuestByCodeInWeddingParams{WeddingID: weddingID, InvitationCode: code})
	if errors.Is(err, pgx.ErrNoRows) {
		return Guest{}, ErrNotFound
	}
	if err != nil {
		return Guest{}, fmt.Errorf("guest: cari tamu check-in: %w", err)
	}
	return toGuest(row), nil
}

// SuggestedPax: jumlah orang yang ditawarkan saat check-in — jawaban RSVP bila
// tamu konfirmasi hadir, selain itu batas undangan.
func (g Guest) SuggestedPax() int {
	if g.RSVPStatus == StatusAttending && g.RSVPPax > 0 {
		return g.RSVPPax
	}
	return max(g.MaxPax, 1)
}

// CheckIn mencatat kedatangan tamu. pax dibatasi 1..MaxPax. Bila undangan itu
// sudah dipakai, mengembalikan tamu apa adanya + ErrAlreadyCheckedIn — dua
// pemindaian bersamaan menghasilkan tepat satu check-in.
func (s *Service) CheckIn(ctx context.Context, weddingID, guestID uuid.UUID, pax int, via string) (Guest, error) {
	g, err := s.Get(ctx, weddingID, guestID)
	if err != nil {
		return Guest{}, err
	}
	if via != ViaScan && via != ViaManual && via != ViaOwner {
		via = ViaManual
	}
	pax = min(max(pax, 1), max(g.MaxPax, 1))
	now, p := s.now(), int16(pax) //nolint:gosec // G115: 1..20
	row, err := s.repo.q.CheckInGuest(ctx, guestdb.CheckInGuestParams{ID: guestID, WeddingID: weddingID, CheckedInAt: &now, CheckedInPax: &p, CheckedInVia: &via})
	if errors.Is(err, pgx.ErrNoRows) {
		cur, gerr := s.Get(ctx, weddingID, guestID)
		if gerr != nil {
			return Guest{}, gerr
		}
		return cur, ErrAlreadyCheckedIn
	}
	if err != nil {
		return Guest{}, fmt.Errorf("guest: check-in: %w", err)
	}
	return toGuest(row), nil
}

// UndoCheckIn membatalkan check-in (salah pindai).
func (s *Service) UndoCheckIn(ctx context.Context, weddingID, guestID uuid.UUID) error {
	n, err := s.repo.q.UndoCheckIn(ctx, guestdb.UndoCheckInParams{ID: guestID, WeddingID: weddingID})
	if err != nil {
		return fmt.Errorf("guest: batal check-in: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchForCheckin: cari manual berdasarkan nama atau kode (maks. 8 hasil).
func (s *Service) SearchForCheckin(ctx context.Context, weddingID uuid.UUID, q string) ([]Guest, error) {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) < 2 {
		return nil, nil
	}
	rows, err := s.repo.q.SearchGuestsForCheckin(ctx, guestdb.SearchGuestsForCheckinParams{WeddingID: weddingID, Q: q})
	if err != nil {
		return nil, fmt.Errorf("guest: cari check-in: %w", err)
	}
	out := make([]Guest, len(rows))
	for i, r := range rows {
		out[i] = toGuest(r)
	}
	return out, nil
}

// Walkin: tamu tambahan tanpa undangan yang dicatat penerima tamu.
type Walkin struct {
	ID        uuid.UUID
	Name      string
	Pax       int
	CreatedAt time.Time
}

// AddWalkin mencatat tamu tambahan.
func (s *Service) AddWalkin(ctx context.Context, weddingID uuid.UUID, name string, pax int) (Walkin, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Walkin{}, ValidationError{"name": "Nama wajib diisi"}
	}
	if utf8.RuneCountInString(name) > 100 {
		return Walkin{}, ValidationError{"name": "Nama maksimal 100 karakter"}
	}
	pax = min(max(pax, 1), 20)
	row, err := s.repo.q.CreateWalkin(ctx, guestdb.CreateWalkinParams{ID: db.NewID(), WeddingID: weddingID, Name: name, Pax: int16(pax), CreatedAt: s.now()}) //nolint:gosec // G115: 1..20
	if err != nil {
		return Walkin{}, fmt.Errorf("guest: tamu tambahan: %w", err)
	}
	return Walkin{ID: row.ID, Name: row.Name, Pax: int(row.Pax), CreatedAt: row.CreatedAt}, nil
}

// CheckinStats: ringkasan kehadiran nyata.
type CheckinStats struct {
	Invited      int // jumlah undangan
	CheckedIn    int // undangan yang sudah datang
	CheckedInPax int // jumlah orang dari undangan yang datang
	Walkins      int // tamu tambahan
	WalkinPax    int
}

// TotalPax: jumlah orang yang hadir (undangan + tamu tambahan).
func (c CheckinStats) TotalPax() int { return c.CheckedInPax + c.WalkinPax }

// CheckinSummary mengembalikan ringkasan kehadiran dan check-in terbaru.
func (s *Service) CheckinSummary(ctx context.Context, weddingID uuid.UUID, recent int) (CheckinStats, []Guest, error) {
	t, err := s.repo.q.CheckinTotals(ctx, weddingID)
	if err != nil {
		return CheckinStats{}, nil, fmt.Errorf("guest: ringkasan check-in: %w", err)
	}
	wk, err := s.repo.q.WalkinTotals(ctx, weddingID)
	if err != nil {
		return CheckinStats{}, nil, fmt.Errorf("guest: ringkasan tamu tambahan: %w", err)
	}
	st := CheckinStats{Invited: int(t.Invited), CheckedIn: int(t.CheckedIn), CheckedInPax: int(t.CheckedInPax), Walkins: int(wk.N), WalkinPax: int(wk.Pax)}
	rows, err := s.repo.q.RecentCheckins(ctx, guestdb.RecentCheckinsParams{WeddingID: weddingID, Limit: int32(recent)}) //nolint:gosec // G115: kecil
	if err != nil {
		return CheckinStats{}, nil, fmt.Errorf("guest: check-in terbaru: %w", err)
	}
	gs := make([]Guest, len(rows))
	for i, r := range rows {
		gs[i] = toGuest(r)
	}
	return st, gs, nil
}
