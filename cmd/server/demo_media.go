package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/imageproc"
)

// Foto undangan contoh (Unsplash License, lihat demomedia/CREDITS.md). Diunggah
// lewat gallery.Upload seperti foto pasangan sungguhan, jadi di produksi
// tersimpan di R2 — bukan disajikan dari binary.
//
//go:embed demomedia/*.jpg
var demoMediaFS embed.FS

type demoPhoto struct {
	File, Category, Caption string
}

type demoSet struct {
	Cover   string // foto sampul pembuka (juga foto utama wedding / OG)
	Gallery []demoPhoto
	// Stories: foto tiap bab cerita contoh, sesuai urutan bab (bertemu, memulai
	// perjalanan, lamaran).
	Stories []string
}

// Set foto per karakter tema. Semua foto tanpa wajah yang bisa dikenali (aturan
// di doc/runbook.md); urutan Stories: bertemu, memulai perjalanan, lamaran.
var demoMedia = map[string]demoSet{
	"signature": {Cover: "XiiexZ2jjrI.jpg", Gallery: []demoPhoto{
		{"464ps_nOflw.jpg", gallery.CategoryPrewedding, ""},
		{"BeHFdxxL6E8.jpg", gallery.CategoryPrewedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, ""},
		{"XRzUf1SGWEI.jpg", gallery.CategoryPrewedding, ""},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"qHjwSGv2p8c.jpg", "IfjHaIoAoqE.jpg", "M2T1j-6Fn8w.jpg"}},
	"elegant": {Cover: "UQl_-yabQiA.jpg", Gallery: []demoPhoto{
		{"wki4KKlMGvc.jpg", gallery.CategoryPrewedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, "Pelaminan"},
		{"RHfYB_USSEc.jpg", gallery.CategoryWedding, "Tukar cincin"},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"BeHFdxxL6E8.jpg", gallery.CategoryPrewedding, ""},
		{"Ko6XKSsO4w8.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"XRzUf1SGWEI.jpg", "XiiexZ2jjrI.jpg", "RHfYB_USSEc.jpg"}},
	"romantic": {Cover: "Ko6XKSsO4w8.jpg", Gallery: []demoPhoto{
		{"M2T1j-6Fn8w.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, "Buket bunga"},
		{"B_R3rmJPeSE.jpg", gallery.CategoryWedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, "Dekorasi resepsi"},
		{"qHjwSGv2p8c.jpg", gallery.CategoryPrewedding, ""},
		{"464ps_nOflw.jpg", gallery.CategoryPrewedding, ""},
	}, Stories: []string{"XRzUf1SGWEI.jpg", "IfjHaIoAoqE.jpg", "M2T1j-6Fn8w.jpg"}},
	"modern": {Cover: "l_ecNnA4xp0.jpg", Gallery: []demoPhoto{
		{"5BB_atDT4oA.jpg", gallery.CategoryPrewedding, ""},
		{"duNBz_LBaEc.jpg", gallery.CategoryPrewedding, ""},
		{"SUwPo4ErQCc.jpg", gallery.CategoryWedding, ""},
		{"B_R3rmJPeSE.jpg", gallery.CategoryWedding, ""},
		{"OJ6LOiMcQ5k.jpg", gallery.CategoryWedding, ""},
		{"fCwagrCDsXE.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"duNBz_LBaEc.jpg", "5BB_atDT4oA.jpg", "ZYet8yoepik.jpg"}},
	"minimal": {Cover: "IfjHaIoAoqE.jpg", Gallery: []demoPhoto{
		{"XiiexZ2jjrI.jpg", gallery.CategoryPrewedding, ""},
		{"nHjB5F1CMLE.jpg", gallery.CategoryPrewedding, ""},
		{"qHjwSGv2p8c.jpg", gallery.CategoryWedding, ""},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"wki4KKlMGvc.jpg", "464ps_nOflw.jpg", "RHfYB_USSEc.jpg"}},
	// Koleksi Daerah (T28): detail busana, inai, dan kain.
	"adat": {Cover: "RHfYB_USSEc.jpg", Gallery: []demoPhoto{
		{"nHjB5F1CMLE.jpg", gallery.CategoryPrewedding, ""},
		{"BeHFdxxL6E8.jpg", gallery.CategoryPrewedding, ""},
		{"lKwp3-FQomY.jpg", gallery.CategoryWedding, ""},
		{"IpORRK_X_gU.jpg", gallery.CategoryPrewedding, ""},
		{"SgY_iQkAe4w.jpg", gallery.CategoryPrewedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, "Dekorasi resepsi"},
	}, Stories: []string{"lKwp3-FQomY.jpg", "wki4KKlMGvc.jpg", "ZYet8yoepik.jpg"}},
	"adat-gelap": {Cover: "SUwPo4ErQCc.jpg", Gallery: []demoPhoto{
		{"OJ6LOiMcQ5k.jpg", gallery.CategoryWedding, ""},
		{"fCwagrCDsXE.jpg", gallery.CategoryPrewedding, ""},
		{"lKwp3-FQomY.jpg", gallery.CategoryWedding, ""},
		{"B_R3rmJPeSE.jpg", gallery.CategoryWedding, ""},
		{"SgY_iQkAe4w.jpg", gallery.CategoryPrewedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"lKwp3-FQomY.jpg", "duNBz_LBaEc.jpg", "M2T1j-6Fn8w.jpg"}},
}

// demoMediaAlias: tema yang tidak punya set foto sendiri meminjam set lain.
var demoMediaAlias = map[string]string{"jawa": "adat", "sunda": "adat", "bali": "adat", "minang": "adat-gelap", "bugis": "adat-gelap"}

// demoCoverOverride: tema yang berbagi set tetap punya sampul sendiri, supaya
// etalase tema tidak terlihat sama.
var demoCoverOverride = map[string]string{"jawa": "BeHFdxxL6E8.jpg", "sunda": "lKwp3-FQomY.jpg", "minang": "B_R3rmJPeSE.jpg"}

// demoMediaSource: nama set foto untuk sebuah tema (cadangan: elegant).
func demoMediaSource(themeID string) string {
	if a, ok := demoMediaAlias[themeID]; ok {
		themeID = a
	}
	if _, ok := demoMedia[themeID]; !ok {
		return "elegant"
	}
	return themeID
}

// demoSettings: personalisasi undangan contoh (kutipan, T20).
func demoSettings() view.Settings {
	q := theme.QuoteSamples[0]
	return view.Settings{QuoteText: q.Text, QuoteSource: q.Source}
}

// ensureDemoMedia melengkapi foto undangan contoh: sampul dan galeri. Mempelai
// contoh tidak berfoto. Dilewati bila galeri sudah berisi (idempoten; demo lama
// tanpa foto ikut dilengkapi saat seed dijalankan ulang). added = foto baru diunggah.
func ensureDemoMedia(ctx context.Context, photos *gallery.Service, w wedding.Wedding, themeID string) (added bool, err error) {
	set := demoMedia[demoMediaSource(themeID)]
	items, err := photos.ListGallery(ctx, w.ID)
	if err != nil || len(items) > 0 {
		return false, err
	}
	upload := func(file, category, caption string) (gallery.Item, error) {
		f, err := demoMediaFS.Open("demomedia/" + file)
		if err != nil {
			return gallery.Item{}, err
		}
		defer func() { _ = f.Close() }()
		it, err := photos.Upload(ctx, w.ID, category, f)
		if err != nil {
			return it, fmt.Errorf("foto %s: %w", file, err)
		}
		if caption != "" {
			return photos.UpdateItem(ctx, w.ID, it.ID, caption, category)
		}
		return it, nil
	}

	coverFile := set.Cover
	if f, ok := demoCoverOverride[themeID]; ok {
		coverFile = f
	}
	cover, err := upload(coverFile, gallery.CategoryCover, "")
	if err != nil {
		return false, err
	}
	if err := photos.SetCover(ctx, w.ID, cover.ID); err != nil {
		return false, err
	}
	for _, p := range set.Gallery {
		if _, err := upload(p.File, p.Category, p.Caption); err != nil {
			return false, err
		}
	}
	return true, nil
}

// clearDemoMedia menghapus semua foto galeri demo (objek storage & kuotanya
// ikut dilepas) supaya ensureDemoMedia mengisinya lagi dengan set terbaru.
func clearDemoMedia(ctx context.Context, photos *gallery.Service, weddingID uuid.UUID) error {
	items, err := photos.ListGallery(ctx, weddingID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := photos.DeleteItem(ctx, weddingID, it.ID); err != nil {
			return err
		}
	}
	return nil
}

// clearDemoPhotos (--refresh) menghapus potret mempelai dan foto cerita demo dari
// storage lalu mengosongkan URL-nya. Hanya untuk wedding demo, supaya foto
// pelanggan tidak mungkin terhapus.
func (a *app) clearDemoPhotos(ctx context.Context, stories *story.Service, w wedding.Wedding) error {
	if !w.IsDemo {
		return fmt.Errorf("wedding %s bukan demo", w.ID)
	}
	c, err := a.weddings.GetCouple(ctx, w.ID)
	if err != nil {
		return err
	}
	for _, u := range []*string{c.GroomPhotoURL, c.BridePhotoURL} {
		if u != nil {
			if err := a.deleteDemoObject(ctx, w.ID, *u); err != nil {
				return err
			}
		}
	}
	if _, err := a.weddings.UpdateCouple(ctx, w.ID, wedding.CoupleInput{
		GroomName: c.GroomName, BrideName: c.BrideName, GroomDescription: c.GroomDescription, BrideDescription: c.BrideDescription,
	}); err != nil {
		return err
	}
	list, err := stories.ListStories(ctx, w.ID)
	if err != nil {
		return err
	}
	for _, s := range list {
		if s.PhotoURL == "" {
			continue
		}
		if err := a.deleteDemoObject(ctx, w.ID, s.PhotoURL); err != nil {
			return err
		}
		if _, err := stories.UpdateStory(ctx, w.ID, s.ID, demoStoryInput(s, "")); err != nil {
			return err
		}
	}
	return nil
}

// deleteDemoObject menghapus objek storage di balik URL publik. URL di luar
// awalan wedding ini dibiarkan: bukan objek milik demo tersebut.
func (a *app) deleteDemoObject(ctx context.Context, weddingID uuid.UUID, url string) error {
	key, ok := demoObjectKey(weddingID, url)
	if !ok {
		return nil
	}
	return a.store.Delete(ctx, key)
}

// demoObjectKey: kunci objek dari URL publik (weddings/<id>/...), diambil dari
// awalannya supaya tetap benar bila domain R2 pernah berganti.
func demoObjectKey(weddingID uuid.UUID, url string) (string, bool) {
	i := strings.Index(url, fmt.Sprintf("weddings/%s/", weddingID))
	if i < 0 {
		return "", false
	}
	key, _, _ := strings.Cut(url[i:], "?")
	return key, true
}

// ensureDemoStoryPhotos: foto bab cerita contoh yang belum punya foto (idempoten;
// demo lama ikut dilengkapi). added = ada foto baru.
func (a *app) ensureDemoStoryPhotos(ctx context.Context, stories *story.Service, weddingID uuid.UUID, themeID string) (added bool, err error) {
	files := demoMedia[demoMediaSource(themeID)].Stories
	list, err := stories.ListStories(ctx, weddingID)
	if err != nil {
		return false, err
	}
	for i, s := range list {
		if s.PhotoURL != "" || i >= len(files) {
			continue
		}
		photo, err := a.putDemoPhoto(ctx, weddingID, "stories", files[i])
		if err != nil {
			return added, err
		}
		if _, err := stories.UpdateStory(ctx, weddingID, s.ID, demoStoryInput(s, photo)); err != nil {
			return added, err
		}
		added = true
	}
	return added, nil
}

// demoStoryInput: isi bab cerita yang sama dengan foto yang diganti.
func demoStoryInput(s story.Story, photo string) story.Input {
	in := story.Input{Year: strconv.Itoa(s.Date.Year), Title: s.Title, Description: s.Description, PhotoURL: photo}
	if s.Date.Month > 0 {
		in.Month = strconv.Itoa(s.Date.Month)
	}
	if s.Date.Day > 0 {
		in.Day = strconv.Itoa(s.Date.Day)
	}
	return in
}

// putDemoPhoto menyimpan foto cerita langsung ke storage (R2 di
// produksi), bukan sebagai item galeri — supaya galeri contoh hanya berisi foto
// pilihan, tidak mengulang foto yang sama. Demo tidak memakai kuota paket.
func (a *app) putDemoPhoto(ctx context.Context, weddingID uuid.UUID, folder, file string) (string, error) {
	f, err := demoMediaFS.Open("demomedia/" + file)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	res, err := imageproc.Process(f)
	if err != nil {
		return "", fmt.Errorf("foto %s: %w", file, err)
	}
	key := fmt.Sprintf("weddings/%s/%s/%s.jpg", weddingID, folder, db.NewID())
	if err := a.store.Put(ctx, key, bytes.NewReader(res.Main), int64(len(res.Main)), imageproc.ContentType); err != nil {
		return "", err
	}
	return a.store.PublicURL(key), nil
}

// ensureDemoSettings: kutipan contoh untuk demo yang belum punya (demo lama).
func ensureDemoSettings(ctx context.Context, themes *theme.Service, id uuid.UUID, themeID string) error {
	st, err := themes.Settings(ctx, id)
	if err != nil || st.QuoteText != "" {
		return err
	}
	d := demoSettings()
	st.QuoteText, st.QuoteSource = d.QuoteText, d.QuoteSource
	_, err = themes.Save(ctx, id, themeID, st)
	return err
}
