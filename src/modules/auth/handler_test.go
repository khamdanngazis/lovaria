package auth

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/server"
)

// newTestApp merakit stack HTTP seperti cmd/server/main.go.
func newTestApp(t *testing.T, rl RateLimit) (*httptest.Server, *Service, *fakeMailer) {
	t.Helper()
	svc, m, _ := newTestService(t)
	cfg, err := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := server.New(cfg, log)
	mw := NewMiddleware(svc, false, log)
	e.Use(mw.LoadSession)
	Register(e, Deps{Service: svc, Middleware: mw, RateLimit: rl})

	whoami := func(c echo.Context) error {
		u, _ := CurrentUser(c.Request().Context())
		return c.String(http.StatusOK, "halo "+u.Email)
	}
	e.Group("/dashboard", mw.RequireAuth).GET("", whoami)
	e.Group("/admin", mw.RequireAuth, mw.RequireRole(RoleAdmin)).GET("", whoami)

	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	return srv, svc, m
}

type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, srv *httptest.Server) *client {
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: srv.URL, http: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *client) do(method, path string, form url.Values, headers map[string]string) (*http.Response, string) {
	c.t.Helper()
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		c.t.Fatal(err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

var csrfRe = regexp.MustCompile(`name="_csrf" value="([^"]+)"`)

// csrf mengambil token CSRF dari halaman form (seperti browser tanpa Sec-Fetch-Site).
func (c *client) csrf(page string) string {
	c.t.Helper()
	_, body := c.do(http.MethodGet, page, nil, nil)
	m := csrfRe.FindStringSubmatch(body)
	if m == nil {
		c.t.Fatalf("token CSRF tidak ada di %s", page)
	}
	return m[1]
}

func (c *client) post(path string, form url.Values, htmx bool) (*http.Response, string) {
	c.t.Helper()
	form.Set("_csrf", c.csrf("/forgot-password"))
	h := map[string]string{}
	if htmx {
		h["HX-Request"] = "true"
	}
	return c.do(http.MethodPost, path, form, h)
}

func (c *client) register(email string) {
	c.t.Helper()
	resp, body := c.post("/register", url.Values{"name": {"Sarah"}, "email": {email}, "password": {"password123"}, "password_confirmation": {"password123"}}, false)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/dashboard" {
		c.t.Fatalf("register: %d %s %s", resp.StatusCode, resp.Header.Get("Location"), body)
	}
}

func TestRegisterAutoLoginAndLogout(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{})
	c := newClient(t, srv)

	c.register("sarah@example.com")
	resp, body := c.do(http.MethodGet, "/dashboard", nil, nil)
	if resp.StatusCode != http.StatusOK || body != "halo sarah@example.com" {
		t.Fatalf("dashboard setelah register: %d %q", resp.StatusCode, body)
	}

	// Cookie session: HttpOnly + SameSite=Lax.
	u, _ := url.Parse(srv.URL)
	var session *http.Cookie
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == SessionCookie {
			session = ck
		}
	}
	if session == nil {
		t.Fatal("cookie session tidak ada")
	}

	// Logout → session tidak bisa dipakai ulang, walau cookie lama dikirim lagi.
	resp, _ = c.post("/logout", url.Values{}, false)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/login" {
		t.Fatalf("logout: %d", resp.StatusCode)
	}
	replay := newClient(t, srv)
	replay.http.Jar.SetCookies(u, []*http.Cookie{{Name: SessionCookie, Value: session.Value}})
	resp, _ = replay.do(http.MethodGet, "/dashboard", nil, nil)
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("cookie lama setelah logout: %d, want 303 ke login", resp.StatusCode)
	}
}

