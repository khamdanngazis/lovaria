package publicsite

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// OwnHost: request datang ke domain Lunovia sendiri (bukan custom domain pasangan).
func (r *Resolver) OwnHost(req *http.Request) bool { return r.isOwnHost(r.hostOf(req)) }

// robots: domain Lunovia → landing & halaman legal boleh diindeks, undangan &
// dashboard tidak; custom domain pasangan → tidak diindeks sama sekali.
func robots(r *Resolver) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
		if !r.OwnHost(c.Request()) {
			return c.String(http.StatusOK, "User-agent: *\nDisallow: /\n")
		}
		base := strings.TrimRight(r.BaseURL, "/")
		return c.String(http.StatusOK, "User-agent: *\nAllow: /$\nAllow: /privacy\nAllow: /terms\n"+
			"Disallow: /dashboard\nDisallow: /admin\nDisallow: /i/\nDisallow: /w/\nDisallow: /login\nDisallow: /register\n\n"+
			"Sitemap: "+base+"/sitemap.xml\n")
	}
}

func sitemap(r *Resolver) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !r.OwnHost(c.Request()) {
			return notFound(c)
		}
		base := strings.TrimRight(r.BaseURL, "/")
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
		for _, p := range []string{"/", "/privacy", "/terms"} {
			b.WriteString("  <url><loc>" + base + p + "</loc></url>\n")
		}
		b.WriteString("</urlset>\n")
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
		return c.Blob(http.StatusOK, "application/xml; charset=utf-8", []byte(b.String()))
	}
}
