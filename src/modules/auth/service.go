// Package auth: registrasi & login couple/admin, session server-side,
// reset password, middleware RequireAuth/RequireRole. Guest tidak punya akun.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/platform/db"
	platformmail "github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/web"

	authdb "github.com/khamdanngazis/lovaria/src/modules/auth/db"
)

const (
	RoleCouple = "couple"
	RoleAdmin  = "admin"

	// SessionTTL: rolling expiry — diperpanjang saat user aktif.
	SessionTTL = 30 * 24 * time.Hour
	// sessionRefresh: session diperpanjang paling sering sekali per interval ini
	// (menghindari write DB di setiap request).
	sessionRefresh = 24 * time.Hour
	// ResetTokenTTL masa berlaku link reset password.
	ResetTokenTTL = time.Hour

	tokenBytes     = 32
	minPasswordLen = 8
	maxPasswordLen = 256
	maxNameLen     = 100
	maxEmailLen    = 254
)

var (
	// ErrInvalidCredentials sengaja generik: tidak membedakan email tidak ada vs password salah.
	ErrInvalidCredentials = errors.New("email atau password salah")
	ErrUserNotFound       = errors.New("user tidak ditemukan")
	// ErrAccountDisabled hanya dikembalikan setelah password benar (tidak membocorkan akun).
	ErrAccountDisabled   = errors.New("akun ini dinonaktifkan. Hubungi admin Lunovia untuk bantuan")
	ErrSessionInvalid    = errors.New("session tidak valid atau kedaluwarsa")
	ErrInvalidResetToken = errors.New("link reset password tidak valid atau sudah kedaluwarsa")
)

// ValidationError memetakan nama field ke pesan error yang siap ditampilkan.
type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

// User adalah akun couple atau admin.
type User struct {
	ID              uuid.UUID
	Email           string
	Name            string
	Role            string
	EmailVerifiedAt *time.Time
	// DisabledAt: dinonaktifkan admin (T16) — tidak bisa login.
	DisabledAt *time.Time
	CreatedAt  time.Time

	passwordHash string
}