func TestSessionCookieAttributes(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{})
	c := newClient(t, srv)
	form := url.Values{"name": {"S"}, "email": {"s@example.com"}, "password": {"password123"}, "password_confirmation": {"password123"}, "_csrf": {c.csrf("/register")}}
	resp, _ := c.do(http.MethodPost, "/register", form, nil)
	var raw string
	for _, h := range resp.Header.Values("Set-Cookie") {
		if strings.HasPrefix(h, SessionCookie+"=") {
			raw = h
		}
	}
	for _, want := range []string{"HttpOnly", "SameSite=Lax", "Path=/", "Max-Age="} {
		if !strings.Contains(raw, want) {
			t.Errorf("Set-Cookie %q tidak memuat %s", raw, want)
		}
	}
}

func TestDashboardRequiresAuth(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{})
	c := newClient(t, srv)

	for _, p := range []string{"/dashboard", "/dashboard/weddings/123"} {
		resp, _ := c.do(http.MethodGet, p, nil, nil)
		want := "/login?next=" + url.QueryEscape(p)
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != want {
			t.Errorf("%s: %d %s, want 303 %s", p, resp.StatusCode, resp.Header.Get("Location"), want)
		}
	}
	resp, _ := c.do(http.MethodGet, "/dashboard", nil, map[string]string{"HX-Request": "true"})
	if resp.Header.Get("HX-Redirect") == "" {
		t.Error("request htmx harus mendapat HX-Redirect")
	}
}

func TestAdminRequiresAdminRole(t *testing.T) {
	srv, svc, _ := newTestApp(t, RateLimit{})

	couple := newClient(t, srv)
	couple.register("couple@example.com")
	if resp, _ := couple.do(http.MethodGet, "/admin", nil, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("couple → /admin: %d, want 403", resp.StatusCode)
	}

	if _, err := svc.CreateAdmin(ctx, RegisterInput{Name: "A", Email: "admin@example.com", Password: "password123"}); err != nil {
		t.Fatal(err)
	}
	admin := newClient(t, srv)
	resp, _ := admin.post("/login", url.Values{"email": {"admin@example.com"}, "password": {"password123"}, "next": {"/admin"}}, false)
	if resp.Header.Get("Location") != "/admin" {
		t.Fatalf("login admin: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	if resp, _ := admin.do(http.MethodGet, "/admin", nil, nil); resp.StatusCode != http.StatusOK {
		t.Errorf("admin → /admin: %d", resp.StatusCode)
	}
}

func TestLoginErrorsAreGeneric(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{})
	c := newClient(t, srv)
	c.register("sarah@example.com")
	c.post("/logout", url.Values{}, false)

	for _, email := range []string{"sarah@example.com", "tidakada@example.com"} {
		resp, body := c.post("/login", url.Values{"email": {email}, "password": {"salah-banget"}}, true)
		if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "email atau password salah") {
			t.Errorf("%s: %d %s", email, resp.StatusCode, body)
		}
		if strings.Contains(body, "<html") {
			t.Error("request htmx harus mendapat fragment, bukan halaman penuh")
		}
	}

	// Open redirect ditolak.
	resp, _ := c.post("/login", url.Values{"email": {"sarah@example.com"}, "password": {"password123"}, "next": {"//evil.com"}}, true)
	if resp.Header.Get("HX-Redirect") != "/dashboard" {
		t.Errorf("next=//evil.com → %q", resp.Header.Get("HX-Redirect"))
	}
}

func TestCSRFRejectedWithoutToken(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{})
	c := newClient(t, srv)
	form := url.Values{"name": {"S"}, "email": {"s@example.com"}, "password": {"password123"}, "password_confirmation": {"password123"}}

	if resp, _ := c.do(http.MethodPost, "/register", form, nil); resp.StatusCode != http.StatusForbidden {
		t.Errorf("tanpa token: %d, want 403", resp.StatusCode)
	}
	if resp, _ := c.do(http.MethodPost, "/login", url.Values{"email": {"x@y.z"}, "password": {"x"}}, map[string]string{"Sec-Fetch-Site": "cross-site"}); resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site: %d, want 403", resp.StatusCode)
	}
	form.Set("_csrf", c.csrf("/register"))
	if resp, _ := c.do(http.MethodPost, "/register", form, nil); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("dengan token: %d, want 303", resp.StatusCode)
	}
}

