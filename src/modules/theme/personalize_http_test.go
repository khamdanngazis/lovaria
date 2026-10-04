package theme_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
)

// personalForm: isian form tema lengkap (semua bagian tampil kecuali hidden).
func personalForm(order []string, hidden ...string) url.Values {
	f := url.Values{"theme_id": {"elegant"}, "_method": {"PATCH"}, "section_order": order}
	for _, d := range theme.Sections {
		skip := false
		for _, h := range hidden {
			skip = skip || h == d.ID
		}
		if d.Hideable && !skip {
			f.Add("visible_sections", d.ID)
		}
	}
	return f
}

func TestPersonalizationSaveAndPreview(t *testing.T) {
	a := newApp(t)
	ctx := context.Background()
	owner, w := a.newWedding(t, "a@example.com")
	base := w.DashboardURL("/theme")

	page := a.do(owner, http.MethodGet, base, nil).Body.String()
	for _, want := range []string{"Teks pembuka", "Kutipan / ayat", "QS. Ar-Rum: 21", "Musik latar", "Susunan bagian", "Naikkan Hitung mundur", "Unggah musik sendiri"} {
		if !strings.Contains(page, want) {
			t.Errorf("halaman tema tidak memuat %q", want)
		}
	}

	order := []string{"quote", "couple", "countdown", "events", "story", "gallery", "rsvp", "guestbook", "gift"}
	f := personalForm(order, "story", "gift")
	f.Set("greeting", "Teruntuk sahabat kami")
	f.Set("closing", "Sampai jumpa!")
	f.Set("quote_text", "Kasih itu sabar")
	f.Set("quote_source", "1 Korintus 13:4")
	rec := a.do(owner, http.MethodPost, base, f)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("simpan: %d %s", rec.Code, rec.Body.String())
	}
	st, _ := a.themes.Settings(ctx, w.ID)
	if st.Greeting != "Teruntuk sahabat kami" || st.Closing != "Sampai jumpa!" || st.QuoteText != "Kasih itu sabar" || st.QuoteSource != "1 Korintus 13:4" ||
		!reflect.DeepEqual(st.SectionOrder, order) || !reflect.DeepEqual(st.HiddenSections, []string{"story", "gift"}) {
		t.Fatalf("settings = %+v", st)
	}
	// Tanpa JS: tombol naik/turun menggeser lalu menyimpan.
	f.Set("move", "couple:up")
	rec = a.do(owner, http.MethodPost, base, f)
	if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "#bagian") {
		t.Fatalf("move: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if st, _ = a.themes.Settings(ctx, w.ID); st.SectionOrder[0] != "couple" || st.SectionOrder[1] != "quote" {
		t.Errorf("setelah naik: %v", st.SectionOrder)
	}
	// Validasi: kutipan terlalu panjang & bagian wajib tidak bisa disembunyikan.
	bad := personalForm(order)
	bad.Set("quote_text", strings.Repeat("x", 501))
	if rec := a.do(owner, http.MethodPost, base, bad); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Kutipan maksimal 500 karakter") {
		t.Errorf("kutipan panjang: %d", rec.Code)
	}

	// Preview mencerminkan isian form (belum disimpan): urutan, bagian tersembunyi, teks.
	q := personalForm([]string{"gallery", "quote", "couple", "countdown", "events", "story", "rsvp", "guestbook", "gift"}, "guestbook")
	q.Del("_method")
	q.Set("quote_text", "Kutipan preview")
	q.Set("greeting", "Sapaan preview")
	html := a.do(owner, http.MethodGet, base+"/preview?"+q.Encode(), nil).Body.String()
	if !strings.Contains(html, "Kutipan preview") || !strings.Contains(html, "Sapaan preview") || strings.Contains(html, `id="guestbook"`) ||
		strings.Index(html, `id="quote"`) > strings.Index(html, `id="couple"`) {
		t.Error("preview tidak mencerminkan pengaturan")
	}
	// Preview tanpa query memakai pengaturan tersimpan (story & gift disembunyikan).
	if html := a.do(owner, http.MethodGet, base+"/preview", nil).Body.String(); strings.Contains(html, `id="story"`) || !strings.Contains(html, "Teruntuk sahabat kami") {
		t.Error("preview tersimpan salah")
	}
}

// mp3 membuat berkas "MP3" kecil (tag ID3 + frame sync) sebesar n byte.
func mp3(n int) []byte {
	b := make([]byte, n)
	copy(b, "ID3\x03\x00\x00\x00\x00\x00\x00\xff\xfb")
	return b
}

func (a app) upload(user uuid.UUID, path, filename string, data []byte) *httptest.ResponseRecorder {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, path, &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("X-Test-User", user.String())
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, r)
	return rec
}