// Web mengubah User menjadi identitas ringkas untuk context request.
func (u User) Web() web.User {
	return web.User{ID: u.ID, Email: u.Email, Name: u.Name, Role: u.Role}
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type Service struct {
	repo    *Repository
	mailer  platformmail.Mailer
	baseURL string
	log     *slog.Logger
	now     func() time.Time
}

func NewService(repo *Repository, mailer platformmail.Mailer, baseURL string, log *slog.Logger) *Service {
	return &Service{repo: repo, mailer: mailer, baseURL: strings.TrimRight(baseURL, "/"), log: log, now: time.Now}
}

// ---------- Validasi ----------

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ValidateField memvalidasi satu field form registrasi/reset ("" bila valid).
func ValidateField(field, value string) string {
	switch field {
	case "name":
		n := utf8.RuneCountInString(strings.TrimSpace(value))
		switch {
		case n == 0:
			return "Nama wajib diisi"
		case n > maxNameLen:
			return fmt.Sprintf("Nama maksimal %d karakter", maxNameLen)
		}
	case "email":
		e := normalizeEmail(value)
		if e == "" {
			return "Email wajib diisi"
		}
		addr, err := mail.ParseAddress(e)
		if err != nil || addr.Address != e || len(e) > maxEmailLen || !strings.Contains(e[strings.LastIndex(e, "@"):], ".") {
			return "Format email tidak valid"
		}
	case "password":
		n := utf8.RuneCountInString(value)
		switch {
		case n < minPasswordLen:
			return fmt.Sprintf("Password minimal %d karakter", minPasswordLen)
		case n > maxPasswordLen:
			return fmt.Sprintf("Password maksimal %d karakter", maxPasswordLen)
		}
	}
	return ""
}

func validate(fields map[string]string) error {
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

// ---------- Akun ----------

// Register membuat akun couple baru.
func (s *Service) Register(ctx context.Context, in RegisterInput) (User, error) {
	return s.createUser(ctx, in, RoleCouple)
}

// CreateAdmin membuat akun admin (dipakai CLI `lovoria create-admin`).
func (s *Service) CreateAdmin(ctx context.Context, in RegisterInput) (User, error) {
	return s.createUser(ctx, in, RoleAdmin)
}

func (s *Service) createUser(ctx context.Context, in RegisterInput, role string) (User, error) {
	if err := validate(map[string]string{"name": in.Name, "email": in.Email, "password": in.Password}); err != nil {
		return User{}, err
	}
	hash, err := HashPassword(in.Password)
	if err != nil {
		return User{}, err
	}
	u, err := s.repo.q.CreateUser(ctx, authdb.CreateUserParams{
		ID:           db.NewID(),
		Email:        normalizeEmail(in.Email),
		PasswordHash: hash,
		Name:         strings.TrimSpace(in.Name),
		Role:         role,
	})
	if err := mapErr(err); errors.Is(err, errEmailTaken) {
		return User{}, ValidationError{"email": "Email sudah terdaftar"}
	} else if err != nil {
		return User{}, fmt.Errorf("auth: create user: %w", err)
	}
	return toUser(u), nil
}

// Authenticate memeriksa email & password. Error selalu ErrInvalidCredentials
// untuk kredensial salah, apa pun penyebabnya.
func (s *Service) Authenticate(ctx context.Context, email, password string) (User, error) {
	row, err := s.repo.q.GetUserByEmail(ctx, normalizeEmail(email))
	if err := mapErr(err); errors.Is(err, errNotFound) {
		_, _, _ = VerifyPassword(password, dummyHash) // samakan waktu respons
		return User{}, ErrInvalidCredentials
	} else if err != nil {
		return User{}, fmt.Errorf("auth: get user: %w", err)
	}

	ok, rehash, err := VerifyPassword(password, row.PasswordHash)
	if err != nil {
		return User{}, fmt.Errorf("auth: verify: %w", err)
	}
	if !ok {
		return User{}, ErrInvalidCredentials
	}
	if row.DisabledAt != nil {
		return User{}, ErrAccountDisabled
	}
	if rehash {
		if h, err := HashPassword(password); err == nil {
			if err := s.repo.q.UpdateUserPassword(ctx, authdb.UpdateUserPasswordParams{ID: row.ID, PasswordHash: h}); err != nil {
				s.log.WarnContext(ctx, "auth: rehash password gagal", slog.String("error", err.Error()))
			}
		}
	}
	return toUser(row), nil
}

// LoginWithGoogle mengembalikan user untuk identitas Google yang sudah
// diverifikasi (T30):
//
//  1. Identitas sudah terhubung → user itu.
//  2. Belum, tetapi ada akun dengan email yang sama → dihubungkan. Bila email
//     akun itu belum pernah terverifikasi (dibuat dengan password), password
//     lama dimatikan dan semua sesinya dicabut — mencegah orang lain yang lebih
//     dulu mendaftarkan email ini tetap bisa masuk (resetPassword = true).
//  3. Belum ada akun → dibuat (tanpa password yang bisa dipakai; bisa diatur
//     lewat "Lupa password").
func (s *Service) LoginWithGoogle(ctx context.Context, p GoogleProfile) (u User, resetPassword bool, err error) {
	if p.Subject == "" || p.Email == "" {
		return User{}, false, errors.New("auth: profil Google tidak lengkap")
	}
	row, err := s.repo.q.GetUserByIdentity(ctx, authdb.GetUserByIdentityParams{Provider: ProviderGoogle, Subject: p.Subject})
	switch err := mapErr(err); {
	case err == nil:
		if row.DisabledAt != nil {
			return User{}, false, ErrAccountDisabled
		}
		return toUser(row), false, nil
	case !errors.Is(err, errNotFound):
		return User{}, false, fmt.Errorf("auth: cari identitas: %w", err)
	}

	// Password acak yang tidak diketahui siapa pun (kolom password_hash wajib isi).
	unusable := func() (string, error) {
		t, err := newToken()
		if err != nil {
			return "", err
		}
		return HashPassword(t)
	}
	now := s.now()
	err = s.repo.inTx(ctx, func(q *authdb.Queries) error {
		row, err = q.GetUserByEmail(ctx, p.Email)
		switch err := mapErr(err); {
		case err == nil:
			if row.DisabledAt != nil {
				return ErrAccountDisabled
			}
			if row.EmailVerifiedAt == nil {
				hash, err := unusable()
				if err != nil {
					return err
				}
				if err := q.UpdateUserPassword(ctx, authdb.UpdateUserPasswordParams{ID: row.ID, PasswordHash: hash}); err != nil {
					return err
				}
				if err := q.DeleteUserSessions(ctx, row.ID); err != nil {
					return err
				}
				if err := q.DeleteUserPasswordResetTokens(ctx, row.ID); err != nil {
					return err
				}
				resetPassword = true
			}
		case errors.Is(err, errNotFound):
			hash, err := unusable()
			if err != nil {
				return err
			}
			name := p.Name
			if name == "" {
				name = strings.SplitN(p.Email, "@", 2)[0]
			}
			if row, err = q.CreateUser(ctx, authdb.CreateUserParams{ID: db.NewID(), Email: p.Email, PasswordHash: hash, Name: name, Role: RoleCouple}); err != nil {
				return err
			}
		default:
			return err
		}
		if err := q.MarkEmailVerified(ctx, authdb.MarkEmailVerifiedParams{ID: row.ID, EmailVerifiedAt: &now}); err != nil {
			return err
		}
		return q.CreateIdentity(ctx, authdb.CreateIdentityParams{Provider: ProviderGoogle, Subject: p.Subject, UserID: row.ID, Email: p.Email})
	})
	if errors.Is(err, ErrAccountDisabled) {
		return User{}, false, err
	}
	if err != nil {
		return User{}, false, fmt.Errorf("auth: masuk dengan Google: %w", err)
	}
	return toUser(row), resetPassword, nil
}

// ---------- Session ----------

// CreateSession membuat session baru dan mengembalikan token untuk cookie.
func (s *Service) CreateSession(ctx context.Context, userID uuid.UUID, ip, userAgent string) (token string, expiresAt time.Time, err error) {
	token, err = newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = s.now().Add(SessionTTL)
	err = s.repo.q.CreateSession(ctx, authdb.CreateSessionParams{
		ID:        hashToken(token),
		UserID:    userID,
		ExpiresAt: expiresAt,
		Ip:        truncate(ip, 64),
		UserAgent: truncate(userAgent, 512),
	})
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth: create session: %w", err)
	}
	return token, expiresAt, nil
}

// SessionUser mengembalikan pemilik session. Bila session diperpanjang (rolling
// expiry), extendedUntil berisi masa berlaku baru supaya cookie ikut diperbarui.
func (s *Service) SessionUser(ctx context.Context, token string) (u User, extendedUntil *time.Time, err error) {
	id, ok := parseToken(token)
	if !ok {
		return User{}, nil, ErrSessionInvalid
	}
	now := s.now()
	row, err := s.repo.q.GetActiveSession(ctx, authdb.GetActiveSessionParams{ID: id, ExpiresAt: now})
	if err := mapErr(err); errors.Is(err, errNotFound) {
		return User{}, nil, ErrSessionInvalid
	} else if err != nil {
		return User{}, nil, fmt.Errorf("auth: get session: %w", err)
	}

	if row.Session.ExpiresAt.Sub(now) < SessionTTL-sessionRefresh {
		newExp := now.Add(SessionTTL)
		if err := s.repo.q.ExtendSession(ctx, authdb.ExtendSessionParams{ID: id, ExpiresAt: newExp, LastSeenAt: now}); err != nil {
			s.log.WarnContext(ctx, "auth: extend session gagal", slog.String("error", err.Error()))
		} else {
			extendedUntil = &newExp
		}
	}
	return toUser(row.User), extendedUntil, nil
}

// Logout menghapus session; token tidak bisa dipakai lagi.
func (s *Service) Logout(ctx context.Context, token string) error {
	id, ok := parseToken(token)
	if !ok {
		return nil
	}
	return s.repo.q.DeleteSession(ctx, id)
}

// DeleteExpired membersihkan session & token reset yang sudah kedaluwarsa.
func (s *Service) DeleteExpired(ctx context.Context) (int64, error) {
	now := s.now()
	n1, err := s.repo.q.DeleteExpiredSessions(ctx, now)
	if err != nil {
		return 0, err
	}
	n2, err := s.repo.q.DeleteExpiredPasswordResetTokens(ctx, now)
	return n1 + n2, err
}

// RunCleanup menjalankan DeleteExpired berkala sampai ctx selesai.
func (s *Service) RunCleanup(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := s.DeleteExpired(ctx); err != nil {
				s.log.WarnContext(ctx, "auth: cleanup gagal", slog.String("error", err.Error()))
			} else if n > 0 {
				s.log.InfoContext(ctx, "auth: cleanup", slog.Int64("deleted", n))
			}
		}
	}
}

