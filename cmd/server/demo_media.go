package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"strconv"

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

// Per tema: pasangan yang sama untuk sampul, potret mempelai
// (demomedia/<tema>-groom.jpg, -bride.jpg), dan galeri + foto detail.
var demoMedia = map[string]demoSet{
	"signature": {Cover: "VPFnktO9TA8.jpg", Gallery: []demoPhoto{
		{"bwo_dSnXuvU.jpg", gallery.CategoryPrewedding, ""},
		{"4LRIIA-24SI.jpg", gallery.CategoryPrewedding, ""},
		{"l0iyOIhrZVw.jpg", gallery.CategoryPrewedding, "Busana adat Karo"},
		{"l4EofTUIANg.jpg", gallery.CategoryPrewedding, ""},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"4LRIIA-24SI.jpg", "bwo_dSnXuvU.jpg", "l0iyOIhrZVw.jpg"}},
	"elegant": {Cover: "Lr834aonJ70.jpg", Gallery: []demoPhoto{
		{"Lr834aonJ70.jpg", gallery.CategoryPrewedding, ""},
		{"BFqxGLypaRo.jpg", gallery.CategoryPrewedding, "Busana adat Sunda"},
		{"qVtjqnWYka8.jpg", gallery.CategoryWedding, ""},
		{"RHfYB_USSEc.jpg", gallery.CategoryWedding, "Tukar cincin"},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, "Pelaminan"},
	}, Stories: []string{"BFqxGLypaRo.jpg", "Lr834aonJ70.jpg", "RHfYB_USSEc.jpg"}},
	"romantic": {Cover: "0cMP8HuJ6zk.jpg", Gallery: []demoPhoto{
		{"0cMP8HuJ6zk.jpg", gallery.CategoryPrewedding, ""},
		{"M2T1j-6Fn8w.jpg", gallery.CategoryWedding, ""},
		{"Ko6XKSsO4w8.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, "Buket bunga"},
		{"B_R3rmJPeSE.jpg", gallery.CategoryWedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, "Dekorasi resepsi"},
	}, Stories: []string{"0cMP8HuJ6zk.jpg", "Ko6XKSsO4w8.jpg", "M2T1j-6Fn8w.jpg"}},
	"modern": {Cover: "1cyUquII5Zg.jpg", Gallery: []demoPhoto{
		{"1cyUquII5Zg.jpg", gallery.CategoryPrewedding, ""},
		{"UjLfNJa-KY8.jpg", gallery.CategoryPrewedding, ""},
		{"duNBz_LBaEc.jpg", gallery.CategoryWedding, ""},
		{"5BB_atDT4oA.jpg", gallery.CategoryWedding, ""},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"UjLfNJa-KY8.jpg", "1cyUquII5Zg.jpg", "5BB_atDT4oA.jpg"}},
	"minimal": {Cover: "4rtHl_X5GNo.jpg", Gallery: []demoPhoto{
		{"4rtHl_X5GNo.jpg", gallery.CategoryPrewedding, ""},
		{"IfjHaIoAoqE.jpg", gallery.CategoryPrewedding, ""},
		{"5BB_atDT4oA.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, ""},
		{"rMO7SR9Z6RQ.jpg", gallery.CategoryPrewedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, ""},
	}, Stories: []string{"rMO7SR9Z6RQ.jpg", "IfjHaIoAoqE.jpg", "4rtHl_X5GNo.jpg"}},
}

// demoMediaAlias: tema yang belum punya set foto sendiri meminjam set tema lain
// (Koleksi Daerah, T28 — foto berbusana adat daerah masing-masing menyusul).
var demoMediaAlias = map[string]string{"jawa": "minimal", "sunda": "elegant", "minang": "signature", "bali": "romantic", "bugis": "modern"}

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

// ensureDemoMedia melengkapi foto undangan contoh: sampul, potret mempelai, dan
// galeri. Dilewati bila galeri sudah berisi (idempoten; demo lama tanpa foto
// ikut dilengkapi saat seed dijalankan ulang). added = foto baru diunggah.
func (a *app) ensureDemoMedia(ctx context.Context, photos *gallery.Service, w wedding.Wedding, themeID string) (added bool, err error) {
	src := demoMediaSource(themeID)
	set := demoMedia[src]
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

	cover, err := upload(set.Cover, gallery.CategoryCover, "")
	if err != nil {
		return false, err
	}
	if err := photos.SetCover(ctx, w.ID, cover.ID); err != nil {
		return false, err
	}
	groom, err := a.putDemoPhoto(ctx, w.ID, "couple", src+"-groom.jpg")
	if err != nil {
		return false, err
	}
	bride, err := a.putDemoPhoto(ctx, w.ID, "couple", src+"-bride.jpg")
	if err != nil {
		return false, err
	}
	c, err := a.weddings.GetCouple(ctx, w.ID)
	if err != nil {
		return false, err
	}
	if _, err := a.weddings.UpdateCouple(ctx, w.ID, wedding.CoupleInput{
		GroomName: c.GroomName, BrideName: c.BrideName, GroomDescription: c.GroomDescription, BrideDescription: c.BrideDescription,
		GroomPhotoURL: groom, BridePhotoURL: bride,
	}); err != nil {
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
		in := story.Input{Year: strconv.Itoa(s.Date.Year), Title: s.Title, Description: s.Description, PhotoURL: photo}
		if s.Date.Month > 0 {
			in.Month = strconv.Itoa(s.Date.Month)
		}
		if s.Date.Day > 0 {
			in.Day = strconv.Itoa(s.Date.Day)
		}
		if _, err := stories.UpdateStory(ctx, weddingID, s.ID, in); err != nil {
			return added, err
		}
		added = true
	}
	return added, nil
}

// putDemoPhoto menyimpan foto mempelai / cerita langsung ke storage (R2 di
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
