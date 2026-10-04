package main

import (
	"bytes"
	"context"
	"embed"
	"fmt"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
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
}

// Per tema: pasangan yang sama untuk sampul, potret mempelai
// (demomedia/<tema>-groom.jpg, -bride.jpg), dan galeri + foto detail.
var demoMedia = map[string]demoSet{
	"signature": {Cover: "l4EofTUIANg.jpg", Gallery: []demoPhoto{
		{"bwo_dSnXuvU.jpg", gallery.CategoryPrewedding, ""},
		{"4LRIIA-24SI.jpg", gallery.CategoryPrewedding, ""},
		{"l0iyOIhrZVw.jpg", gallery.CategoryPrewedding, "Busana adat Karo"},
		{"VPFnktO9TA8.jpg", gallery.CategoryPrewedding, ""},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, ""},
	}},
	"elegant": {Cover: "Lr834aonJ70.jpg", Gallery: []demoPhoto{
		{"Lr834aonJ70.jpg", gallery.CategoryPrewedding, ""},
		{"BFqxGLypaRo.jpg", gallery.CategoryPrewedding, "Busana adat Sunda"},
		{"qVtjqnWYka8.jpg", gallery.CategoryWedding, ""},
		{"RHfYB_USSEc.jpg", gallery.CategoryWedding, "Tukar cincin"},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, "Pelaminan"},
	}},
	"romantic": {Cover: "0cMP8HuJ6zk.jpg", Gallery: []demoPhoto{
		{"0cMP8HuJ6zk.jpg", gallery.CategoryPrewedding, ""},
		{"M2T1j-6Fn8w.jpg", gallery.CategoryWedding, ""},
		{"Ko6XKSsO4w8.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, "Buket bunga"},
		{"B_R3rmJPeSE.jpg", gallery.CategoryWedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, "Dekorasi resepsi"},
	}},
	"modern": {Cover: "1cyUquII5Zg.jpg", Gallery: []demoPhoto{
		{"1cyUquII5Zg.jpg", gallery.CategoryPrewedding, ""},
		{"UjLfNJa-KY8.jpg", gallery.CategoryPrewedding, ""},
		{"duNBz_LBaEc.jpg", gallery.CategoryWedding, ""},
		{"5BB_atDT4oA.jpg", gallery.CategoryWedding, ""},
		{"ZYet8yoepik.jpg", gallery.CategoryWedding, ""},
		{"4FGhD_iGuqg.jpg", gallery.CategoryWedding, ""},
	}},
	"minimal": {Cover: "nXmt5zQ7l0c.jpg", Gallery: []demoPhoto{
		{"nXmt5zQ7l0c.jpg", gallery.CategoryPrewedding, ""},
		{"IfjHaIoAoqE.jpg", gallery.CategoryPrewedding, ""},
		{"5BB_atDT4oA.jpg", gallery.CategoryWedding, ""},
		{"UQl_-yabQiA.jpg", gallery.CategoryWedding, ""},
		{"RHfYB_USSEc.jpg", gallery.CategoryWedding, ""},
		{"qG2yK_iNspE.jpg", gallery.CategoryWedding, ""},
	}},
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
	set, ok := demoMedia[themeID]
	if !ok {
		set = demoMedia["elegant"]
	}
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
	groom, err := a.putDemoPortrait(ctx, w.ID, themeID+"-groom.jpg")
	if err != nil {
		return false, err
	}
	bride, err := a.putDemoPortrait(ctx, w.ID, themeID+"-bride.jpg")
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

// putDemoPortrait menyimpan potret mempelai langsung ke storage (R2 di
// produksi), bukan sebagai item galeri — supaya galeri contoh hanya berisi foto
// pilihan, tidak mengulang potongan potret. Demo tidak memakai kuota paket.
func (a *app) putDemoPortrait(ctx context.Context, weddingID uuid.UUID, file string) (string, error) {
	f, err := demoMediaFS.Open("demomedia/" + file)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	res, err := imageproc.Process(f)
	if err != nil {
		return "", fmt.Errorf("foto %s: %w", file, err)
	}
	key := fmt.Sprintf("weddings/%s/couple/%s.jpg", weddingID, db.NewID())
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
