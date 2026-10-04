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
	got, err := f.svc.ChangeSlug(ctx, w.ID, "  samuel-sarah-2026 ")
	if err != nil || got.Slug != "samuel-sarah-2026" {
		t.Fatalf("ganti: %+v %v", got, err)
	}
	// Slug lama dialihkan ke wedding ini.
	if tw, ok, _ := f.svc.SlugRedirectTarget(ctx, old); !ok || tw.ID != w.ID || tw.Slug != "samuel-sarah-2026" {
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

// T23: alamat undangan bisa dipilih saat membuat wedding (opsional).
func TestCreateWeddingWithCustomSlug(t *testing.T) {
	f := newFixture(t)
	owner := f.user(t, "a@example.com")
	in := validInput()

	// Kosong → otomatis dari nama mempelai (perilaku lama).
	auto, err := f.svc.CreateWedding(ctx, owner, in)
	if err != nil || auto.Slug == "" {
		t.Fatalf("otomatis: %q %v", auto.Slug, err)
	}
	// Pilihan sendiri: dinormalkan ke huruf kecil.
	in.Slug = "  Azis-Ida "
	w, err := f.svc.CreateWedding(ctx, owner, in)
	if err != nil || w.Slug != "azis-ida" || w.Status != StatusDraft {
		t.Fatalf("custom: %+v %v", w.Slug, err)
	}
	// Sudah dipakai (termasuk slug otomatis wedding lain) → error di kolom slug, wedding tidak dibuat.
	before, _ := f.svc.ListWeddingsByOwner(ctx, owner)
	for _, taken := range []string{"azis-ida", "AZIS-IDA", auto.Slug} {
		in.Slug = taken
		var ve ValidationError
		if _, err := f.svc.CreateWedding(ctx, owner, in); !errors.As(err, &ve) || ve["slug"] != "Alamat ini sudah dipakai undangan lain" {
			t.Errorf("slug %q terpakai: %v", taken, err)
		}
	}
	// Format tidak valid & kata terlarang.
	for _, bad := range []string{"ada spasi", "a/b", "dashboard", strings.Repeat("a", 61)} {
		in.Slug = bad
		var ve ValidationError
		if _, err := f.svc.CreateWedding(ctx, owner, in); !errors.As(err, &ve) || ve["slug"] == "" {
			t.Errorf("slug %q harus ditolak: %v", bad, err)
		}
	}
	if after, _ := f.svc.ListWeddingsByOwner(ctx, owner); len(after) != len(before) {
		t.Errorf("wedding bertambah walau slug ditolak: %d → %d", len(before), len(after))
	}
	// Slug lama yang masih dialihkan (redirect aktif) tidak bisa diambil wedding baru.
	if _, err := f.svc.ChangeSlug(ctx, w.ID, "azis-ida-baru"); err != nil {
		t.Fatal(err)
	}
	in.Slug = "azis-ida"
	var ve ValidationError
	if _, err := f.svc.CreateWedding(ctx, owner, in); !errors.As(err, &ve) || ve["slug"] == "" {
		t.Errorf("slug dengan redirect aktif harus ditolak: %v", err)
	}
}