func TestRegisterValidationAndInline(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{})
	c := newClient(t, srv)

	resp, body := c.post("/register", url.Values{"name": {""}, "email": {"bukan"}, "password": {"123"}}, true)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	for _, want := range []string{"Nama wajib diisi", "Format email tidak valid", "Password minimal 8 karakter", `value="bukan"`} {
		if !strings.Contains(body, want) {
			t.Errorf("form tidak memuat %q", want)
		}
	}
	if strings.Contains(body, `value="123"`) {
		t.Error("password tidak boleh dirender ulang")
	}

	// Konfirmasi password (T23): tidak sama / kosong → ditolak, akun tidak dibuat,
	// dan pesan kolom lain tetap tampil bersamaan.
	resp, body = c.post("/register", url.Values{"name": {"Sarah"}, "email": {"bukan"}, "password": {"password123"}, "password_confirmation": {"password124"}}, true)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "Konfirmasi password tidak sama") || !strings.Contains(body, "Format email tidak valid") {
		t.Errorf("konfirmasi beda: %d", resp.StatusCode)
	}
	if strings.Contains(body, "password124") || strings.Contains(body, `value="password123"`) {
		t.Error("password / konfirmasi tidak boleh dirender ulang")
	}
	if resp, body := c.post("/register", url.Values{"name": {"Sarah"}, "email": {"sarah@example.com"}, "password": {"password123"}}, true); resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "Konfirmasi password tidak sama") {
		t.Errorf("tanpa konfirmasi: %d", resp.StatusCode)
	}
	if resp, _ := c.post("/login", url.Values{"email": {"sarah@example.com"}, "password": {"password123"}}, true); resp.StatusCode != http.StatusUnprocessableEntity {
		t.Error("akun tidak boleh dibuat bila konfirmasi salah")
	}
	// Validasi inline konfirmasi membandingkan dengan kolom password (hx-include).
	_, page := c.do(http.MethodGet, "/register", nil, nil)
	if !strings.Contains(page, `id="register-password_confirmation"`) || !strings.Contains(page, `hx-include="#register-password"`) {
		t.Error("kolom konfirmasi password tidak ada di form")
	}
	if _, body := c.post("/register/validate", url.Values{"field": {"password_confirmation"}, "password": {"password123"}, "password_confirmation": {"password12"}}, true); !strings.Contains(body, "Konfirmasi password tidak sama") {
		t.Errorf("inline beda: %s", body)
	}
	if _, body := c.post("/register/validate", url.Values{"field": {"password_confirmation"}, "password": {"password123"}, "password_confirmation": {"password123"}}, true); strings.Contains(body, "tidak sama") || !strings.Contains(body, `id="register-password_confirmation-error"`) {
		t.Errorf("inline sama: %s", body)
	}

	resp, body = c.post("/register/validate", url.Values{"field": {"email"}, "email": {"x"}}, true)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `id="register-email-error"`) || !strings.Contains(body, "Format email tidak valid") {
		t.Errorf("validate: %d %s", resp.StatusCode, body)
	}
}

func TestForgotAndResetFlow(t *testing.T) {
	srv, _, m := newTestApp(t, RateLimit{})
	c := newClient(t, srv)
	c.register("sarah@example.com")

	resp, body := c.post("/forgot-password", url.Values{"email": {"sarah@example.com"}}, false)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "Jika email tersebut terdaftar") {
		t.Fatalf("forgot: %d", resp.StatusCode)
	}
	resp2, body2 := c.post("/forgot-password", url.Values{"email": {"tidakada@example.com"}}, false)
	if resp2.StatusCode != resp.StatusCode || !strings.Contains(body2, "Jika email tersebut terdaftar") {
		t.Error("respons untuk email tidak terdaftar harus identik")
	}

	token := resetLinkRe.FindStringSubmatch(m.last(t).Text)[1]
	if resp, _ := c.do(http.MethodGet, "/reset-password?token="+token, nil, nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("halaman reset: %d", resp.StatusCode)
	}
	if resp, _ := c.do(http.MethodGet, "/reset-password?token=ngawur", nil, nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("token rusak: %d", resp.StatusCode)
	}

	resp, _ = c.post("/reset-password", url.Values{"token": {token}, "password": {"password-baru-123"}}, false)
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/login?reset=1" {
		t.Fatalf("reset: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	resp, body = c.post("/reset-password", url.Values{"token": {token}, "password": {"password-lain-123"}}, true)
	if resp.StatusCode != http.StatusUnprocessableEntity || !strings.Contains(body, "tidak valid atau sudah kedaluwarsa") {
		t.Errorf("token dipakai 2x: %d", resp.StatusCode)
	}
}

