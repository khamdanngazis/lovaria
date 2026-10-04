package server

import (
	"regexp"
	"strings"
	"testing"
)

// Regresi T23: tombol "Bayar" adalah form POST yang dibalas redirect ke halaman
// bayar gateway. Browser menerapkan form-action ke rantai redirect, jadi domain
// gateway harus terdaftar — kalau tidak redirect diblokir tanpa pesan.
func TestCSPAllowsCheckoutRedirect(t *testing.T) {
	for name, csp := range map[string]string{"dashboard": CSPDashboard, "publik": CSPPublic} {
		fa := regexp.MustCompile(`form-action ([^;]*)`).FindStringSubmatch(csp)
		if fa == nil {
			t.Fatalf("%s: tidak ada form-action", name)
		}
		for _, want := range []string{"'self'", "https://app.midtrans.com", "https://app.sandbox.midtrans.com"} {
			if !strings.Contains(" "+fa[1]+" ", " "+want+" ") {
				t.Errorf("%s: form-action tidak memuat %s (%q)", name, want, fa[1])
			}
		}
		// Domain gateway hanya sebagai tujuan navigasi — bukan sumber script,
		// frame, atau koneksi.
		for _, d := range []string{"script-src", "connect-src", "frame-src", "default-src"} {
			m := regexp.MustCompile(d + ` ([^;]*)`).FindStringSubmatch(csp)
			if m == nil || strings.Contains(m[1], "midtrans") {
				t.Errorf("%s: %s tidak boleh memuat domain gateway", name, d)
			}
		}
	}
}
