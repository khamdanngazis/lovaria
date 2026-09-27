package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
)

func TestSeedDemo(t *testing.T) {
	a, _ := hardeningApp(t)
	ctx := context.Background()
	var out strings.Builder
	if err := a.seedDemo(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if err := a.seedDemo(ctx, io.Discard); err != nil { // idempoten
		t.Fatalf("seed ulang: %v", err)
	}
	for _, d := range theme.All() {
		w, err := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		if err != nil || !w.IsPublic() || w.ThemeID != d.ID {
			t.Errorf("demo %s: %+v %v", d.ID, w, err)
		}
	}
	if strings.Count(out.String(), "dibuat:") != len(theme.All()) {
		t.Errorf("output seed: %s", out.String())
	}
	// Foto lengkap: sampul (foto utama), potret mempelai, galeri; kutipan contoh.
	photos := gallery.NewService(gallery.NewRepository(a.pool), a.store, a.weddings, a.cfg.Storage.QuotaBytes, a.log)
	themes := theme.NewService(a.pool, a.weddings)
	for _, d := range theme.All() {
		w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		items, _ := photos.ListGallery(ctx, w.ID)
		c, _ := a.weddings.GetCouple(ctx, w.ID)
		st, _ := themes.Settings(ctx, w.ID)
		// Sampul (kategori cover, tidak tampil di galeri) + 6 foto galeri;
		// potret mempelai tidak menjadi item galeri.
		if len(items) != 7 || w.MainPhotoURL == nil || c.GroomPhotoURL == nil || c.BridePhotoURL == nil || st.QuoteText == "" {
			t.Errorf("demo %s belum lengkap: %d foto, main=%v, pasangan=%v/%v, kutipan=%q", d.ID, len(items), w.MainPhotoURL, c.GroomPhotoURL, c.BridePhotoURL, st.QuoteText)
		}
	}
	// Demo dari versi lama (tanpa foto & kutipan) dilengkapi saat seed diulang.
	if _, err := a.pool.Exec(ctx, "DELETE FROM gallery_items; UPDATE wedding_theme_settings SET quote_text = NULL"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := a.seedDemo(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "dilengkapi:") != len(theme.All()) {
		t.Errorf("seed ulang demo lama: %s", out.String())
	}
	if w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug("modern")); true {
		if items, _ := photos.ListGallery(ctx, w.ID); len(items) != 7 {
			t.Errorf("dilengkapi: %d foto", len(items))
		}
		if st, _ := themes.Settings(ctx, w.ID); st.QuoteText == "" {
			t.Error("dilengkapi: kutipan")
		}
	}
	// Tidak dihitung laporan admin.
	if p, _ := a.weddings.AdminList(ctx, wedding.AdminFilter{}, 25); p.Total != 0 {
		t.Errorf("demo muncul di admin: %d", p.Total)
	}
	if by, _ := a.weddings.CountByStatus(ctx); len(by) != 0 {
		t.Errorf("demo terhitung status: %v", by)
	}
	// Pemilik demo tidak bisa login (akun dinonaktifkan).
	w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug("elegant"))
	if u, err := a.auth.GetUser(ctx, w.OwnerUserID); err != nil || u.DisabledAt == nil {
		t.Errorf("pemilik demo harus nonaktif: %+v %v", u, err)
	}
	// Scheduler lifecycle tidak menyentuh demo walau tanggalnya sudah lewat.
	if _, err := a.pool.Exec(ctx, "UPDATE weddings SET wedding_date = now() - interval '30 days' WHERE is_demo"); err != nil {
		t.Fatal(err)
	}
	if n, err := a.weddings.AdvanceDue(ctx, 365); err != nil || n != 0 {
		t.Errorf("lifecycle memproses demo: %d %v", n, err)
	}
}
