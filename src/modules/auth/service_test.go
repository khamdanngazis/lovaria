package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	platformmail "github.com/khamdanngazis/lovaria/src/platform/mail"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

// fakeMailer menyimpan email terkirim untuk diperiksa test.
type fakeMailer struct {
	mu   sync.Mutex
	sent []platformmail.Message
}

func (f *fakeMailer) Send(_ context.Context, m platformmail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeMailer) last(t *testing.T) platformmail.Message {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.sent) == 0 {
		t.Fatal("tidak ada email terkirim")
	}
	return f.sent[len(f.sent)-1]
}

// clock adalah jam palsu yang bisa dimajukan.
type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func newTestService(t *testing.T) (*Service, *fakeMailer, *clock) {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	m := &fakeMailer{}
	clk := &clock{t: time.Now().Truncate(time.Second)}
	svc := NewService(NewRepository(pool), m, "https://lovoria.test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.now = clk.now
	return svc, m, clk
}

var ctx = context.Background()

func mustRegister(t *testing.T, svc *Service, email string) User {
	t.Helper()
	u, err := svc.Register(ctx, RegisterInput{Name: "Sarah", Email: email, Password: "password123"})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return u
}

func TestRegister(t *testing.T) {
	svc, _, _ := newTestService(t)

	u := mustRegister(t, svc, "  Sarah@Example.COM ")
	if u.Email != "sarah@example.com" || u.Role != RoleCouple || u.ID.Version() != 7 {
		t.Errorf("user = %+v", u)
	}

	_, err := svc.Register(ctx, RegisterInput{Name: "Lain", Email: "SARAH@example.com", Password: "password123"})
	var v ValidationError
	if !errors.As(err, &v) || v["email"] != "Email sudah terdaftar" {
		t.Errorf("email duplikat (case-insensitive): err = %v", err)
	}

	_, err = svc.Register(ctx, RegisterInput{Name: "", Email: "bukan-email", Password: "pendek"})
	if !errors.As(err, &v) || v["name"] == "" || v["email"] == "" || v["password"] == "" {
		t.Errorf("validasi: err = %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	svc, _, _ := newTestService(t)
	mustRegister(t, svc, "sarah@example.com")

	if u, err := svc.Authenticate(ctx, "SARAH@example.com", "password123"); err != nil || u.Email != "sarah@example.com" {
		t.Fatalf("login benar: %v", err)
	}
	for _, c := range [][2]string{{"sarah@example.com", "salah-banget"}, {"tidakada@example.com", "password123"}} {
		if _, err := svc.Authenticate(ctx, c[0], c[1]); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("%v: err = %v, want ErrInvalidCredentials", c, err)
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	svc, _, clk := newTestService(t)
	u := mustRegister(t, svc, "sarah@example.com")

	token, exp, err := svc.CreateSession(ctx, u.ID, "1.2.3.4", "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if !exp.Equal(clk.now().Add(SessionTTL)) {
		t.Errorf("expires = %v", exp)
	}

	got, extended, err := svc.SessionUser(ctx, token)
	if err != nil || got.ID != u.ID || extended != nil {
		t.Fatalf("session baru: user=%v extended=%v err=%v", got.ID, extended, err)
	}

	// Rolling expiry: setelah > 1 hari, session diperpanjang 30 hari dari sekarang.
	clk.advance(25 * time.Hour)
	_, extended, err = svc.SessionUser(ctx, token)
	if err != nil || extended == nil || !extended.Equal(clk.now().Add(SessionTTL)) {
		t.Fatalf("rolling: extended=%v err=%v", extended, err)
	}

	// Tanpa aktivitas > 30 hari → kedaluwarsa.
	clk.advance(SessionTTL + time.Minute)
	if _, _, err := svc.SessionUser(ctx, token); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("expired: err = %v", err)
	}
	if n, err := svc.DeleteExpired(ctx); err != nil || n != 1 {
		t.Errorf("cleanup: n=%d err=%v", n, err)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	svc, _, _ := newTestService(t)
	u := mustRegister(t, svc, "sarah@example.com")
	token, _, _ := svc.CreateSession(ctx, u.ID, "", "")

	if err := svc.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SessionUser(ctx, token); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("session setelah logout: err = %v", err)
	}
	for _, bad := range []string{"", "abc", "!!!!"} {
		if _, _, err := svc.SessionUser(ctx, bad); !errors.Is(err, ErrSessionInvalid) {
			t.Errorf("token %q: err = %v", bad, err)
		}
	}
}

var resetLinkRe = regexp.MustCompile(`https://lovoria\.test/reset-password\?token=([A-Za-z0-9_-]+)`)

func requestResetToken(t *testing.T, svc *Service, m *fakeMailer, email string) string {
	t.Helper()
	if err := svc.RequestPasswordReset(ctx, email); err != nil {
		t.Fatal(err)
	}
	match := resetLinkRe.FindStringSubmatch(m.last(t).Text)
	if match == nil {
		t.Fatalf("link reset tidak ada di email: %s", m.last(t).Text)
	}
	return match[1]
}

func TestPasswordReset(t *testing.T) {
	svc, m, _ := newTestService(t)
	u := mustRegister(t, svc, "sarah@example.com")
	session, _, _ := svc.CreateSession(ctx, u.ID, "", "")

	// Email tidak terdaftar: tidak error, tidak kirim email.
	if err := svc.RequestPasswordReset(ctx, "tidakada@example.com"); err != nil || len(m.sent) != 0 {
		t.Fatalf("email tidak terdaftar: err=%v sent=%d", err, len(m.sent))
	}

	token := requestResetToken(t, svc, m, "sarah@example.com")
	if m.last(t).To != "sarah@example.com" {
		t.Errorf("to = %s", m.last(t).To)
	}

	var v ValidationError
	if err := svc.ResetPassword(ctx, token, "pendek"); !errors.As(err, &v) {
		t.Fatalf("password pendek: err = %v", err)
	}
	if err := svc.ResetPassword(ctx, token, "password-baru-123"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "sarah@example.com", "password-baru-123"); err != nil {
		t.Errorf("login dengan password baru: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "sarah@example.com", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("password lama masih berlaku: %v", err)
	}
	if _, _, err := svc.SessionUser(ctx, session); !errors.Is(err, ErrSessionInvalid) {
		t.Errorf("session lama harus dihapus setelah reset: %v", err)
	}

	// Token sekali pakai.
	if err := svc.ResetPassword(ctx, token, "password-lain-123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("token dipakai 2x: err = %v", err)
	}
}

func TestPasswordResetTokenExpires(t *testing.T) {
	svc, m, clk := newTestService(t)
	mustRegister(t, svc, "sarah@example.com")
	token := requestResetToken(t, svc, m, "sarah@example.com")

	clk.advance(ResetTokenTTL + time.Second)
	if err := svc.ResetPassword(ctx, token, "password-baru-123"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("token > 1 jam: err = %v", err)
	}

	// Masih dalam 1 jam → berhasil.
	token = requestResetToken(t, svc, m, "sarah@example.com")
	clk.advance(ResetTokenTTL - time.Minute)
	if err := svc.ResetPassword(ctx, token, "password-baru-123"); err != nil {
		t.Errorf("token < 1 jam: %v", err)
	}
}

func TestCreateAdminAndSeeder(t *testing.T) {
	svc, _, _ := newTestService(t)
	u, err := svc.CreateAdmin(ctx, RegisterInput{Name: "Admin", Email: "admin@example.com", Password: "password123"})
	if err != nil || u.Role != RoleAdmin {
		t.Fatalf("admin: %+v %v", u, err)
	}

	s := Seeder(svc)
	for i := 0; i < 2; i++ { // idempoten
		if err := s.Run(ctx); err != nil {
			t.Fatalf("seed run %d: %v", i, err)
		}
	}
	if a, err := svc.Authenticate(ctx, "admin@lovoria.test", DevPassword); err != nil || a.Role != RoleAdmin {
		t.Errorf("seed admin: %+v %v", a, err)
	}
}

func TestValidateField(t *testing.T) {
	cases := []struct{ field, value, want string }{
		{"email", "a@b.co", ""},
		{"email", "Nama <a@b.co>", "Format email tidak valid"},
		{"email", "a@localhost", "Format email tidak valid"},
		{"email", "", "Email wajib diisi"},
		{"name", "  ", "Nama wajib diisi"},
		{"password", "1234567", "Password minimal 8 karakter"},
		{"password", "12345678", ""},
	}
	for _, c := range cases {
		if got := ValidateField(c.field, c.value); got != c.want {
			t.Errorf("%s=%q: got %q want %q", c.field, c.value, got, c.want)
		}
	}
}

func TestSafeNext(t *testing.T) {
	cases := map[string]string{
		"":                    "/dashboard",
		"/dashboard/weddings": "/dashboard/weddings",
		"//evil.com":          "/dashboard",
		"/\\evil.com":         "/dashboard",
		"https://evil.com/x":  "/dashboard",
		"/admin?tab=1":        "/admin?tab=1",
	}
	for in, want := range cases {
		if got := safeNext(in); got != want {
			t.Errorf("safeNext(%q) = %q, want %q", in, got, want)
		}
	}
}
