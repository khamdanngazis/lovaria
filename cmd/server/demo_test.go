package main

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
)

func TestSeedDemo(t *testing.T) {
	a, _ := hardeningApp(t)
	ctx := context.Background()
	var out strings.Builder
	if err := a.seedDemo(ctx, &out, false); err != nil {
		t.Fatal(err)
	}
	if err := a.seedDemo(ctx, io.Discard, false); err != nil { // idempoten
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
	// Foto lengkap: sampul (foto utama), galeri, kutipan contoh; foto mempelai hanya
	// di set yang memilikinya (Signature).
	photos := gallery.NewService(gallery.NewRepository(a.pool), a.store, a.weddings, a.cfg.Storage.QuotaBytes, a.log)
	themes := theme.NewService(a.pool, a.weddings)
	for _, d := range theme.All() {
		w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		items, _ := photos.ListGallery(ctx, w.ID)
		c, _ := a.weddings.GetCouple(ctx, w.ID)
		st, _ := themes.Settings(ctx, w.ID)
		// Sampul (kategori cover, tidak tampil di galeri) + 6 foto galeri.
		wantCouple := demoMedia[demoMediaSource(d.ID)].Groom != ""
		if len(items) != 7 || w.MainPhotoURL == nil || (c.GroomPhotoURL != nil) != wantCouple || (c.BridePhotoURL != nil) != wantCouple || st.QuoteText == "" {
			t.Errorf("demo %s belum lengkap: %d foto, main=%v, pasangan=%v/%v, kutipan=%q", d.ID, len(items), w.MainPhotoURL, c.GroomPhotoURL, c.BridePhotoURL, st.QuoteText)
		}
	}
	// QR kehadiran: check-in menyala dengan tepat satu tamu contoh, juga saat
	// seed diulang.
	guests := guest.NewService(guest.NewRepository(a.pool), a.cfg.BaseURL)
	for _, d := range theme.All() {
		w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		page, err := guests.List(ctx, w.ID, guest.Filter{Page: 1})
		if err != nil || !w.CheckinOpen() || !w.IsDemo || page.Total != 1 {
			t.Errorf("demo %s: check-in %v, demo %v, %d tamu, %v", d.ID, w.CheckinOpen(), w.IsDemo, page.Total, err)
		}
	}
	// Tiap bab cerita contoh punya foto; demo lama ikut dilengkapi saat seed diulang.
	stories := story.NewService(story.NewRepository(a.pool))
	assertStoryPhotos := func() {
		t.Helper()
		for _, d := range theme.All() {
			w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
			list, err := stories.ListStories(ctx, w.ID)
			if err != nil || len(list) != 3 {
				t.Fatalf("demo %s: %d cerita, %v", d.ID, len(list), err)
			}
			for _, s := range list {
				if s.PhotoURL == "" {
					t.Errorf("demo %s: cerita %q tanpa foto", d.ID, s.Title)
				}
			}
		}
	}
	assertStoryPhotos()
	if _, err := a.pool.Exec(ctx, "UPDATE love_stories SET photo_url = NULL"); err != nil {
		t.Fatal(err)
	}
	if err := a.seedDemo(ctx, io.Discard, false); err != nil {
		t.Fatal(err)
	}
	assertStoryPhotos()
	// Demo dari versi lama (tanpa foto & kutipan) dilengkapi saat seed diulang. Satu
	// tema saja: mengisi ulang semua tema terlalu lambat di bawah -race.
	if _, err := a.pool.Exec(ctx, "DELETE FROM gallery_items WHERE wedding_id = (SELECT id FROM weddings WHERE slug = $1)", publicsite.DemoSlug("modern")); err != nil {
		t.Fatal(err)
	}
	if _, err := a.pool.Exec(ctx, "UPDATE wedding_theme_settings SET quote_text = NULL, color_preset = NULL"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := a.seedDemo(ctx, &out, false); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "dilengkapi:") != 1 {
		t.Errorf("seed ulang demo lama: %s", out.String())
	}
	// Tema warna undangan contoh (T37): dipasang lagi saat seed diulang.
	for _, d := range theme.All() {
		w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		st, _ := themes.Settings(ctx, w.ID)
		if st.ColorPreset != demoColorPreset[d.ID] || st.QuoteText == "" {
			t.Errorf("demo %s: tema warna %q, want %q; kutipan %q", d.ID, st.ColorPreset, demoColorPreset[d.ID], st.QuoteText)
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

// Berkas demomedia konsisten dengan demoMedia dan CREDITS.md. Keberadaan wajah
// tidak bisa dites: periksa manual setiap foto baru (aturan di doc/runbook.md).
func TestDemoMediaFiles(t *testing.T) {
	credits, err := fs.ReadFile(os.DirFS("demomedia"), "CREDITS.md")
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for name, set := range demoMedia {
		if set.Cover == "" || len(set.Gallery) != 6 || len(set.Stories) != 3 {
			t.Errorf("set %s tidak lengkap: %+v", name, set)
		}
		used[set.Cover] = true
		for _, p := range set.Gallery {
			used[p.File] = true
		}
		for _, f := range set.Stories {
			used[f] = true
		}
		if set.Groom != "" {
			used[set.Groom], used[set.Bride] = true, true
		}
	}
	for _, file := range demoCoverOverride {
		used[file] = true
	}
	for file := range used {
		if _, err := fs.Stat(demoMediaFS, "demomedia/"+file); err != nil {
			t.Errorf("dirujuk demoMedia, berkas tidak ada: %s", file)
		}
	}
	files, err := fs.Glob(demoMediaFS, "demomedia/*.jpg")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		file := filepath.Base(path)
		if strings.HasSuffix(file, "-groom.jpg") || strings.HasSuffix(file, "-bride.jpg") {
			t.Errorf("potret mempelai tidak boleh ada: %s", file)
		}
		if !used[file] {
			t.Errorf("berkas yatim: %s", file)
		}
		if !strings.Contains(string(credits), "| "+file+" |") {
			t.Errorf("belum tercatat di CREDITS.md: %s", file)
		}
	}
	for id, preset := range demoColorPreset {
		d, ok := theme.Get(id)
		if _, found := d.Preset(preset); !ok || !found {
			t.Errorf("demoColorPreset: %s/%s tidak ada", id, preset)
		}
	}
	for _, d := range theme.All() {
		if _, ok := demoMedia[demoMediaAlias[d.ID]]; !ok {
			if _, ok := demoMedia[d.ID]; !ok {
				t.Errorf("tema %s tanpa set foto", d.ID)
			}
		}
	}
}
