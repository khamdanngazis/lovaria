package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/server"
)

// fakeGoogle meniru endpoint token Google: menukar kode apa pun dengan ID token
// berisi klaim yang diatur test, dan mencatat form yang diterimanya.
type fakeGoogle struct {
	srv    *httptest.Server
	claims map[string]any
	form   url.Values
	status int
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	f := &fakeGoogle{status: http.StatusOK}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.form = r.PostForm
		if f.status != http.StatusOK {
			w.WriteHeader(f.status)
			return
		}
		payload, _ := json.Marshal(f.claims)
		tok := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".tanda-tangan"
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at", "id_token": tok})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGoogle) client() *Google {
	g := NewGoogle("client-id", "client-secret", "https://lovoria.test")
	g.TokenURL, g.Client = f.srv.URL, f.srv.Client()
	return g
}

func (f *fakeGoogle) set(sub, email, nonce string) {
	f.claims = map[string]any{
		"iss": "https://accounts.google.com", "aud": "client-id", "sub": sub, "email": email, "email_verified": true,
		"name": "Sarah Putri", "nonce": nonce, "exp": time.Now().Add(time.Hour).Unix(),
	}
}

func TestNewGoogleRequiresBothCredentials(t *testing.T) {
	if NewGoogle("", "s", "https://x") != nil || NewGoogle("id", "", "https://x") != nil {
		t.Error("tanpa kredensial lengkap fitur harus nonaktif")
	}
	g := NewGoogle("id", "s", "https://lunovia.id/")
	if g == nil || g.RedirectURL != "https://lunovia.id/auth/google/callback" {
		t.Fatalf("redirect URL: %+v", g)
	}
	u, _ := url.Parse(g.AuthCodeURL("st", "nc", "verifier"))
	q := u.Query()
	if u.Host != "accounts.google.com" || q.Get("client_id") != "id" || q.Get("state") != "st" || q.Get("nonce") != "nc" || q.Get("response_type") != "code" ||
		q.Get("scope") != "openid email profile" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") != pkceChallenge("verifier") || q.Get("code_challenge") == "verifier" {
		t.Errorf("auth URL: %s", u)
	}
}

func TestGoogleExchangeValidatesClaims(t *testing.T) {
	f := newFakeGoogle(t)
	g := f.client()
	f.set("sub-1", "Sarah@Gmail.com", "nonce-1")
	p, err := g.Exchange(ctx, "kode", "verifier", "nonce-1")
	if err != nil || p.Subject != "sub-1" || p.Email != "sarah@gmail.com" || p.Name != "Sarah Putri" {
		t.Fatalf("exchange: %+v %v", p, err)
	}
	if f.form.Get("code") != "kode" || f.form.Get("code_verifier") != "verifier" || f.form.Get("client_secret") != "client-secret" || f.form.Get("redirect_uri") != "https://lovoria.test/auth/google/callback" {
		t.Errorf("form token: %v", f.form)
	}
	bad := map[string]func(){
		"audience lain":        func() { f.claims["aud"] = "aplikasi-lain" },
		"penerbit lain":        func() { f.claims["iss"] = "https://evil.example" },
		"kedaluwarsa":          func() { f.claims["exp"] = time.Now().Add(-time.Minute).Unix() },
		"nonce tidak cocok":    func() { f.claims["nonce"] = "nonce-lain" },
		"tanpa subject":        func() { f.claims["sub"] = "" },
		"endpoint token 400":   func() { f.status = http.StatusBadRequest },
		"email belum verified": func() { f.claims["email_verified"] = false },
	}
	for name, mutate := range bad {
		f.set("sub-1", "sarah@gmail.com", "nonce-1")
		f.status = http.StatusOK
		mutate()
		if _, err := g.Exchange(ctx, "kode", "verifier", "nonce-1"); err == nil {
			t.Errorf("%s: harus ditolak", name)
		} else if name == "email belum verified" && !errors.Is(err, ErrGoogleUnverified) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Google kadang mengirim email_verified sebagai string.
	f.set("sub-1", "sarah@gmail.com", "nonce-1")
	f.status = http.StatusOK
	f.claims["email_verified"] = "true"
	if _, err := g.Exchange(ctx, "kode", "verifier", "nonce-1"); err != nil {
		t.Errorf("email_verified string: %v", err)
	}
}

func TestLoginWithGoogleCreatesLinksAndProtects(t *testing.T) {
	svc, _, _ := newTestService(t)

	// Akun baru: dibuat, email terverifikasi, password tidak bisa ditebak.
	u, reset, err := svc.LoginWithGoogle(ctx, GoogleProfile{Subject: "g-1", Email: "baru@gmail.com", Name: "Baru"})
	if err != nil || reset || u.Email != "baru@gmail.com" || u.Name != "Baru" || u.Role != RoleCouple {
		t.Fatalf("akun baru: %+v reset=%v err=%v", u, reset, err)
	}
	if _, err := svc.Authenticate(ctx, "baru@gmail.com", ""); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("akun Google tanpa password: %v", err)
	}
	// Masuk lagi → user yang sama, walau email di Google berubah.
	again, _, err := svc.LoginWithGoogle(ctx, GoogleProfile{Subject: "g-1", Email: "ganti@gmail.com", Name: "Baru"})
	if err != nil || again.ID != u.ID {
		t.Fatalf("masuk ulang: %+v %v", again, err)
	}

	// Akun lama dibuat dengan password (email belum terverifikasi), mungkin oleh
	// orang lain: saat pemilik email masuk lewat Google, password lama & sesi
	// lama dimatikan.
	old := mustRegister(t, svc, "lama@gmail.com")
	token, _, err := svc.CreateSession(ctx, old.ID, "1.1.1.1", "ua")
	if err != nil {
		t.Fatal(err)
	}
	linked, reset, err := svc.LoginWithGoogle(ctx, GoogleProfile{Subject: "g-2", Email: "lama@gmail.com", Name: "Lama"})
	if err != nil || !reset || linked.ID != old.ID {
		t.Fatalf("hubungkan akun lama: %+v reset=%v err=%v", linked, reset, err)
	}
	if _, err := svc.Authenticate(ctx, "lama@gmail.com", "password123"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("password lama harus mati setelah dihubungkan: %v", err)
	}
	if _, _, err := svc.SessionUser(ctx, token); err == nil {
		t.Error("sesi lama harus dicabut setelah dihubungkan")
	}
	// Sudah terhubung & terverifikasi: masuk berikutnya tidak mereset apa pun.
	if _, reset, err := svc.LoginWithGoogle(ctx, GoogleProfile{Subject: "g-2", Email: "lama@gmail.com"}); err != nil || reset {
		t.Errorf("masuk kedua: reset=%v err=%v", reset, err)
	}

	// Akun dinonaktifkan admin tidak bisa masuk lewat Google (terhubung maupun belum).
	if _, err := svc.SetDisabled(ctx, old.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.LoginWithGoogle(ctx, GoogleProfile{Subject: "g-2", Email: "lama@gmail.com"}); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("akun nonaktif (terhubung): %v", err)
	}
	blocked := mustRegister(t, svc, "blokir@gmail.com")
	if _, err := svc.SetDisabled(ctx, blocked.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.LoginWithGoogle(ctx, GoogleProfile{Subject: "g-3", Email: "blokir@gmail.com"}); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("akun nonaktif (belum terhubung): %v", err)
	}
}