func TestMusicLibraryAndUpload(t *testing.T) {
	old := theme.MusicLibrary
	theme.MusicLibrary = []theme.Track{{ID: "sunrise", Title: "Sunrise", Artist: "Lunovia", Duration: "2:30", File: "sunrise.mp3", License: "CC0"}}
	t.Cleanup(func() { theme.MusicLibrary = old })
	a := newApp(t)
	ctx := context.Background()
	owner, w := a.newWedding(t, "a@example.com")
	bob, wb := a.newWedding(t, "b@example.com")
	base := w.DashboardURL("/theme")
	used := func(id uuid.UUID) int64 {
		got, _ := a.weddings.GetWedding(ctx, id)
		return got.StorageUsedBytes
	}

	// Lagu bawaan bisa dipilih; URL asing ditolak.
	lib := a.gallery.PublicURL("music/sunrise.mp3")
	if _, err := a.themes.Save(ctx, w.ID, "elegant", view.Settings{MusicURL: lib, MusicEnabled: true}); err != nil {
		t.Fatalf("lagu bawaan: %v", err)
	}
	f := personalForm(theme.SectionIDs)
	f.Set("music_url", "https://evil.example/lagu.mp3")
	f.Set("music_enabled", "1")
	if rec := a.do(owner, http.MethodPost, base, f); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Pilih lagu dari pustaka") {
		t.Errorf("URL asing: %d", rec.Code)
	}
	if page := a.do(owner, http.MethodGet, base, nil).Body.String(); !strings.Contains(page, "Sunrise") || !strings.Contains(page, `src="`+lib+`"`) {
		t.Error("pustaka tidak tampil di halaman tema")
	}

	// Unggahan: bukan MP3 / > 8 MB ditolak tanpa memakai kuota.
	if rec := a.upload(owner, base+"/music", "lagu.mp3", []byte("<html>bukan musik</html>")); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Berkas harus MP3") {
		t.Errorf("bukan MP3: %d", rec.Code)
	}
	if rec := a.upload(owner, base+"/music", "besar.mp3", mp3(gallery.MaxAudioBytes+1)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "maksimal 8 MB") {
		t.Errorf("> 8 MB: %d", rec.Code)
	}
	if used(w.ID) != 0 {
		t.Fatalf("kuota terpakai setelah upload gagal: %d", used(w.ID))
	}

	// Unggahan sah: dipilih & diaktifkan, kuota bertambah.
	if rec := a.upload(owner, base+"/music", "lagu.mp3", mp3(40_000)); rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	m, _ := a.themes.Music(ctx, w.ID)
	st, _ := a.themes.Settings(ctx, w.ID)
	if m.UploadURL == "" || st.MusicURL != m.UploadURL || !st.MusicEnabled || used(w.ID) != 40_000 {
		t.Fatalf("setelah upload: music=%+v settings=%+v used=%d", m, st, used(w.ID))
	}
	// Ganti unggahan: berkas lama dihapus, kuota hanya untuk yang baru.
	a.upload(owner, base+"/music", "lagu2.mp3", mp3(25_000))
	if m2, _ := a.themes.Music(ctx, w.ID); m2.UploadURL == m.UploadURL || used(w.ID) != 25_000 {
		t.Errorf("ganti: used=%d", used(w.ID))
	}
	m, _ = a.themes.Music(ctx, w.ID)

	// Wedding lain tidak boleh memakai unggahan wedding ini.
	if _, err := a.themes.Save(ctx, wb.ID, "elegant", view.Settings{MusicURL: m.UploadURL, MusicEnabled: true}); err == nil {
		t.Error("wedding lain memakai unggahan wedding ini")
	}
	if rec := a.do(bob, http.MethodDelete, base+"/music", nil); rec.Code != http.StatusNotFound || used(w.ID) != 25_000 {
		t.Errorf("bob hapus: %d", rec.Code)
	}

	// Kuota penuh → ditolak.
	a.gallery.SetQuotaSource(func(context.Context, uuid.UUID) (int64, bool, error) { return 30_000, true, nil })
	if rec := a.upload(owner, base+"/music", "lagu3.mp3", mp3(10_000)); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Kuota penyimpanan") {
		t.Errorf("kuota penuh: %d", rec.Code)
	}
	if used(w.ID) != 25_000 {
		t.Errorf("kuota berubah setelah ditolak: %d", used(w.ID))
	}

	// Hapus unggahan: kuota kembali, musik yang memakainya dikosongkan.
	if rec := a.do(owner, http.MethodDelete, base+"/music", nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("hapus: %d", rec.Code)
	}
	st, _ = a.themes.Settings(ctx, w.ID)
	if used(w.ID) != 0 || st.MusicURL != "" {
		t.Errorf("setelah hapus: used=%d music=%q", used(w.ID), st.MusicURL)
	}
}
