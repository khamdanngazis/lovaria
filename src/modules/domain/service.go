// Package domain: custom domain per wedding lewat Cloudflare for SaaS (Custom
// Hostnames, Arsitektur §5). Pasangan mendaftarkan domain → Lovoria membuat
// custom hostname di Cloudflare → pasangan memasang CNAME → scheduler memantau
// sampai aktif (atau gagal setelah 72 jam) → resolver public site mencocokkan
// Host header ke wedding lewat WeddingIDByHost.
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	domaindb "github.com/khamdanngazis/lovaria/src/modules/domain/db"
)

const (
	StatusPending = "pending_verification"
	StatusActive  = "active"
	StatusFailed  = "failed"
	StatusRemoved = "removed"
)

var statusLabels = map[string]string{
	StatusPending: "Menunggu verifikasi",
	StatusActive:  "Aktif",
	StatusFailed:  "Gagal",
	StatusRemoved: "Dihapus",
}

func StatusLabel(s string) string {
	if l, ok := statusLabels[s]; ok {
		return l
	}
	return s
}

var (
	ErrNotFound = errors.New("domain tidak ditemukan")
	// ErrDisabled: Cloudflare belum dikonfigurasi (CLOUDFLARE_* kosong).
	ErrDisabled = errors.New("fitur custom domain belum diaktifkan")
	// ErrProvider: Cloudflare menolak / tidak bisa dihubungi.
	ErrProvider = errors.New("gagal menghubungi Cloudflare, coba lagi beberapa saat lagi")
)

// ValidationError memetakan nama field ke pesan error.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

type Domain struct {
	ID            uuid.UUID
	WeddingID     uuid.UUID
	Domain        string
	Status        string
	Errors        []string
	VerifiedAt    *time.Time
	LastCheckedAt *time.Time
	CreatedAt     time.Time
}

func (d Domain) Active() bool { return d.Status == StatusActive }

// Config adalah setelan modul (dari config aplikasi).
type Config struct {
	// CNAMETarget: tujuan CNAME yang dipasang pasangan, mis. domains.lovoria.com.
	CNAMETarget string
	// Reserved: domain milik Lovoria yang tidak boleh didaftarkan (beserta subdomainnya).
	Reserved []string
	// PendingTimeout: lama menunggu verifikasi sebelum dianggap gagal (default 72 jam).
	PendingTimeout time.Duration
	// CacheTTL: umur cache lookup Host → wedding (default 60 detik).
	CacheTTL time.Duration
	// QuotaWarn: ambang peringatan jumlah hostname aktif (default 90 dari kuota gratis 100).
	QuotaWarn int
}

type cacheEntry struct {
	id   uuid.UUID // lookup Host → wedding
	host string    // lookup wedding → domain aktif
	ok   bool
	exp  time.Time
}

