package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Cookie sementara selama pengguna berada di halaman Google: menyimpan state
// (anti-CSRF), nonce, PKCE verifier, dan tujuan setelah masuk. Ditandatangani
// HMAC supaya tidak bisa dipalsukan; SameSite=Lax supaya ikut terkirim saat
// Google mengalihkan kembali ke situs ini.
const (
	googleCookie    = "lovoria_oauth"
	googleCookieTTL = 10 * time.Minute
)

type googleState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Next     string `json:"x"`
	Exp      int64  `json:"e"`
}

func (h *Handler) signState(st googleState) string {
	b, _ := json.Marshal(st)
	body := base64.RawURLEncoding.EncodeToString(b)
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (h *Handler) readState(c echo.Context) (googleState, bool) {
	ck, err := c.Cookie(googleCookie)
	if err != nil {
		return googleState{}, false
	}
	body, sig, ok := strings.Cut(ck.Value, ".")
	if !ok {
		return googleState{}, false
	}
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(body))
	want, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil || !hmac.Equal(want, mac.Sum(nil)) {
		return googleState{}, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	var st googleState
	if err != nil || json.Unmarshal(raw, &st) != nil || st.Exp < time.Now().Unix() {
		return googleState{}, false
	}
	return st, true
}

func (h *Handler) setStateCookie(c echo.Context, value string, maxAge int) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // G124: Secure sengaja dari config
		Name: googleCookie, Value: value, Path: "/auth/google", MaxAge: maxAge,
		HttpOnly: true, Secure: h.mw.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// googleFailed: kembali ke halaman masuk dengan pesan yang bisa ditindaklanjuti.
func (h *Handler) googleFailed(c echo.Context, next string) error {
	h.setStateCookie(c, "", -1)
	return web.Redirect(c, addQuery(withNext("/login", next), "google=gagal"))
}

// addQuery menambahkan satu pasangan kunci=nilai ke path yang mungkin sudah berquery.
func addQuery(path, kv string) string {
	if strings.Contains(path, "?") {
		return path + "&" + kv
	}
	return path + "?" + kv
}

// GET /auth/google?next=… → halaman persetujuan Google.
func (h *Handler) GoogleStart(c echo.Context) error {
	if h.google == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	next := c.QueryParam("next")
	if _, ok := CurrentUser(c.Request().Context()); ok {
		return web.Redirect(c, safeNext(next))
	}
	var vals [3]string
	for i := range vals {
		t, err := newToken()
		if err != nil {
			return err
		}
		vals[i] = t
	}
	st := googleState{State: vals[0], Nonce: vals[1], Verifier: vals[2], Next: next, Exp: time.Now().Add(googleCookieTTL).Unix()}
	h.setStateCookie(c, h.signState(st), int(googleCookieTTL.Seconds()))
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Redirect(http.StatusSeeOther, h.google.AuthCodeURL(st.State, st.Nonce, st.Verifier))
}

// GET /auth/google/callback?state=…&code=… → masuk (membuat / menghubungkan akun).
func (h *Handler) GoogleCallback(c echo.Context) error {
	if h.google == nil {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	ctx := c.Request().Context()
	q := c.QueryParams() // state, code, error dari Google
	st, ok := h.readState(c)
	// state dari Google harus sama dengan yang kita simpan (anti-CSRF login).
	if !ok || q.Get("state") == "" || !hmac.Equal([]byte(q.Get("state")), []byte(st.State)) {
		return h.googleFailed(c, "")
	}
	if q.Get("error") != "" || q.Get("code") == "" { // pengguna membatalkan di halaman Google
		return h.googleFailed(c, st.Next)
	}
	profile, err := h.google.Exchange(ctx, q.Get("code"), st.Verifier, st.Nonce)
	if err != nil {
		h.log.WarnContext(ctx, "auth: masuk dengan Google gagal", slog.String("error", err.Error()))
		return h.googleFailed(c, st.Next)
	}
	u, reset, err := h.svc.LoginWithGoogle(ctx, profile)
	if errors.Is(err, ErrAccountDisabled) {
		h.setStateCookie(c, "", -1)
		return web.Redirect(c, "/login?google=nonaktif")
	}
	if err != nil {
		return err
	}
	h.setStateCookie(c, "", -1)
	next := safeNext(st.Next)
	if !reset {
		return h.startSession(c, u, next)
	}
	// Akun lama (dibuat dengan password, email belum terverifikasi) baru saja
	// dihubungkan: password lamanya dimatikan — beri tahu sebelum melanjutkan.
	token, exp, err := h.svc.CreateSession(ctx, u.ID, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return err
	}
	h.mw.setCookie(c, token, exp)
	c.Response().Header().Set("Cache-Control", "no-store")
	return web.Render(c, http.StatusOK, googleLinkedPage(u.Email, next))
}

// googleNotice: pesan di halaman masuk setelah percobaan lewat Google.
func googleNotice(code string) string {
	switch code {
	case "gagal":
		return "Masuk dengan Google tidak berhasil. Silakan coba lagi, atau masuk dengan email dan password."
	case "nonaktif":
		return ErrAccountDisabled.Error()
	}
	return ""
}
