package publicsite

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/time/rate"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/platform/web"
	"github.com/khamdanngazis/lovaria/src/templates/shared"
)

// Token form RSVP menggantikan token CSRF (halaman undangan di-cache, jadi
// tidak memuat token per pengunjung): HMAC(secret, kode tamu + hari terbit).
// Timestamp dibulatkan per hari supaya HTML (dan ETag) stabil sepanjang hari.
const (
	tokenStep   = 24 * time.Hour
	tokenMaxAge = 30 * 24 * time.Hour
)

func (h *Handler) rsvpToken(code string, now time.Time) string {
	ts := strconv.FormatInt(now.Truncate(tokenStep).Unix(), 10)
	return ts + "." + h.rsvpMAC(code, ts)
}

func (h *Handler) rsvpMAC(code, ts string) string {
	m := hmac.New(sha256.New, h.Secret)
	m.Write([]byte("rsvp|" + code + "|" + ts))
	return hex.EncodeToString(m.Sum(nil)[:16])
}

func (h *Handler) validRSVPToken(code, token string, now time.Time) bool {
	ts, mac, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	age := now.Sub(time.Unix(sec, 0))
	if age < -tokenStep || age > tokenMaxAge {
		return false
	}
	return hmac.Equal([]byte(mac), []byte(h.rsvpMAC(code, ts)))
}

// rsvpView: data minimum untuk merender shared.RSVPSection (fragment htmx).
func (h *Handler) rsvpView(res Resolved, g guest.Guest) view.View {
	return view.View{
		AllowRSVP: res.Wedding.AllowsRSVP(),
		Guest:     &view.Guest{Name: g.Name, Code: g.InvitationCode, MaxPax: g.MaxPax, RSVPStatus: g.RSVPStatus, RSVPPax: g.RSVPPax, RSVPMessage: g.RSVPMessage},
		RSVP:      view.RSVPForm{Action: res.Prefix + "/rsvp", Token: h.rsvpToken(g.InvitationCode, h.clock())},
	}
}

func (h *Handler) renderRSVP(c echo.Context, status int, v view.View) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	return web.Render(c, status, shared.RSVPSection(v))
}

// RSVP: POST /i/:code/rsvp — payload {status, pax, message, token}. htmx →
// fragment section RSVP; form biasa (tanpa JS) → redirect 303 ke undangan.
func (h *Handler) RSVP(c echo.Context) error {
	ctx := c.Request().Context()
	res, _ := FromContext(ctx)
	if res.Guest == nil || res.Preview {
		return notFound(c)
	}
	g := *res.Guest
	v := h.rsvpView(res, g)
	if !res.Wedding.AllowsRSVP() {
		v.AllowRSVP = true // tampilkan form beserta pesannya
		v.RSVP.Error = "Maaf, konfirmasi kehadiran sudah ditutup."
		return h.renderRSVP(c, http.StatusForbidden, v)
	}
	if !h.validRSVPToken(g.InvitationCode, c.FormValue("token"), h.clock()) {
		v.RSVP.Error = "Halaman sudah terlalu lama dibuka. Silakan kirim ulang."
		return h.renderRSVP(c, http.StatusForbidden, v)
	}
	status, message := c.FormValue("status"), c.FormValue("message")
	pax, _ := strconv.Atoi(c.FormValue("pax"))
	updated, err := h.Guests.UpdateRSVP(ctx, g.WeddingID, g.ID, status, pax, message)
	var ve guest.ValidationError
	if errors.As(err, &ve) {
		v.RSVP.Errors = ve
		v.RSVP.Status, v.RSVP.Pax, v.RSVP.Message = status, pax, message
		if v.RSVP.Status == "" {
			v.RSVP.Status = "-" // pertahankan isian kosong, jangan kembali ke jawaban lama
		}
		return h.renderRSVP(c, http.StatusUnprocessableEntity, v)
	}
	if err != nil {
		return err
	}
	if !web.IsHTMX(c) {
		return c.Redirect(http.StatusSeeOther, res.Prefix+"?rsvp=ok#rsvp")
	}
	v = h.rsvpView(res, updated)
	v.RSVP.Notice = rsvpNotice(updated.RSVPStatus)
	return h.renderRSVP(c, http.StatusOK, v)
}

func rsvpNotice(status string) string {
	if status == guest.StatusDeclined {
		return "Terima kasih atas kabarnya. Konfirmasi Anda tersimpan."
	}
	return "Terima kasih! Konfirmasi Anda tersimpan. Sampai jumpa di hari bahagia kami."
}

// RSVPLimit: batas kiriman RSVP per kode tamu (in-memory, single instance).
type RSVPLimit struct {
	PerMinute float64
	Burst     int
}

var defaultRSVPLimit = RSVPLimit{PerMinute: 6, Burst: 5}

// rsvpLimiter dipasang SETELAH ResolveWedding supaya pesan 429 bisa dirender
// sebagai section RSVP.
func (h *Handler) rsvpLimiter(l RSVPLimit) echo.MiddlewareFunc {
	if l.PerMinute <= 0 {
		l = defaultRSVPLimit
	}
	return middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
			Rate: rate.Limit(l.PerMinute / 60), Burst: l.Burst, ExpiresIn: 10 * time.Minute,
		}),
		IdentifierExtractor: func(c echo.Context) (string, error) {
			if res, ok := FromContext(c.Request().Context()); ok && res.Guest != nil {
				return res.Guest.InvitationCode, nil
			}
			return "", nil
		},
		DenyHandler: func(c echo.Context, _ string, _ error) error {
			res, _ := FromContext(c.Request().Context())
			if res.Guest == nil {
				return notFound(c)
			}
			v := h.rsvpView(res, *res.Guest)
			v.RSVP.Error = "Terlalu banyak percobaan. Coba lagi sebentar lagi."
			return h.renderRSVP(c, http.StatusTooManyRequests, v)
		},
	})
}