type Service struct {
	pool *pgxpool.Pool
	q    *domaindb.Queries
	cf   Hostnames // nil → fitur nonaktif
	cfg  Config
	log  *slog.Logger
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewService: cf boleh nil (custom domain nonaktif; lookup tetap berjalan).
func NewService(pool *pgxpool.Pool, cf Hostnames, cfg Config, log *slog.Logger) *Service {
	cfg.Reserved = append([]string{}, cfg.Reserved...)
	if cfg.PendingTimeout <= 0 {
		cfg.PendingTimeout = 72 * time.Hour
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = time.Minute
	}
	if cfg.QuotaWarn <= 0 {
		cfg.QuotaWarn = 90
	}
	// Domain Lovoria sendiri beserta domain induknya (lovoria.com, railway.app, …).
	reserved := append([]string{}, cfg.Reserved...)
	if cfg.CNAMETarget != "" {
		reserved = append(reserved, cfg.CNAMETarget)
	}
	for _, r := range reserved {
		if r != "" {
			cfg.Reserved = append(cfg.Reserved, ParentDomain(r))
		}
	}
	return &Service{pool: pool, q: domaindb.New(pool), cf: cf, cfg: cfg, log: log, now: time.Now, cache: map[string]cacheEntry{}}
}

// Enabled: Cloudflare & target CNAME sudah dikonfigurasi.
func (s *Service) Enabled() bool { return s.cf != nil && s.cfg.CNAMETarget != "" }

func (s *Service) CNAMETarget() string { return s.cfg.CNAMETarget }

func toDomain(r domaindb.CustomDomain) Domain {
	d := Domain{
		ID: r.ID, WeddingID: r.WeddingID, Domain: r.Domain, Status: r.Status,
		VerifiedAt: r.VerifiedAt, LastCheckedAt: r.LastCheckedAt, CreatedAt: r.CreatedAt,
	}
	_ = json.Unmarshal(r.VerificationErrors, &d.Errors)
	return d
}

func errorsJSON(errs []string) []byte {
	if errs == nil {
		errs = []string{}
	}
	b, _ := json.Marshal(errs)
	return b
}

// invalidate mengosongkan cache lookup (dipanggil setiap status berubah).
func (s *Service) invalidate() {
	s.mu.Lock()
	s.cache = map[string]cacheEntry{}
	s.mu.Unlock()
}

// Get mengembalikan domain wedding (ErrNotFound bila belum ada).
func (s *Service) Get(ctx context.Context, weddingID uuid.UUID) (Domain, error) {
	r, err := s.q.GetByWedding(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrNotFound
	}
	if err != nil {
		return Domain{}, err
	}
	return toDomain(r), nil
}

// Add mendaftarkan domain untuk wedding. Mendaftarkan domain yang sama lagi
// mengembalikan data yang ada; domain lain harus dihapus dulu.
func (s *Service) Add(ctx context.Context, weddingID uuid.UUID, input string) (Domain, error) {
	if !s.Enabled() {
		return Domain{}, ErrDisabled
	}
	name, err := Normalize(input)
	if err != nil {
		return Domain{}, ValidationError{"domain": err.Error()}
	}
	if reservedSuffix(name, s.cfg.Reserved) {
		return Domain{}, ValidationError{"domain": "Domain Lovoria tidak bisa dipakai sebagai custom domain"}
	}
	if cur, err := s.Get(ctx, weddingID); err == nil {
		if cur.Domain == name {
			return cur, nil
		}
		return Domain{}, ValidationError{"domain": "Undangan ini sudah punya domain " + cur.Domain + ". Hapus dulu sebelum mengganti."}
	} else if !errors.Is(err, ErrNotFound) {
		return Domain{}, err
	}
	taken, err := s.q.DomainTaken(ctx, domaindb.DomainTakenParams{Domain: name, WeddingID: weddingID})
	if err != nil {
		return Domain{}, err
	}
	if taken {
		return Domain{}, ValidationError{"domain": "Domain ini sudah dipakai undangan lain"}
	}

	h, err := s.cf.Create(ctx, name)
	if err != nil {
		s.log.WarnContext(ctx, "domain: cloudflare create", slog.String("domain", name), slog.String("error", err.Error()))
		return Domain{}, ErrProvider
	}
	now := s.now()
	status, verified := StatusPending, (*time.Time)(nil)
	if h.Active() {
		status, verified = StatusActive, &now
	}
	row, err := s.q.InsertDomain(ctx, domaindb.InsertDomainParams{
		ID: db.NewID(), WeddingID: weddingID, Domain: name, CfHostnameID: h.ID, Status: status,
		VerificationErrors: errorsJSON(h.Errors), VerifiedAt: verified, LastCheckedAt: &now,
		CreatedAt: now, // jam yang sama dengan batas 72 jam di check
	})
	if err != nil {
		// Balapan dengan permintaan lain: jangan tinggalkan hostname yatim di Cloudflare.
		if derr := s.cf.Delete(ctx, h.ID); derr != nil {
			s.log.ErrorContext(ctx, "domain: rollback cloudflare", slog.String("hostname_id", h.ID), slog.String("error", derr.Error()))
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Domain{}, ValidationError{"domain": "Domain ini sudah dipakai undangan lain"}
		}
		return Domain{}, fmt.Errorf("domain: simpan: %w", err)
	}
	s.invalidate()
	s.log.InfoContext(ctx, "domain: didaftarkan", slog.String("domain", name), slog.String("wedding_id", weddingID.String()))
	return toDomain(row), nil
}

// Recheck menanyakan status terbaru ke Cloudflare ("Cek ulang" di dashboard).
func (s *Service) Recheck(ctx context.Context, weddingID uuid.UUID) (Domain, error) {
	if !s.Enabled() {
		return Domain{}, ErrDisabled
	}
	d, err := s.q.GetByWedding(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Domain{}, ErrNotFound
	}
	if err != nil {
		return Domain{}, err
	}
	return s.check(ctx, d)
}

// check memperbarui satu domain dari status Cloudflare.
func (s *Service) check(ctx context.Context, r domaindb.CustomDomain) (Domain, error) {
	now := s.now()
	h, err := s.cf.Get(ctx, r.CfHostnameID)
	var status string
	var errs []string
	verified := r.VerifiedAt
	switch {
	case errors.Is(err, ErrHostnameNotFound):
		status, errs = StatusRemoved, []string{"Domain tidak lagi terdaftar di Cloudflare. Hapus lalu daftarkan ulang."}
	case err != nil:
		s.log.WarnContext(ctx, "domain: cloudflare get", slog.String("domain", r.Domain), slog.String("error", err.Error()))
		return toDomain(r), ErrProvider
	case h.Gone():
		status, errs = StatusRemoved, []string{"Domain tidak lagi terdaftar di Cloudflare. Hapus lalu daftarkan ulang."}
	case h.Active():
		status, errs = StatusActive, nil
		if verified == nil {
			verified = &now
		}
	case now.Sub(r.CreatedAt) > s.cfg.PendingTimeout:
		status, errs = StatusFailed, h.Errors
	default:
		status, errs = StatusPending, h.Errors
	}
	row, err := s.q.UpdateStatus(ctx, domaindb.UpdateStatusParams{
		ID: r.ID, WeddingID: r.WeddingID, Status: status, VerificationErrors: errorsJSON(errs), VerifiedAt: verified, LastCheckedAt: &now,
	})
	if err != nil {
		return Domain{}, err
	}
	if status != r.Status {
		s.invalidate()
		s.log.InfoContext(ctx, "domain: status berubah", slog.String("domain", r.Domain), slog.String("from", r.Status), slog.String("to", status))
		if status == StatusActive {
			s.warnQuota(ctx)
		}
	}
	return toDomain(row), nil
}

// warnQuota mencatat peringatan bila hostname aktif mendekati kuota gratis
// Cloudflare for SaaS (100).
func (s *Service) warnQuota(ctx context.Context) {
	n, err := s.ActiveCount(ctx)
	if err == nil && n >= s.cfg.QuotaWarn {
		s.log.WarnContext(ctx, "domain: jumlah custom hostname aktif mendekati kuota Cloudflare for SaaS",
			slog.Int("active", n), slog.Int("warn_at", s.cfg.QuotaWarn), slog.Int("free_quota", 100))
	}
}

// ActiveCount: jumlah domain aktif (pemantauan kuota; panel admin T16).
func (s *Service) ActiveCount(ctx context.Context) (int, error) {
	n, err := s.q.CountActive(ctx)
	return int(n), err
}

// Remove menghapus custom hostname di Cloudflare lalu baris domain. Bila
// Cloudflare gagal, baris tetap ada supaya tidak ada hostname yatim.
func (s *Service) Remove(ctx context.Context, weddingID uuid.UUID) error {
	d, err := s.q.GetByWedding(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if d.CfHostnameID != "" {
		if s.cf == nil {
			return ErrDisabled
		}
		if err := s.cf.Delete(ctx, d.CfHostnameID); err != nil && !errors.Is(err, ErrHostnameNotFound) {
			s.log.WarnContext(ctx, "domain: cloudflare delete", slog.String("domain", d.Domain), slog.String("error", err.Error()))
			return ErrProvider
		}
	}
	if _, err := s.q.DeleteByWedding(ctx, weddingID); err != nil {
		return err
	}
	s.invalidate()
	s.log.InfoContext(ctx, "domain: dihapus", slog.String("domain", d.Domain), slog.String("wedding_id", weddingID.String()))
	return nil
}

// WeddingIDByHost memenuhi publicsite.DomainLookup: hanya domain aktif, dengan
// cache in-memory (TTL CacheTTL, dikosongkan saat status berubah).
func (s *Service) WeddingIDByHost(ctx context.Context, host string) (uuid.UUID, bool, error) {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	now := s.now()
	s.mu.Lock()
	e, hit := s.cache[host]
	s.mu.Unlock()
	if hit && now.Before(e.exp) {
		return e.id, e.ok, nil
	}
	r, err := s.q.GetActiveByDomain(ctx, host)
	e = cacheEntry{exp: now.Add(s.cfg.CacheTTL)}
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return uuid.Nil, false, err
	default:
		e.id, e.ok = r.WeddingID, true
	}
	s.mu.Lock()
	s.cache[host] = e
	s.mu.Unlock()
	return e.id, e.ok, nil
}

// ActiveDomain memenuhi wedding.DomainSource (URL kanonik undangan). Di-cache
// seperti lookup Host karena dipanggil di setiap akses /w/:slug.
func (s *Service) ActiveDomain(ctx context.Context, weddingID uuid.UUID) (string, bool, error) {
	key, now := "w:"+weddingID.String(), s.now()
	s.mu.Lock()
	e, hit := s.cache[key]
	s.mu.Unlock()
	if hit && now.Before(e.exp) {
		return e.host, e.ok, nil
	}
	d, err := s.Get(ctx, weddingID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return "", false, err
	}
	e = cacheEntry{exp: now.Add(s.cfg.CacheTTL)}
	if err == nil && d.Active() {
		e.host, e.ok = d.Domain, true
	}
	s.mu.Lock()
	s.cache[key] = e
	s.mu.Unlock()
	return e.host, e.ok, nil
}

// ListAll: semua domain (panel admin T16), terbaru dulu, halaman mulai 1.
func (s *Service) ListAll(ctx context.Context, page, perPage int) ([]Domain, int, error) {
	page = max(1, page)
	total, err := s.q.CountAllDomains(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAllDomains(ctx, domaindb.ListAllDomainsParams{Lim: int32(perPage), Off: int32((page - 1) * perPage)}) //nolint:gosec // G115: halaman kecil
	if err != nil {
		return nil, 0, err
	}
	out := make([]Domain, len(rows))
	for i, r := range rows {
		out[i] = toDomain(r)
	}
	return out, int(total), nil
}

// QuotaWarn: ambang peringatan kuota (ditampilkan di panel admin).
func (s *Service) QuotaWarn() int { return s.cfg.QuotaWarn }
