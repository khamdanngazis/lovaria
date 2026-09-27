package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

type fakeLister []wedding.Wedding

func (f fakeLister) ListWeddingsByOwner(context.Context, uuid.UUID) ([]wedding.Wedding, error) {
	return f, nil
}

func TestHomeRedirects(t *testing.T) {
	one := wedding.Wedding{ID: uuid.New()}
	cases := []struct {
		name string
		role string
		list fakeLister
		want string
	}{
		{"kosong", "couple", nil, "/dashboard/weddings/new"},
		{"satu", "couple", fakeLister{one}, "/dashboard/weddings/" + one.ID.String()},
		{"banyak", "couple", fakeLister{one, {ID: uuid.New()}}, "/dashboard/weddings"},
		// Admin tanpa wedding → panel admin, bukan wizard.
		{"admin kosong", "admin", nil, "/admin"},
		{"admin punya wedding", "admin", fakeLister{one}, "/dashboard/weddings/" + one.ID.String()},
	}
	for _, tc := range cases {
		e := echo.New()
		Register(e.Group("/dashboard"), Deps{Weddings: tc.list})
		r := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
		r = r.WithContext(web.WithUser(r.Context(), web.User{ID: uuid.New(), Role: tc.role}))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, r)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != tc.want {
			t.Errorf("%s: %d %s, want %s", tc.name, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}
