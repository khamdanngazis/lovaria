package wedding

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type weddingCtxKey struct{}

// FromContext mengembalikan wedding yang sudah diotorisasi oleh RequireWeddingOwner.
func FromContext(ctx context.Context) (Wedding, bool) {
	w, ok := ctx.Value(weddingCtxKey{}).(Wedding)
	return w, ok
}

// RequireWeddingOwner melindungi /dashboard/weddings/:weddingID/*: user login harus
// pemilik wedding. Wedding milik orang lain, ID tidak valid, atau tidak ada → 404
// (bukan 403) supaya keberadaan ID tidak bocor. Bila lolos, wedding disimpan di
// context (FromContext) dan wedding_id di web.WeddingID(ctx) untuk modul lain.
// Pasang setelah RequireAuth.
func (s *Service) RequireWeddingOwner(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		u, ok := web.CurrentUser(c.Request().Context())
		if !ok {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		id, err := uuid.Parse(c.Param("weddingID"))
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		w, err := s.GetWeddingForOwner(c.Request().Context(), u.ID, id)
		readOnly := false
		if errors.Is(err, ErrNotFound) && s.admin != nil && s.admin.ReadOnlyAccess(c, u, id) {
			// Admin sedang "lihat sebagai pasangan": hanya GET/HEAD.
			if m := c.Request().Method; m != http.MethodGet && m != http.MethodHead {
				return echo.NewHTTPError(http.StatusForbidden, "Mode lihat saja: admin tidak bisa mengubah data pasangan")
			}
			w, err = s.GetWedding(c.Request().Context(), id)
			readOnly = true
		}
		if errors.Is(err, ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if err != nil {
			return err
		}
		r := c.Request()
		ctx := context.WithValue(r.Context(), weddingCtxKey{}, w)
		if readOnly {
			ctx = web.WithReadOnly(ctx)
		}
		c.SetRequest(r.WithContext(web.WithWeddingID(ctx, w.ID)))
		return next(c)
	}
}

// AdminAccess memberi admin akses lihat-saja ke dashboard wedding orang lain
// ("impersonate read-only", modul admin T16). Diperiksa hanya bila user bukan pemilik.
type AdminAccess interface {
	ReadOnlyAccess(c echo.Context, u web.User, weddingID uuid.UUID) bool
}

// SetAdminAccess memasang pemeriksa akses admin (nil = tidak ada).
func (s *Service) SetAdminAccess(a AdminAccess) { s.admin = a }