func TestLoginRateLimited(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{PerMinute: 1, Burst: 3})
	c := newClient(t, srv)
	var last int
	for i := 0; i < 4; i++ {
		resp, _ := c.post("/login", url.Values{"email": {"x@example.com"}, "password": {"salah-banget"}}, true)
		last = resp.StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Errorf("percobaan ke-4: %d, want 429", last)
	}
	// Register punya kuota terpisah.
	if resp, _ := c.post("/register", url.Values{"name": {"S"}, "email": {"s@example.com"}, "password": {"password123"}, "password_confirmation": {"password123"}}, false); resp.StatusCode != http.StatusSeeOther {
		t.Errorf("register setelah login dibatasi: %d", resp.StatusCode)
	}
}

// T22: halaman auth memakai kerangka brand — logo menaut ke "/", huruf brand,
// panel tagline, komponen ui-*; poin manfaat hanya di halaman daftar.
func TestAuthPagesUseBrandShell(t *testing.T) {
	srv, _, _ := newTestApp(t, RateLimit{PerMinute: 1, Burst: 1})
	c := newClient(t, srv)
	for _, path := range []string{"/login", "/register", "/forgot-password", "/reset-password?token=tidak-valid"} {
		_, body := c.do(http.MethodGet, path, nil, nil)
		for _, want := range []string{
			`<a href="/" aria-label="Lovoria — beranda"`, "LOVORIA", "Your Forever.", "family=Inter", "Playfair+Display",
			"bg-lovoria-bg", "bg-lovoria-deep", "ui-card", `href="/privacy"`, `href="/terms"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: tidak memuat %q", path, want)
			}
		}
		for _, bad := range []string{"slate-", "bg-primary", "text-primary", "red-600", "green-50", "amber-50"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s: masih memuat kelas lama %q", path, bad)
			}
		}
		if strings.Count(body, "<h1") != 1 {
			t.Errorf("%s: h1 = %d, want 1", path, strings.Count(body, "<h1"))
		}
		if hasBenefits := strings.Contains(body, authBenefits[0]); hasBenefits != (path == "/register") {
			t.Errorf("%s: poin manfaat tampil = %v", path, hasBenefits)
		}
	}
	// id & tombol form tidak berubah (dipakai e2e dan handler test).
	_, login := c.do(http.MethodGet, "/login", nil, nil)
	for _, want := range []string{`id="login-email"`, `id="login-password"`, `class="ui-input"`, `class="ui-btn ui-btn-primary w-full"`, "Lupa password?"} {
		if !strings.Contains(login, want) {
			t.Errorf("login: tidak memuat %q", want)
		}
	}
	// Pesan error, sukses, dan rate limit memakai ui-alert.
	_, body := c.post("/login", url.Values{"email": {"x@example.com"}, "password": {"salah-banget"}}, true)
	if !strings.Contains(body, "ui-alert ui-alert-danger") {
		t.Errorf("error login: %s", body)
	}
	resp, body := c.post("/login", url.Values{"email": {"x@example.com"}, "password": {"salah-banget"}}, true)
	if resp.StatusCode != http.StatusTooManyRequests || !strings.Contains(body, "ui-alert ui-alert-warning") {
		t.Errorf("rate limit: %d %s", resp.StatusCode, body)
	}
}
