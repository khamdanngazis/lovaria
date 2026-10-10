package server

import (
	"strings"

	"github.com/labstack/echo/v4"
)

// Content-Security-Policy. Semua script dilayani dari /static (htmx, Alpine,
// lovoria.js, invitation.js); tidak ada script atau handler inline.
//   - CSPDashboard: Alpine versi standar mengevaluasi ekspresi x-data/@click
//     lewat Function(), jadi butuh 'unsafe-eval'.
//   - CSPPublic: halaman undangan tanpa Alpine → tanpa eval sama sekali.
//
// style 'unsafe-inline': atribut style (lebar progress bar) & blok token CSS
// tema. img-src https: untuk foto R2 / URL gambar yang diisi pasangan.
// checkoutOrigins: halaman bayar payment gateway (T23). Tombol "Bayar" adalah
// form POST yang dibalas redirect ke gateway — browser menerapkan form-action
// ke seluruh rantai redirect, jadi tanpa ini redirect diblokir diam-diam dan
// tombol terasa "tidak ke mana-mana". Hanya tujuan navigasi; script, frame, dan
// koneksi ke domain ini tetap tidak diizinkan.
const checkoutOrigins = "https://app.midtrans.com https://app.sandbox.midtrans.com"

const (
	cspBase = "default-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
		"font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https:; media-src 'self' https:; " +
		"connect-src 'self'; frame-src 'self'; frame-ancestors 'self'; form-action 'self' " + checkoutOrigins + "; base-uri 'self'; object-src 'none'"
	CSPDashboard = cspBase + "; script-src 'self' 'unsafe-eval'"
	CSPPublic    = cspBase + "; script-src 'self'"
)

// securityHeaders memasang CSP default (dashboard) dan Permissions-Policy.
// Route publik menimpa CSP dengan StrictCSP.
func securityHeaders(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		h := c.Response().Header()
		h.Set("Content-Security-Policy", CSPDashboard)
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		return next(c)
	}
}

// StrictCSP: CSP tanpa eval untuk halaman undangan publik.
func StrictCSP(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Content-Security-Policy", CSPPublic)
		return next(c)
	}
}

// AllowCamera: izinkan kamera untuk halaman ini saja (pemindai QR check-in,
// T31). Halaman lain tetap camera=().
func AllowCamera(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=(), payment=(), usb=()")
		return next(c)
	}
}

// NoStore: halaman pribadi (dashboard, admin) tidak boleh disimpan cache
// browser bersama maupun CDN.
func NoStore(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Before(func() {
			h := c.Response().Header()
			if h.Get("Cache-Control") == "" || !strings.Contains(h.Get("Cache-Control"), "no-store") {
				h.Set("Cache-Control", "private, no-store")
			}
		})
		return next(c)
	}
}
