package publicsite

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// DomainLookup mencocokkan Host header ke wedding (custom domain, T15).
type DomainLookup interface {
	WeddingIDByHost(ctx context.Context, host string) (id uuid.UUID, ok bool, err error)
}

// NoDomains adalah DomainLookup kosong sampai custom domain tersedia (T15).
type NoDomains struct{}

func (NoDomains) WeddingIDByHost(context.Context, string) (uuid.UUID, bool, error) {
	return uuid.Nil, false, nil
}

// Resolved adalah hasil ResolveWedding yang disimpan di context.
type Resolved struct {
	Wedding wedding.Wedding
	Guest   *guest.Guest // nil bila tanpa kode tamu
	// Preview: wedding belum publik dan dilihat oleh pemiliknya.
	Preview bool
	// Origin: basis URL halaman ini (domain Lovoria atau custom domain).
	Origin string
	// Prefix: path dasar undangan di origin ("/i/KODE", "/w/slug", atau "" di custom domain).
	Prefix string
}

type resolvedKey struct{}

// FromContext mengembalikan wedding yang di-resolve ResolveWedding.
func FromContext(ctx context.Context) (Resolved, bool) {
	r, ok := ctx.Value(resolvedKey{}).(Resolved)
	return r, ok
}

// Resolver adalah SATU-SATUNYA tempat resolusi wedding dari request
// (Arsitektur §3 aturan 5): Host header (custom domain) → path /i/:code → /w/:slug.
type Resolver struct {
	Weddings *wedding.Service
	Guests   *guest.Service
	Domains  DomainLookup
	BaseURL  string // domain utama Lovoria
	Log      *slog.Logger
	ownHosts map[string]bool
}

func (r *Resolver) isOwnHost(host string) bool {
	if r.ownHosts == nil {
		r.ownHosts = map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
		if u, err := url.Parse(r.BaseURL); err == nil && u.Hostname() != "" {
			r.ownHosts[strings.ToLower(u.Hostname())] = true
		}
	}
	return r.ownHosts[host]
}

func hostOf(req *http.Request) string {
	h := req.Host
	if hh, _, err := net.SplitHostPort(h); err == nil {
		h = hh
	}
	return strings.ToLower(strings.TrimSuffix(h, "."))
}

// ResolveWedding menyimpan Resolved di context. Tanpa wedding (mis. "/" di
// domain utama) request diteruskan apa adanya; kode/slug tidak valid atau
// wedding yang belum publik (bukan owner) → halaman 404 ramah.
func (r *Resolver) ResolveWedding(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		host := hostOf(c.Request())
		res := Resolved{Origin: strings.TrimRight(r.BaseURL, "/")}
		found := false

		// 1. Custom domain.
		if !r.isOwnHost(host) {
			id, ok, err := r.Domains.WeddingIDByHost(ctx, host)
			if err != nil {
				return err
			}
			if ok {
				w, err := r.Weddings.GetWedding(ctx, id)
				if err != nil && !errors.Is(err, wedding.ErrNotFound) {
					return err
				}
				if err == nil {
					res.Wedding, found = w, true
					res.Origin = c.Scheme() + "://" + host
				}
			}
		}

		// 2. /i/:code → tamu → wedding.
		if code := c.Param("code"); code != "" {
			g, err := r.Guests.GetByCode(ctx, code)
			if errors.Is(err, guest.ErrNotFound) {
				return notFound(c)
			}
			if err != nil {
				return err
			}
			if found && g.WeddingID != res.Wedding.ID {
				return notFound(c) // kode milik wedding lain di custom domain ini
			}
			if !found {
				w, err := r.Weddings.GetWedding(ctx, g.WeddingID)
				if err != nil {
					return err
				}
				res.Wedding, found = w, true
			}
			res.Guest = &g
			res.Prefix = "/i/" + g.InvitationCode
		} else if slug := c.Param("slug"); slug != "" && !found {
			// 3. /w/:slug.
			w, err := r.Weddings.GetWeddingBySlug(ctx, slug)
			if errors.Is(err, wedding.ErrNotFound) {
				return notFound(c)
			}
			if err != nil {
				return err
			}
			res.Wedding, found = w, true
			res.Prefix = "/w/" + w.Slug
		}

		if !found {
			if c.Param("code") != "" || c.Param("slug") != "" || c.Path() != "/" {
				return notFound(c)
			}
			return next(c) // "/" di domain utama → landing page
		}

		// Gerbang status (wedding.IsPublic): draft hanya untuk owner (preview).
		if !res.Wedding.IsPublic() {
			u, ok := web.CurrentUser(ctx)
			if !ok || u.ID != res.Wedding.OwnerUserID {
				return notFound(c)
			}
			res.Preview = true
		}
		c.SetRequest(c.Request().WithContext(context.WithValue(ctx, resolvedKey{}, res)))
		return next(c)
	}
}

func notFound(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "no-store")
	return web.Render(c, http.StatusNotFound, notFoundPage())
}