// ---------- Reset password ----------

// RequestPasswordReset mengirim link reset bila email terdaftar. Selalu nil untuk
// email yang tidak terdaftar (tidak membocorkan keberadaan akun).
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	row, err := s.repo.q.GetUserByEmail(ctx, normalizeEmail(email))
	if err := mapErr(err); errors.Is(err, errNotFound) {
		return nil
	} else if err != nil {
		return fmt.Errorf("auth: get user: %w", err)
	}

	token, err := newToken()
	if err != nil {
		return err
	}
	if err := s.repo.q.CreatePasswordResetToken(ctx, authdb.CreatePasswordResetTokenParams{
		ID: hashToken(token), UserID: row.ID, ExpiresAt: s.now().Add(ResetTokenTTL),
	}); err != nil {
		return fmt.Errorf("auth: create reset token: %w", err)
	}

	link := s.baseURL + "/reset-password?token=" + token
	return s.mailer.Send(ctx, platformmail.Message{
		To:      row.Email,
		Subject: "Reset password Lunovia",
		Text: fmt.Sprintf("Halo %s,\n\nKlik link berikut untuk membuat password baru (berlaku 1 jam):\n%s\n\n"+
			"Abaikan email ini bila kamu tidak meminta reset password.\n\n— Lunovia", row.Name, link),
	})
}

