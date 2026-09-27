package wedding

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestChangeSlug(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")
	w, _ := f.svc.CreateWedding(ctx, owner, validInput())
	other, _ := f.svc.CreateWedding(ctx, f.user(t, "b@example.com"), validInput())
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }

	var se *SlugError
	for in, want := range map[string]string{
		"":                      "wajib",
		"Admin":                 "tidak bisa dipakai",
		"i":                     "tidak bisa dipakai",
		"budi_sari":             "huruf kecil",
		"-budi":                 "huruf kecil",
		other.Slug:              "sudah dipakai",
		strings.Repeat("a", 61): "maksimal",
	} {
		if _, err := f.svc.ChangeSlug(ctx, w.ID, in); !errors.As(err, &se) || !strings.Contains(se.Msg, want) {
			t.Errorf("ChangeSlug(%q) = %v, want %q", in, err, want)
		}
	}
	old := w.Slug
	got, err := f.svc.ChangeSlug(ctx, w.ID, "  Khamdan-Sarah-2026 ")
	if err != nil || got.Slug != "khamdan-sarah-2026" {
		t.Fatalf("ganti: %+v %v", got, err)
	}
	// Slug lama dialihkan ke wedding ini.
	if tw, ok, _ := f.svc.SlugRedirectTarget(ctx, old); !ok || tw.ID != w.ID || tw.Slug != "khamdan-sarah-2026" {
		t.Errorf("redirect: %v %+v", ok, tw)
	}
	// Slug lama milik wedding ini tidak boleh diambil wedding lain selama redirect aktif.
	if _, err := f.svc.ChangeSlug(ctx, other.ID, old); !errors.As(err, &se) {
		t.Errorf("ambil slug lama wedding lain: %v", err)
	}
	// Ganti lagi: dua slug lama dialihkan; kembali ke slug lama sendiri boleh (redirect-nya dihapus).
	f.svc.ChangeSlug(ctx, w.ID, "ks-2026") //nolint:errcheck
	if rs, _ := f.svc.SlugRedirects(ctx, w.ID); len(rs) != 2 {
		t.Errorf("redirects = %+v", rs)
	}
	if got, err := f.svc.ChangeSlug(ctx, w.ID, old); err != nil || got.Slug != old {
		t.Fatalf("kembali ke slug lama: %v", err)
	}
	if _, ok, _ := f.svc.SlugRedirectTarget(ctx, old); ok {
		t.Error("slug yang dipakai lagi tidak boleh tetap jadi redirect")
	}
	// Setelah 90 hari redirect kedaluwarsa & slug bebas dipakai wedding lain.
	now = now.Add(SlugRedirectTTL + time.Hour)
	if _, ok, _ := f.svc.SlugRedirectTarget(ctx, "ks-2026"); ok {
		t.Error("redirect harus kedaluwarsa")
	}
	if _, err := f.svc.ChangeSlug(ctx, other.ID, "ks-2026"); err != nil {
		t.Errorf("slug kedaluwarsa dipakai wedding lain: %v", err)
	}
}
