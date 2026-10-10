package auth

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type Handler struct {
	svc *Service
	mw  *Middleware
	// google: Sign in with Google (T30); nil = nonaktif. secret menandatangani
	// cookie state OAuth.
	google *Google
	secret []byte
	log    *slog.Logger
}

func NewHandler(svc *Service, mw *Middleware) *Handler {
	return &Handler{svc: svc, mw: mw, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

// render: request htmx mendapat fragment form, selain itu halaman penuh.
func render(c echo.Context, status int, fragment, page templ.Component) error {
	if web.IsHTMX(c) {
		return web.Render(c, status, fragment)
	}
	return web.Render(c, status, page)
}

func formFrom(c echo.Context, fields ...string) form {
	f := form{Values: map[string]string{}, Errors: map[string]string{}}
	for _, name := range fields {
		f.Values[name] = c.FormValue(name)
	}
	return f
}

func (f *form) applyErr(err error) bool {
	var v ValidationError
	if errors.As(err, &v) {
		for k, msg := range v {
			f.Errors[k] = msg
		}
		return true
	}
	return false
}

func (h *Handler) redirectIfLoggedIn(c echo.Context) (bool, error) {
	if _, ok := CurrentUser(c.Request().Context()); ok {
		// Sudah masuk: hormati tujuan (mis. wizard dengan tema pilihan, T29).
		return true, web.Redirect(c, safeNext(c.QueryParam("next")))
	}
	return false, nil
}

func (h *Handler) startSession(c echo.Context, u User, next string) error {
	token, exp, err := h.svc.CreateSession(c.Request().Context(), u.ID, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return err
	}
	h.mw.setCookie(c, token, exp)
	return web.Redirect(c, safeNext(next))
}

// ---------- Login ----------

// GET /login
func (h *Handler) LoginPage(c echo.Context) error {
	if done, err := h.redirectIfLoggedIn(c); done {
		return err
	}
	notice := ""
	if c.QueryParam("reset") == "1" {
		notice = "Password berhasil diganti. Silakan masuk dengan password baru."
	}
	f := form{Next: c.QueryParam("next"), Google: h.google != nil}
	f.Message = googleNotice(c.QueryParam("google"))
	return web.Render(c, http.StatusOK, loginPage(f, notice))
}

// POST /login (form: email, password, next)
func (h *Handler) Login(c echo.Context) error {
	f := formFrom(c, "email")
	f.Next = c.FormValue("next")
	f.Google = h.google != nil
	password := c.FormValue("password")

	if strings.TrimSpace(f.v("email")) == "" || password == "" {
		f.Message = ErrInvalidCredentials.Error()
		return render(c, http.StatusUnprocessableEntity, loginForm(f), loginPage(f, ""))
	}
	u, err := h.svc.Authenticate(c.Request().Context(), f.v("email"), password)
	if errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrAccountDisabled) {
		f.Message = err.Error()
		return render(c, http.StatusUnprocessableEntity, loginForm(f), loginPage(f, ""))
	}
	if err != nil {
		return err
	}
	return h.startSession(c, u, f.Next)
}

// POST /logout
func (h *Handler) Logout(c echo.Context) error {
	if ck, err := c.Cookie(SessionCookie); err == nil {
		if err := h.svc.Logout(c.Request().Context(), ck.Value); err != nil {
			return err
		}
	}
	h.mw.clearCookie(c)
	return web.Redirect(c, "/login")
}

// ---------- Register ----------

// GET /register
func (h *Handler) RegisterPage(c echo.Context) error {
	if done, err := h.redirectIfLoggedIn(c); done {
		return err
	}
	return web.Render(c, http.StatusOK, registerPage(form{Next: c.QueryParam("next"), Google: h.google != nil}))
}

// errPasswordMismatch: pesan kolom konfirmasi password (T23).
const errPasswordMismatch = "Konfirmasi password tidak sama"

// POST /register (form: name, email, password, password_confirmation, next) →
// auto login → next (bawaan /dashboard; pengguna baru diarahkan ke wizard).
func (h *Handler) Register(c echo.Context) error {
	f := formFrom(c, "name", "email")
	f.Next = c.FormValue("next")
	f.Google = h.google != nil
	in := RegisterInput{Name: f.v("name"), Email: f.v("email"), Password: c.FormValue("password")}
	// Konfirmasi password adalah urusan form (salah ketik), bukan aturan akun:
	// diperiksa di sini bersama validasi kolom lain supaya semua pesan tampil sekaligus.
	if c.FormValue("password_confirmation") != in.Password {
		v := ValidationError{"password_confirmation": errPasswordMismatch}
		for _, k := range []string{"name", "email", "password"} {
			val := map[string]string{"name": in.Name, "email": in.Email, "password": in.Password}[k]
			if msg := ValidateField(k, val); msg != "" {
				v[k] = msg
			}
		}
		f.applyErr(v)
		return render(c, http.StatusUnprocessableEntity, registerForm(f), registerPage(f))
	}
	u, err := h.svc.Register(c.Request().Context(), in)
	if f.applyErr(err) {
		return render(c, http.StatusUnprocessableEntity, registerForm(f), registerPage(f))
	}
	if err != nil {
		return err
	}
	return h.startSession(c, u, f.Next)
}

// POST /register/validate (form: field + nilai field) → fragment pesan error field.
func (h *Handler) ValidateRegisterField(c echo.Context) error {
	name := c.FormValue("field")
	switch name {
	case "name", "email", "password":
	case "password_confirmation":
		// Dibandingkan dengan kolom password (ikut terkirim lewat hx-include).
		msg := ""
		if c.FormValue("password_confirmation") != c.FormValue("password") {
			msg = errPasswordMismatch
		}
		return web.Render(c, http.StatusOK, fieldError("register", name, msg))
	default:
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	return web.Render(c, http.StatusOK, fieldError("register", name, ValidateField(name, c.FormValue(name))))
}

// ---------- Lupa & reset password ----------

// GET /forgot-password
func (h *Handler) ForgotPage(c echo.Context) error {
	return web.Render(c, http.StatusOK, forgotPage(form{}))
}

// POST /forgot-password (form: email). Respons selalu sama, terdaftar atau tidak.
func (h *Handler) Forgot(c echo.Context) error {
	f := formFrom(c, "email")
	if msg := ValidateField("email", f.v("email")); msg != "" {
		f.Errors["email"] = msg
		return render(c, http.StatusUnprocessableEntity, forgotForm(f), forgotPage(f))
	}
	if err := h.svc.RequestPasswordReset(c.Request().Context(), f.v("email")); err != nil {
		return err
	}
	return render(c, http.StatusOK, forgotSent(), forgotSentPage())
}

// GET /reset-password?token=...
func (h *Handler) ResetPage(c echo.Context) error {
	token := c.QueryParam("token")
	if _, ok := parseToken(token); !ok {
		return web.Render(c, http.StatusNotFound, resetInvalidPage())
	}
	return web.Render(c, http.StatusOK, resetPage(form{}, token))
}

// POST /reset-password (form: token, password) → /login?reset=1
func (h *Handler) Reset(c echo.Context) error {
	token := c.FormValue("token")
	f := formFrom(c)
	err := h.svc.ResetPassword(c.Request().Context(), token, c.FormValue("password"))
	switch {
	case f.applyErr(err):
		return render(c, http.StatusUnprocessableEntity, resetForm(f, token), resetPage(f, token))
	case errors.Is(err, ErrInvalidResetToken):
		f.Message = ErrInvalidResetToken.Error()
		return render(c, http.StatusUnprocessableEntity, resetForm(f, token), resetPage(f, token))
	case err != nil:
		return err
	}
	return web.Redirect(c, "/login?reset=1")
}
