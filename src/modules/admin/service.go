package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/domain"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/web"

	admindb "github.com/khamdanngazis/lovaria/src/modules/admin/db"
)

const PerPage = 25

var (
	ErrNotFound    = errors.New("data tidak ditemukan")
	ErrPackageUsed = errors.New("paket masih dipakai wedding; lepas dulu dari wedding tersebut")
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

// Package adalah paket layanan (batas storage & lama arsip).
type Package struct {
	ID           uuid.UUID
	Name         string
	StorageMB    int
	ArchiveDays  int
	PriceDisplay string
	Weddings     int // jumlah wedding yang memakai (hanya di ListPackages)
}

func toPackage(r admindb.Package) Package {
	return Package{ID: r.ID, Name: r.Name, StorageMB: int(r.StorageMb), ArchiveDays: int(r.ArchiveDays), PriceDisplay: r.PriceDisplay}
}

// PackageInput: nilai mentah form paket.
type PackageInput struct {
	Name, StorageMB, ArchiveDays, PriceDisplay string
}

// AuditEntry adalah satu baris audit log.
type AuditEntry struct {
	ID         uuid.UUID
	AdminEmail string
	Action     string
	TargetType string
	TargetID   string
	Details    map[string]string
	CreatedAt  time.Time
}

// Service: panel admin. Data modul lain HANYA lewat service modul tersebut;
// modul admin memiliki tabel packages, wedding_packages, admin_audit_logs.
type Service struct {
	q            *admindb.Queries
	Users        *auth.Service
	Weddings     *wedding.Service
	Guests       *guest.Service
	Gallery      *gallery.Service
	Domains      *domain.Service
	Themes       *theme.Service
	secret       []byte
	cookieSecure bool
	log          *slog.Logger
	now          func() time.Time
}

type Deps struct {
	Pool     *pgxpool.Pool
	Users    *auth.Service
	Weddings *wedding.Service
	Guests   *guest.Service
	Gallery  *gallery.Service
	Domains  *domain.Service
	Themes   *theme.Service
	// Secret: kunci HMAC cookie mode lihat-saja (APP_SECRET).
	Secret []byte
	// CookieSecure: atribut Secure untuk cookie mode lihat-saja.
	CookieSecure bool
	Log          *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{
		q: admindb.New(d.Pool), Users: d.Users, Weddings: d.Weddings, Guests: d.Guests, Gallery: d.Gallery,
		Domains: d.Domains, Themes: d.Themes, secret: d.Secret, cookieSecure: d.CookieSecure, log: d.Log, now: time.Now,
	}
}

// ---------- Audit log ----------

// audit mencatat aksi tulis admin. Dipanggil oleh setiap method yang mengubah data.
func (s *Service) audit(ctx context.Context, action, targetType, targetID string, details map[string]string) error {
	u, ok := web.CurrentUser(ctx)
	if !ok {
		return errors.New("admin: audit tanpa user login")
	}
	if details == nil {
		details = map[string]string{}
	}
	b, _ := json.Marshal(details)
	err := s.q.InsertAuditLog(ctx, admindb.InsertAuditLogParams{
		ID: db.NewID(), AdminUserID: &u.ID, AdminEmail: u.Email, Action: action,
		TargetType: targetType, TargetID: targetID, Details: b, CreatedAt: s.now(),
	})
	if err != nil {
		return fmt.Errorf("admin: audit: %w", err)
	}
	s.log.InfoContext(ctx, "admin: aksi", slog.String("admin", u.Email), slog.String("action", action), slog.String("target", targetType+":"+targetID))
	return nil
}

// AuditLog: daftar audit (terbaru dulu); targetID opsional.
func (s *Service) AuditLog(ctx context.Context, targetID string, page int) ([]AuditEntry, int, error) {
	var t *string
	if targetID != "" {
		t = &targetID
	}
	page = max(1, page)
	total, err := s.q.CountAuditLogs(ctx, t)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.ListAuditLogs(ctx, admindb.ListAuditLogsParams{TargetID: t, Lim: PerPage, Off: int32((page - 1) * PerPage)}) //nolint:gosec // G115: halaman kecil
	if err != nil {
		return nil, 0, err
	}
	out := make([]AuditEntry, len(rows))
	for i, r := range rows {
		out[i] = AuditEntry{ID: r.ID, AdminEmail: r.AdminEmail, Action: r.Action, TargetType: r.TargetType, TargetID: r.TargetID, CreatedAt: r.CreatedAt}
		_ = json.Unmarshal(r.Details, &out[i].Details)
	}
	return out, int(total), nil
}

// ---------- Customers ----------

func (s *Service) SetUserDisabled(ctx context.Context, userID uuid.UUID, disabled bool) (auth.User, error) {
	if u, ok := web.CurrentUser(ctx); ok && u.ID == userID && disabled {
		return auth.User{}, ValidationError{"user": "Tidak bisa menonaktifkan akun sendiri"}
	}
	u, err := s.Users.SetDisabled(ctx, userID, disabled)
	if errors.Is(err, auth.ErrUserNotFound) {
		return auth.User{}, ErrNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	action := "user.enable"
	if disabled {
		action = "user.disable"
	}
	return u, s.audit(ctx, action, "user", userID.String(), map[string]string{"email": u.Email})
}

// ---------- Weddings ----------

// SetWeddingStatus: ubah status sebagai admin (tabel transisi T12, aktor admin).
func (s *Service) SetWeddingStatus(ctx context.Context, weddingID uuid.UUID, to string) (wedding.Wedding, error) {
	u, _ := web.CurrentUser(ctx)
	before, err := s.Weddings.GetWedding(ctx, weddingID)
	if errors.Is(err, wedding.ErrNotFound) {
		return wedding.Wedding{}, ErrNotFound
	}
	if err != nil {
		return wedding.Wedding{}, err
	}
	w, err := s.Weddings.Transition(ctx, weddingID, to, wedding.Actor{Kind: wedding.ActorAdmin, UserID: u.ID})
	if err != nil {
		return wedding.Wedding{}, err
	}
	return w, s.audit(ctx, "wedding.status", "wedding", weddingID.String(), map[string]string{"from": before.Status, "to": to, "slug": w.Slug})
}

// ---------- Packages ----------

func parsePackage(in PackageInput) (admindb.Package, error) {
	v := ValidationError{}
	p := admindb.Package{Name: strings.TrimSpace(in.Name), PriceDisplay: strings.TrimSpace(in.PriceDisplay)}
	if n := utf8.RuneCountInString(p.Name); n == 0 || n > 60 {
		v["name"] = "Nama paket 1–60 karakter"
	}
	mb, err := strconv.Atoi(strings.TrimSpace(in.StorageMB))
	if err != nil || mb < 1 || mb > 102400 {
		v["storage_mb"] = "Storage 1–102400 MB"
	}
	days, err := strconv.Atoi(strings.TrimSpace(in.ArchiveDays))
	if err != nil || days < 1 || days > 3650 {
		v["archive_days"] = "Durasi arsip 1–3650 hari"
	}
	if utf8.RuneCountInString(p.PriceDisplay) > 40 {
		v["price_display"] = "Harga maksimal 40 karakter"
	}
	if len(v) > 0 {
		return p, v
	}
	p.StorageMb, p.ArchiveDays = int32(mb), int32(days) //nolint:gosec // G115: sudah divalidasi
	return p, nil
}

func isUniqueName(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Service) ListPackages(ctx context.Context) ([]Package, error) {
	rows, err := s.q.ListPackages(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Package, len(rows))
	for i, r := range rows {
		out[i] = Package{ID: r.ID, Name: r.Name, StorageMB: int(r.StorageMb), ArchiveDays: int(r.ArchiveDays), PriceDisplay: r.PriceDisplay, Weddings: int(r.Weddings)}
	}
	return out, nil
}

func (s *Service) CreatePackage(ctx context.Context, in PackageInput) (Package, error) {
	p, err := parsePackage(in)
	if err != nil {
		return Package{}, err
	}
	row, err := s.q.CreatePackage(ctx, admindb.CreatePackageParams{ID: db.NewID(), Name: p.Name, StorageMb: p.StorageMb, ArchiveDays: p.ArchiveDays, PriceDisplay: p.PriceDisplay})
	if isUniqueName(err) {
		return Package{}, ValidationError{"name": "Nama paket sudah dipakai"}
	}
	if err != nil {
		return Package{}, err
	}
	return toPackage(row), s.audit(ctx, "package.create", "package", row.ID.String(), map[string]string{"name": row.Name})
}

func (s *Service) UpdatePackage(ctx context.Context, id uuid.UUID, in PackageInput) (Package, error) {
	p, err := parsePackage(in)
	if err != nil {
		return Package{}, err
	}
	row, err := s.q.UpdatePackage(ctx, admindb.UpdatePackageParams{ID: id, Name: p.Name, StorageMb: p.StorageMb, ArchiveDays: p.ArchiveDays, PriceDisplay: p.PriceDisplay})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Package{}, ErrNotFound
	case isUniqueName(err):
		return Package{}, ValidationError{"name": "Nama paket sudah dipakai"}
	case err != nil:
		return Package{}, err
	}
	return toPackage(row), s.audit(ctx, "package.update", "package", id.String(), map[string]string{
		"name": row.Name, "storage_mb": strconv.Itoa(int(row.StorageMb)), "archive_days": strconv.Itoa(int(row.ArchiveDays)),
	})
}

func (s *Service) DeletePackage(ctx context.Context, id uuid.UUID) error {
	n, err := s.q.DeletePackage(ctx, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // masih dirujuk wedding_packages
		return ErrPackageUsed
	}
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, "package.delete", "package", id.String(), nil)
}