// ResetPassword memakai token (sekali pakai, maks. 1 jam) untuk mengganti password.
// Semua session user dihapus sehingga perangkat lain ikut logout.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if err := validate(map[string]string{"password": password}); err != nil {
		return err
	}
	id, ok := parseToken(token)
	if !ok {
		return ErrInvalidResetToken
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.repo.inTx(ctx, func(q *authdb.Queries) error {
		userID, err := q.ConsumePasswordResetToken(ctx, authdb.ConsumePasswordResetTokenParams{ID: id, UsedAt: ptr(s.now())})
		if err := mapErr(err); errors.Is(err, errNotFound) {
			return ErrInvalidResetToken
		} else if err != nil {
			return err
		}
		if err := q.UpdateUserPassword(ctx, authdb.UpdateUserPasswordParams{ID: userID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := q.DeleteUserSessions(ctx, userID); err != nil {
			return err
		}
		return q.DeleteUserPasswordResetTokens(ctx, userID)
	})
}

// ---------- Token ----------

func newToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// parseToken memvalidasi bentuk token sebelum menyentuh DB.
func parseToken(token string) ([]byte, bool) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != tokenBytes {
		return nil, false
	}
	return hashToken(token), true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func ptr[T any](v T) *T { return &v }

// ---------- Admin (T16) ----------

// UserPage adalah satu halaman hasil pencarian user.
type UserPage struct {
	Users   []User
	Total   int
	Page    int
	PerPage int
}

func (p UserPage) Pages() int { return max(1, (p.Total+p.PerPage-1)/p.PerPage) }

// SearchUsers: daftar user untuk panel admin (cari nama/email), halaman mulai 1.
func (s *Service) SearchUsers(ctx context.Context, q string, page, perPage int) (UserPage, error) {
	var qp *string
	if q = strings.TrimSpace(q); q != "" {
		q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
		qp = &q
	}
	page = max(1, page)
	total, err := s.repo.q.CountSearchUsers(ctx, qp)
	if err != nil {
		return UserPage{}, err
	}
	rows, err := s.repo.q.SearchUsers(ctx, authdb.SearchUsersParams{Q: qp, Lim: int32(perPage), Off: int32((page - 1) * perPage)}) //nolint:gosec // G115: halaman kecil
	if err != nil {
		return UserPage{}, err
	}
	out := UserPage{Users: make([]User, len(rows)), Total: int(total), Page: page, PerPage: perPage}
	for i, r := range rows {
		out.Users[i] = toUser(r)
	}
	return out, nil
}

// GetUser mengembalikan user berdasarkan ID (ErrUserNotFound bila tidak ada).
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := s.repo.q.GetUserByID(ctx, id)
	if err := mapErr(err); errors.Is(err, errNotFound) {
		return User{}, ErrUserNotFound
	} else if err != nil {
		return User{}, err
	}
	return toUser(row), nil
}

// SetDisabled menonaktifkan (dan mengeluarkan dari semua sesi) atau
// mengaktifkan kembali akun.
func (s *Service) SetDisabled(ctx context.Context, id uuid.UUID, disabled bool) (User, error) {
	var at *time.Time
	if disabled {
		now := s.now()
		at = &now
	}
	row, err := s.repo.q.SetUserDisabled(ctx, authdb.SetUserDisabledParams{ID: id, DisabledAt: at})
	if err := mapErr(err); errors.Is(err, errNotFound) {
		return User{}, ErrUserNotFound
	} else if err != nil {
		return User{}, err
	}
	if disabled {
		if err := s.repo.q.DeleteUserSessions(ctx, id); err != nil {
			return User{}, err
		}
	}
	return toUser(row), nil
}
