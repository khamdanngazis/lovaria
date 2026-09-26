// Package web berisi helper HTTP yang dipakai lintas modul.
package web

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// Render menulis komponen templ sebagai respons HTML.
func Render(c echo.Context, status int, component templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return component.Render(c.Request().Context(), c.Response().Writer)
}

// IsHTMX mengembalikan true bila request dikirim oleh htmx (header HX-Request).
func IsHTMX(c echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

// Redirect mengarahkan browser ke url. Request htmx mendapat header HX-Redirect
// (htmx tidak mengikuti 3xx sebagai navigasi halaman penuh).
func Redirect(c echo.Context, url string) error {
	if IsHTMX(c) {
		c.Response().Header().Set("HX-Redirect", url)
		return c.NoContent(http.StatusOK)
	}
	return c.Redirect(http.StatusSeeOther, url)
}

// Retarget mengarahkan swap htmx respons ini ke selector lain (mis. form yang
// gagal validasi, sementara hx-target default-nya daftar).
func Retarget(c echo.Context, selector string) {
	c.Response().Header().Set("HX-Retarget", selector)
	c.Response().Header().Set("HX-Reswap", "outerHTML")
}