// WeddingPackage: paket wedding (ok=false → default dari config).
func (s *Service) WeddingPackage(ctx context.Context, weddingID uuid.UUID) (Package, bool, error) {
	row, err := s.q.GetWeddingPackage(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Package{}, false, nil
	}
	if err != nil {
		return Package{}, false, err
	}
	return toPackage(row), true, nil
}

// AssignPackage memasang paket ke wedding; packageID = uuid.Nil melepas paket.
func (s *Service) AssignPackage(ctx context.Context, weddingID, packageID uuid.UUID) error {
	if _, err := s.Weddings.GetWedding(ctx, weddingID); errors.Is(err, wedding.ErrNotFound) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if packageID == uuid.Nil {
		if _, err := s.q.UnassignPackage(ctx, weddingID); err != nil {
			return err
		}
		return s.audit(ctx, "wedding.package", "wedding", weddingID.String(), map[string]string{"package": "(default)"})
	}
	p, err := s.q.GetPackage(ctx, packageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := s.q.AssignPackage(ctx, admindb.AssignPackageParams{WeddingID: weddingID, PackageID: packageID, AssignedAt: s.now()}); err != nil {
		return err
	}
	return s.audit(ctx, "wedding.package", "wedding", weddingID.String(), map[string]string{"package": p.Name})
}

// QuotaBytes & ArchiveDays dipasang ke gallery / wedding (SetQuotaSource,
// SetArchiveDaysSource): nilai dari paket wedding bila ada.
func (s *Service) QuotaBytes(ctx context.Context, weddingID uuid.UUID) (int64, bool, error) {
	p, ok, err := s.WeddingPackage(ctx, weddingID)
	return int64(p.StorageMB) << 20, ok, err
}

func (s *Service) ArchiveDays(ctx context.Context, weddingID uuid.UUID) (int, bool, error) {
	p, ok, err := s.WeddingPackage(ctx, weddingID)
	return p.ArchiveDays, ok, err
}

// ---------- Themes ----------

func (s *Service) SetThemeEnabled(ctx context.Context, themeID string, enabled bool) error {
	if err := s.Themes.SetEnabled(ctx, themeID, enabled); err != nil {
		if errors.Is(err, theme.ErrUnknownTheme) {
			return ErrNotFound
		}
		return err
	}
	action := "theme.enable"
	if !enabled {
		action = "theme.disable"
	}
	return s.audit(ctx, action, "theme", themeID, nil)
}
