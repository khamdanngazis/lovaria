package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Mode lihat-saja ("impersonate read-only"): admin membuka dashboard wedding
// pasangan tanpa bisa mengubah apa pun. Cookie bertanda tangan HMAC berisi
// admin, wedding, dan batas waktu; wedding.RequireWeddingOwner memanggil
// ReadOnlyAccess dan hanya mengizinkan GET/HEAD.
const (
	viewCookie = "lovoria_admin_view"
	viewTTL    = time.Hour
)

func (s *Service) viewMAC(payload string) string {
	m := hmac.New(sha256.New, s.secret)
	m.Write([]byte("admin-view|" + payload))
	return hex.EncodeToString(m.Sum(nil))
}

// viewToken: "<admin id>.<wedding id>.<unix kedaluwarsa>.<mac>".
func (s *Service) viewToken(adminID, weddingID uuid.UUID, exp time.Time) string {
	payload := adminID.String() + "." + weddingID.String() + "." + strconv.FormatInt(exp.Unix(), 10)
	return payload + "." + s.viewMAC(payload)
}

// viewTarget memeriksa cookie dan mengembalikan wedding yang boleh dilihat admin.
func (s *Service) viewTarget(c echo.Context, adminID uuid.UUID) (uuid.UUID, bool) {
	ck, err := c.Cookie(viewCookie)
	if err != nil {
		return uuid.Nil, false
	}
	i := strings.LastIndexByte(ck.Value, '.')
	if i < 0 || !hmac.Equal([]byte(ck.Value[i+1:]), []byte(s.viewMAC(ck.Value[:i]))) {
		return uuid.Nil, false
	}
	parts := strings.Split(ck.Value[:i], ".")
	if len(parts) != 3 || parts[0] != adminID.String() {
		return uuid.Nil, false
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil || s.now().Unix() > exp {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(parts[1])
	return id, err == nil
}

// ReadOnlyAccess memenuhi wedding.AdminAccess.
func (s *Service) ReadOnlyAccess(c echo.Context, u web.User, weddingID uuid.UUID) bool {
	if u.Role != auth.RoleAdmin {
		return false
	}
	id, ok := s.viewTarget(c, u.ID)
	return ok && id == weddingID
}

func (s *Service) setViewCookie(c echo.Context, value string, maxAge int) {
	c.SetCookie(&http.Cookie{ //nolint:gosec // G124: Secure sengaja dari config (false hanya di development)
		Name: viewCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, Secure: s.cookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

// StartView memulai mode lihat-saja untuk wedding (dicatat di audit log).
func (s *Service) StartView(c echo.Context, weddingID uuid.UUID) error {
	ctx := c.Request().Context()
	u, _ := web.CurrentUser(ctx)
	w, err := s.Weddings.GetWedding(ctx, weddingID)
	if err != nil {
		return ErrNotFound
	}
	s.setViewCookie(c, s.viewToken(u.ID, weddingID, s.now().Add(viewTTL)), int(viewTTL.Seconds()))
	return s.audit(ctx, "wedding.view_as_couple", "wedding", weddingID.String(), map[string]string{"slug": w.Slug})
}

// StopView mengakhiri mode lihat-saja; mengembalikan wedding yang tadi dilihat.
func (s *Service) StopView(c echo.Context) (uuid.UUID, error) {
	ctx := c.Request().Context()
	u, _ := web.CurrentUser(ctx)
	id, ok := s.viewTarget(c, u.ID)
	s.setViewCookie(c, "", -1)
	if !ok {
		return uuid.Nil, nil
	}
	return id, s.audit(ctx, "wedding.view_end", "wedding", id.String(), nil)
}
