package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/storage"
)

func TestMediaPrefix(t *testing.T) {
	cases := map[string]string{
		"https://abc.r2.cloudflarestorage.com": "https://abc.r2.cloudflarestorage.com/",
		"https://pub-1.r2.dev/":                "https://pub-1.r2.dev/",
		" https://media.lovoria.com/foto// ":   "https://media.lovoria.com/foto/",
	}
	for in, want := range cases {
		if got, err := mediaPrefix(in); err != nil || got != want {
			t.Errorf("mediaPrefix(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "/media", "ftp://x", "https://"} {
		if _, err := mediaPrefix(in); err == nil {
			t.Errorf("%q harus invalid", in)
		}
	}
}

// TestRebaseMediaURLs: foto di gallery, foto utama, foto pasangan, dan foto cerita
// yang tersimpan dengan basis URL lama dipindah ke basis baru; URL lain tidak tersentuh.
func TestRebaseMediaURLs(t *testing.T) {
	ctx := context.Background()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg, _ := config.LoadFrom(func(k string) string {
		return map[string]string{"APP_ENV": config.EnvTest, "STORAGE_LOCAL_DIR": t.TempDir()}[k]
	})
	a, err := newApp(ctx, cfg, log, pool)
	if err != nil {
		t.Fatal(err)
	}

	const oldBase = "https://acc.r2.cloudflarestorage.com"
	const newBase = "https://pub-123.r2.dev"
	oldStore, _ := storage.NewLocal(t.TempDir(), oldBase)

	u, _ := a.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: "u@example.com", Password: "password123"})
	w, _ := a.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})
	gal := gallery.NewService(gallery.NewRepository(pool), oldStore, a.weddings, 500<<20, log)
	var img bytes.Buffer
	_ = jpeg.Encode(&img, image.NewRGBA(image.Rect(0, 0, 20, 20)), nil)
	item, err := gal.Upload(ctx, w.ID, gallery.CategoryCover, bytes.NewReader(img.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	_ = gal.SetCover(ctx, w.ID, item.ID)
	_, _ = a.weddings.UpdateCouple(ctx, w.ID, wedding.CoupleInput{GroomName: "A", BrideName: "B", GroomPhotoURL: item.URL, BridePhotoURL: "https://cdn.lain.com/b.jpg"})
	stories := story.NewService(story.NewRepository(pool))
	st, _ := stories.CreateStory(ctx, w.ID, story.Input{Title: "Kenal", Year: "2019", PhotoURL: item.ThumbURL})

	// Simulasi: menghitung, tidak mengubah.
	var out bytes.Buffer
	if err := a.rebaseMediaURLs(ctx, []string{"--from", oldBase, "--to", newBase}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SIMULASI", "gallery_items:       1 baris", "weddings + couples:  2 baris", "love_stories:        1 baris", "--apply"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output simulasi tidak memuat %q:\n%s", want, out.String())
		}
	}
	if got, _ := gal.GetItem(ctx, w.ID, item.ID); !strings.HasPrefix(got.URL, oldBase) {
		t.Fatal("simulasi tidak boleh mengubah data")
	}

	// Terapkan.
	out.Reset()
	if err := a.rebaseMediaURLs(ctx, []string{"--from", oldBase + "/", "--to", newBase, "--apply"}, &out); err != nil {
		t.Fatal(err)
	}
	got, _ := gal.GetItem(ctx, w.ID, item.ID)
	if !strings.HasPrefix(got.URL, newBase+"/weddings/") || !strings.HasPrefix(got.ThumbURL, newBase+"/weddings/") {
		t.Errorf("gallery = %s / %s", got.URL, got.ThumbURL)
	}
	wd, _ := a.weddings.GetWedding(ctx, w.ID)
	c, _ := a.weddings.GetCouple(ctx, w.ID)
	s, _ := stories.GetStory(ctx, w.ID, st.ID)
	if *wd.MainPhotoURL != got.URL || *c.GroomPhotoURL != got.URL || s.PhotoURL != got.ThumbURL {
		t.Errorf("main=%s groom=%s story=%s", *wd.MainPhotoURL, *c.GroomPhotoURL, s.PhotoURL)
	}
	if *c.BridePhotoURL != "https://cdn.lain.com/b.jpg" {
		t.Errorf("URL eksternal ikut berubah: %s", *c.BridePhotoURL)
	}

	// Idempoten & validasi argumen.
	out.Reset()
	_ = a.rebaseMediaURLs(ctx, []string{"--from", oldBase, "--to", newBase, "--apply"}, &out)
	if !strings.Contains(out.String(), "gallery_items:       0 baris") {
		t.Errorf("jalan kedua harus 0 baris:\n%s", out.String())
	}
	if err := a.rebaseMediaURLs(ctx, []string{"--to", newBase}, &out); err == nil {
		t.Error("--from wajib")
	}
	if err := a.rebaseMediaURLs(ctx, []string{"--from", newBase, "--to", newBase + "/"}, &out); err == nil {
		t.Error("--from == --to harus ditolak")
	}
}
