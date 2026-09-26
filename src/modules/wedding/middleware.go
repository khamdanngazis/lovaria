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
		if errors.Is(err, ErrNotFound) {
			return echo.NewHTTPError(http.StatusNotFound)
		}
		if err != nil {
			return err
		}
		r := c.Request()
		ctx := context.WithValue(r.Context(), weddingCtxKey{}, w)
		c.SetRequest(r.WithContext(web.WithWeddingID(ctx, w.ID)))
		return next(c)
	}
}