func newGoogleApp(t *testing.T, g *Google) (*httptest.Server, *Service) {
	t.Helper()
	svc, _, _ := newTestService(t)
	cfg, _ := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := server.New(cfg, log)
	mw := NewMiddleware(svc, false, log)
	e.Use(mw.LoadSession)
	Register(e, Deps{Service: svc, Middleware: mw, Google: g, Secret: []byte("rahasia-test-rahasia-test-rahasia"), RateLimit: RateLimit{PerMinute: 6000, Burst: 1000}})
	e.Group("/dashboard", mw.RequireAuth).GET("", func(c echo.Context) error {
		u, _ := CurrentUser(c.Request().Context())
		return c.String(http.StatusOK, "halo "+u.Email)
	})
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return srv, svc
}

func TestGoogleSignInFlow(t *testing.T) {
	// Tanpa konfigurasi: tombol tidak ada, route 404.
	plain, _ := newGoogleApp(t, nil)
	pc := newClient(t, plain)
	if _, body := pc.do(http.MethodGet, "/login", nil, nil); strings.Contains(body, "Lanjutkan dengan Google") {
		t.Error("tanpa kredensial tombol Google tidak boleh tampil")
	}
	if resp, _ := pc.do(http.MethodGet, "/auth/google", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("route Google tanpa konfigurasi: %d", resp.StatusCode)
	}

	f := newFakeGoogle(t)
	srv, svc := newGoogleApp(t, f.client())
	c := newClient(t, srv)
	const next = "/dashboard/weddings/new?tema=jawa"
	for _, p := range []string{"/login", "/register"} {
		if _, body := c.do(http.MethodGet, p+"?next="+url.QueryEscape(next), nil, nil); !strings.Contains(body, "Lanjutkan dengan Google") || !strings.Contains(body, `href="/auth/google?next=%2Fdashboard%2Fweddings%2Fnew%3Ftema%3Djawa"`) {
			t.Errorf("%s: tombol Google dengan next", p)
		}
	}

	// Mulai: cookie state + redirect ke Google dengan state, nonce, PKCE.
	start := func(cl *client, next string) url.Values {
		t.Helper()
		resp, _ := cl.do(http.MethodGet, "/auth/google?next="+url.QueryEscape(next), nil, nil)
		loc, _ := url.Parse(resp.Header.Get("Location"))
		if resp.StatusCode != http.StatusSeeOther || loc.Host != "accounts.google.com" || loc.Query().Get("state") == "" || loc.Query().Get("code_challenge") == "" {
			t.Fatalf("start: %d %s", resp.StatusCode, loc)
		}
		return loc.Query()
	}
	q := start(c, next)

	// State tidak cocok (login CSRF) → gagal, tidak ada sesi.
	resp, _ := c.do(http.MethodGet, "/auth/google/callback?code=x&state=palsu", nil, nil)
	if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(resp.Header.Get("Location"), "/login?") || !strings.Contains(resp.Header.Get("Location"), "google=gagal") {
		t.Fatalf("state palsu: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if _, body := c.do(http.MethodGet, "/login?google=gagal", nil, nil); !strings.Contains(body, "Masuk dengan Google tidak berhasil") {
		t.Error("pesan gagal di halaman masuk")
	}
	// Tanpa cookie state sama sekali (callback dibuka langsung) → gagal.
	if resp, _ := newClient(t, srv).do(http.MethodGet, "/auth/google/callback?code=x&state="+q.Get("state"), nil, nil); !strings.Contains(resp.Header.Get("Location"), "google=gagal") {
		t.Errorf("tanpa cookie state: %s", resp.Header.Get("Location"))
	}
	// Pengguna membatalkan di halaman Google → kembali ke masuk dengan next terjaga.
	q = start(c, next)
	resp, _ = c.do(http.MethodGet, "/auth/google/callback?error=access_denied&state="+q.Get("state"), nil, nil)
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "google=gagal") || !strings.Contains(loc, "next=%2Fdashboard%2Fweddings%2Fnew") {
		t.Errorf("batal: %s", loc)
	}

	// Berhasil: akun baru dibuat, sesi dimulai, menuju next.
	q = start(c, next)
	f.set("g-100", "pengantin@gmail.com", q.Get("nonce"))
	resp, _ = c.do(http.MethodGet, "/auth/google/callback?code=kode-sah&state="+q.Get("state"), nil, nil)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != next {
		t.Fatalf("callback: %d → %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	if f.form.Get("code") != "kode-sah" || pkceChallenge(f.form.Get("code_verifier")) != q.Get("code_challenge") {
		t.Error("kode & PKCE verifier harus dikirim ke endpoint token")
	}
	if _, body := c.do(http.MethodGet, "/dashboard", nil, nil); body != "halo pengantin@gmail.com" {
		t.Fatalf("sesi setelah Google: %q", body)
	}
	// State sekali pakai: callback yang sama diulang tidak berlaku lagi.
	if resp, _ := newClient(t, srv).do(http.MethodGet, "/auth/google/callback?code=kode-sah&state="+q.Get("state"), nil, nil); !strings.Contains(resp.Header.Get("Location"), "google=gagal") {
		t.Error("callback tanpa cookie state harus gagal")
	}
	// Tujuan ke luar situs diabaikan.
	other := newClient(t, srv)
	q = start(other, "//evil.example/x")
	f.set("g-100", "pengantin@gmail.com", q.Get("nonce"))
	if resp, _ := other.do(http.MethodGet, "/auth/google/callback?code=k&state="+q.Get("state"), nil, nil); resp.Header.Get("Location") != "/dashboard" {
		t.Errorf("next luar situs: %q", resp.Header.Get("Location"))
	}

	// Akun lama ber-password dihubungkan → halaman pemberitahuan, sudah masuk.
	mustRegister(t, svc, "lama@gmail.com")
	linker := newClient(t, srv)
	q = start(linker, "")
	f.set("g-200", "lama@gmail.com", q.Get("nonce"))
	resp, body := linker.do(http.MethodGet, "/auth/google/callback?code=k&state="+q.Get("state"), nil, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "terhubung dengan Google") || !strings.Contains(body, "password lama akun ini dinonaktifkan") || !strings.Contains(body, `href="/dashboard"`) {
		t.Fatalf("halaman terhubung: %d", resp.StatusCode)
	}
	if _, body := linker.do(http.MethodGet, "/dashboard", nil, nil); body != "halo lama@gmail.com" {
		t.Errorf("sesi setelah dihubungkan: %q", body)
	}
	// Email Google belum terverifikasi → ditolak.
	unv := newClient(t, srv)
	q = start(unv, "")
	f.set("g-300", "belum@contoh.id", q.Get("nonce"))
	f.claims["email_verified"] = false
	if resp, _ := unv.do(http.MethodGet, "/auth/google/callback?code=k&state="+q.Get("state"), nil, nil); !strings.Contains(resp.Header.Get("Location"), "google=gagal") {
		t.Errorf("email belum terverifikasi: %s", resp.Header.Get("Location"))
	}
}
