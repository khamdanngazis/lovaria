package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Sign in with Google (T30): alur OAuth 2.0 authorization code + PKCE + OpenID
// Connect, seluruhnya di sisi server — tanpa script Google di halaman (CSP
// tetap ketat). Dokumentasi: developers.google.com/identity/protocols/oauth2/openid-connect

// ProviderGoogle: nilai kolom user_identities.provider.
const ProviderGoogle = "google"

// ErrGoogleUnverified: Google tidak menyatakan email itu terverifikasi —
// tidak boleh dipakai untuk masuk / menghubungkan akun.
var ErrGoogleUnverified = errors.New("email akun Google belum terverifikasi")

// GoogleProfile: identitas dari Google yang kita pakai.
type GoogleProfile struct {
	Subject string // ID akun Google (stabil; email bisa berubah)
	Email   string
	Name    string
}

// Google: klien OAuth Google. Nil → fitur nonaktif (tombol disembunyikan).
type Google struct {
	ClientID, ClientSecret string
	// RedirectURL: BASE_URL + /auth/google/callback (harus terdaftar di Google Cloud Console).
	RedirectURL string
	// AuthURL & TokenURL: endpoint Google (diganti di test).
	AuthURL, TokenURL string
	Client            *http.Client
	now               func() time.Time
}

// NewGoogle membuat klien; nil bila kredensial tidak lengkap.
func NewGoogle(clientID, clientSecret, baseURL string) *Google {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &Google{ //nolint:gosec // G101: URL endpoint token, bukan kredensial
		ClientID: clientID, ClientSecret: clientSecret,
		RedirectURL: strings.TrimRight(baseURL, "/") + "/auth/google/callback",
		AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:    "https://oauth2.googleapis.com/token",
		Client:      &http.Client{Timeout: 10 * time.Second},
		now:         time.Now,
	}
}

// pkceChallenge: S256(code_verifier).
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthCodeURL: alamat halaman persetujuan Google.
func (g *Google) AuthCodeURL(state, nonce, verifier string) string {
	q := url.Values{
		"client_id": {g.ClientID}, "redirect_uri": {g.RedirectURL}, "response_type": {"code"},
		"scope": {"openid email profile"}, "state": {state}, "nonce": {nonce},
		"code_challenge": {pkceChallenge(verifier)}, "code_challenge_method": {"S256"},
		"prompt": {"select_account"},
	}
	return g.AuthURL + "?" + q.Encode()
}

type googleClaims struct {
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // bool, kadang string "true"
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	Exp           int64  `json:"exp"`
}

// Exchange menukar kode otorisasi dengan ID token lalu memeriksa klaimnya.
// ID token diterima langsung dari endpoint token Google lewat TLS dengan
// client secret kita, jadi keasliannya dijamin kanal itu (OIDC Core §3.1.3.7);
// yang diperiksa di sini: penerbit, audience, masa berlaku, nonce, dan bahwa
// emailnya terverifikasi.
func (g *Google) Exchange(ctx context.Context, code, verifier, nonce string) (GoogleProfile, error) {
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {g.RedirectURL},
		"client_id": {g.ClientID}, "client_secret": {g.ClientSecret}, "code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return GoogleProfile{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return GoogleProfile{}, fmt.Errorf("google: tukar kode: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return GoogleProfile{}, fmt.Errorf("google: baca jawaban token: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return GoogleProfile{}, fmt.Errorf("google: endpoint token HTTP %d", resp.StatusCode)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return GoogleProfile{}, errors.New("google: jawaban token tanpa id_token")
	}
	parts := strings.Split(tok.IDToken, ".")
	if len(parts) != 3 {
		return GoogleProfile{}, errors.New("google: id_token tidak berbentuk JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return GoogleProfile{}, errors.New("google: id_token tidak terbaca")
	}
	var c googleClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return GoogleProfile{}, errors.New("google: klaim id_token tidak terbaca")
	}
	switch {
	case c.Iss != "https://accounts.google.com" && c.Iss != "accounts.google.com":
		return GoogleProfile{}, errors.New("google: penerbit id_token tidak dikenal")
	case c.Aud != g.ClientID:
		return GoogleProfile{}, errors.New("google: id_token bukan untuk aplikasi ini")
	case c.Exp <= g.now().Unix():
		return GoogleProfile{}, errors.New("google: id_token kedaluwarsa")
	case nonce == "" || c.Nonce != nonce:
		return GoogleProfile{}, errors.New("google: nonce tidak cocok")
	case c.Sub == "" || c.Email == "":
		return GoogleProfile{}, errors.New("google: id_token tanpa identitas")
	}
	if v, _ := c.EmailVerified.(bool); !v && c.EmailVerified != "true" {
		return GoogleProfile{}, ErrGoogleUnverified
	}
	return GoogleProfile{Subject: c.Sub, Email: normalizeEmail(c.Email), Name: strings.TrimSpace(c.Name)}, nil
}
