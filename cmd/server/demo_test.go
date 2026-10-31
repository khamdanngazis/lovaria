package main

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

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
	// Foto lengkap: sampul (foto utama), galeri, tanpa potret mempelai; kutipan contoh.
	photos := gallery.NewService(gallery.NewRepository(a.pool), a.store, a.weddings, a.cfg.Storage.QuotaBytes, a.log)
	themes := theme.NewService(a.pool, a.weddings)
	for _, d := range theme.All() {
		w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		items, _ := photos.ListGallery(ctx, w.ID)
		c, _ := a.weddings.GetCouple(ctx, w.ID)
		st, _ := themes.Settings(ctx, w.ID)
		// Sampul (kategori cover, tidak tampil di galeri) + 6 foto galeri.
		if len(items) != 7 || w.MainPhotoURL == nil || c.GroomPhotoURL != nil || c.BridePhotoURL != nil || st.QuoteText == "" {
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
	// Demo dari versi lama (tanpa foto & kutipan) dilengkapi saat seed diulang.
	if _, err := a.pool.Exec(ctx, "DELETE FROM gallery_items; UPDATE wedding_theme_settings SET quote_text = NULL"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := a.seedDemo(ctx, &out, false); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "dilengkapi:") != len(theme.All()) {
		t.Errorf("seed ulang demo lama: %s", out.String())
	}
	// --refresh: potret & foto cerita lama benar-benar hilang dari storage, tetapi
	// wedding bukan demo tidak tersentuh walau memakai slug contoh.
	objectExists := func(id uuid.UUID, url string) bool {
		t.Helper()
		key, ok := demoObjectKey(id, url)
		if !ok {
			t.Fatalf("URL di luar wedding: %s", url)
		}
		_, err := os.Stat(filepath.Join(a.cfg.Storage.LocalDir, key))
		return err == nil
	}
	giveOldPhotos := func(slug string) (wedding.Wedding, []string) {
		t.Helper()
		w, err := a.weddings.GetWeddingBySlug(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		groom, err := a.putDemoPhoto(ctx, w.ID, "couple", "ZYet8yoepik.jpg")
		if err != nil {
			t.Fatal(err)
		}
		bride, err := a.putDemoPhoto(ctx, w.ID, "couple", "ZYet8yoepik.jpg")
		if err != nil {
			t.Fatal(err)
		}
		c, _ := a.weddings.GetCouple(ctx, w.ID)
		if _, err := a.weddings.UpdateCouple(ctx, w.ID, wedding.CoupleInput{
			GroomName: c.GroomName, BrideName: c.BrideName, GroomDescription: c.GroomDescription, BrideDescription: c.BrideDescription,
			GroomPhotoURL: groom, BridePhotoURL: bride,
		}); err != nil {
			t.Fatal(err)
		}
		urls := []string{groom, bride}
		list, _ := stories.ListStories(ctx, w.ID)
		for _, s := range list {
			urls = append(urls, s.PhotoURL)
		}
		for _, u := range urls {
			if !objectExists(w.ID, u) {
				t.Fatalf("objek lama belum ada: %s", u)
			}
		}
		return w, urls
	}
	demo, demoOld := giveOldPhotos(publicsite.DemoSlug("modern"))
	other, otherOld := giveOldPhotos(publicsite.DemoSlug("sunda"))
	if _, err := a.pool.Exec(ctx, "UPDATE weddings SET is_demo = false WHERE id = $1", other.ID); err != nil {
		t.Fatal(err)
	}
	otherItems, _ := photos.ListGallery(ctx, other.ID)
	out.Reset()
	if err := a.seedDemo(ctx, &out, true); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "dilengkapi:") != len(theme.All())-1 || !strings.Contains(out.String(), "bukan undangan contoh") {
		t.Errorf("refresh: %s", out.String())
	}
	for _, u := range demoOld {
		if objectExists(demo.ID, u) {
			t.Errorf("refresh: objek lama masih ada: %s", u)
		}
	}
	if c, _ := a.weddings.GetCouple(ctx, demo.ID); c.GroomPhotoURL != nil || c.BridePhotoURL != nil {
		t.Errorf("refresh: potret masih terisi: %v/%v", c.GroomPhotoURL, c.BridePhotoURL)
	}
	assertStoryReplaced := func(id uuid.UUID) {
		t.Helper()
		list, _ := stories.ListStories(ctx, id)
		for _, s := range list {
			if s.PhotoURL == "" || slices.Contains(demoOld, s.PhotoURL) || !objectExists(id, s.PhotoURL) {
				t.Errorf("refresh: foto cerita %q belum diganti: %q", s.Title, s.PhotoURL)
			}
		}
	}
	assertStoryReplaced(demo.ID)
	// Bukan demo: objek, URL, dan galeri tetap.
	for _, u := range otherOld {
		if !objectExists(other.ID, u) {
			t.Errorf("refresh menghapus objek bukan demo: %s", u)
		}
	}
	if c, _ := a.weddings.GetCouple(ctx, other.ID); c.GroomPhotoURL == nil || *c.GroomPhotoURL != otherOld[0] || c.BridePhotoURL == nil || *c.BridePhotoURL != otherOld[1] {
		t.Errorf("refresh mengubah potret bukan demo: %v/%v", c.GroomPhotoURL, c.BridePhotoURL)
	}
	if list, _ := stories.ListStories(ctx, other.ID); len(list) != 3 || list[0].PhotoURL != otherOld[2] {
		t.Errorf("refresh mengubah cerita bukan demo: %+v", list)
	}
	if items, _ := photos.ListGallery(ctx, other.ID); len(items) != len(otherItems) || items[0].ID != otherItems[0].ID {
		t.Errorf("refresh mengubah galeri bukan demo")
	}
	if _, err := a.pool.Exec(ctx, "UPDATE weddings SET is_demo = true WHERE id = $1", other.ID); err != nil {
		t.Fatal(err)
	}
	// Refresh tetap idempoten.
	if err := a.seedDemo(ctx, io.Discard, true); err != nil {
		t.Fatalf("refresh ulang: %v", err)
	}
	if err := a.seedDemo(ctx, io.Discard, true); err != nil {
		t.Fatalf("refresh ulang: %v", err)
	}
	assertStoryPhotos()
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
	for _, d := range theme.All() {
		if _, ok := demoMedia[demoMediaAlias[d.ID]]; !ok {
			if _, ok := demoMedia[d.ID]; !ok {
				t.Errorf("tema %s tanpa set foto", d.ID)
			}
		}
	}
}
