package gallery

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

func newTestServer(t *testing.T, f fixture) *echo.Echo {
	t.Helper()
	cfg, _ := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	e := server.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := uuid.Parse(c.Request().Header.Get("X-Test-User")); err == nil {
				r := c.Request()
				c.SetRequest(r.WithContext(web.WithUser(r.Context(), web.User{ID: id, Name: "T"})))
			}
			return next(c)
		}
	})
	owned := wedding.Register(e.Group("/dashboard/weddings"), wedding.Deps{Service: f.weddings})
	Register(owned, Deps{Service: f.svc, Weddings: f.weddings})
	return e
}

func send(e *echo.Echo, user uuid.UUID, r *http.Request, htmx bool) *httptest.ResponseRecorder {
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("X-Test-User", user.String())
	if htmx {
		r.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

func uploadReq(t *testing.T, path, category string, data []byte, accept string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("category", category)
	fw, _ := mw.CreateFormFile("file", "foto.jpg")
	_, _ = fw.Write(data)
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, path, &body)
	r.Header.Set(echo.HeaderContentType, mw.FormDataContentType())
	if accept != "" {
		r.Header.Set(echo.HeaderAccept, accept)
	}
	return r
}

func formReq(method, path string, form url.Values) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
	r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	return r
}

func TestUploadViaHTTP(t *testing.T) {
	f := newFixture(t, nil, 500<<20)
	e := newTestServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	items := w.DashboardURL("/gallery/items")

	// JSON (pengunggah JS / ImageUpload).
	rec := send(e, owner, uploadReq(t, items, CategoryCouple, photo(900, 600), "application/json"), false)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var got itemJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.URL == "" || got.Width != 900 || got.Category != CategoryCouple {
		t.Errorf("json = %+v err=%v", got, err)
	}

	// File teks yang di-rename .jpg → 422 dengan pesan.
	rec = send(e, owner, uploadReq(t, items, CategoryWedding, []byte("bukan gambar"), "application/json"), false)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "bukan gambar JPG") {
		t.Errorf("fake jpg: %d %s", rec.Code, rec.Body.String())
	}
	// Tanpa file.
	rec = send(e, owner, formReq(http.MethodPost, items, url.Values{"category": {"wedding"}}), false)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("tanpa file: %d", rec.Code)
	}
	// > 11 MB → 413 sebelum diproses.
	rec = send(e, owner, uploadReq(t, items, CategoryWedding, bytes.Repeat([]byte{1}, 11<<20+1), "application/json"), false)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("11MB+: %d", rec.Code)
	}
	// Tanpa JS → redirect ke halaman gallery.
	rec = send(e, owner, uploadReq(t, items, CategoryWedding, photo(100, 100), ""), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != w.DashboardURL("/gallery") {
		t.Errorf("no-JS: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	list, _ := f.svc.ListGallery(ctx, w.ID)
	if len(list) != 2 {
		t.Fatalf("items = %d", len(list))
	}
	page := send(e, owner, httptest.NewRequest(http.MethodGet, w.DashboardURL("/gallery"), nil), false)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), list[0].ThumbURL) || !strings.Contains(page.Body.String(), "Penyimpanan terpakai") {
		t.Errorf("halaman gallery: %d", page.Code)
	}

	// Jadikan foto utama → penanda muncul & wedding ter-update.
	rec = send(e, owner, formReq(http.MethodPost, w.DashboardURL("/gallery/items/"+list[0].ID.String()+"/cover"), url.Values{}), true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Foto utama") {
		t.Errorf("cover: %d", rec.Code)
	}
	// Ubah keterangan & kategori.
	rec = send(e, owner, formReq(http.MethodPatch, w.DashboardURL("/gallery/items/"+list[1].ID.String()), url.Values{"caption": {"Resepsi"}, "category": {"wedding"}}), true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Resepsi") {
		t.Errorf("update: %d", rec.Code)
	}
	// Hapus.
	rec = send(e, owner, httptest.NewRequest(http.MethodDelete, w.DashboardURL("/gallery/items/"+list[1].ID.String()), nil), true)
	if rec.Code != http.StatusOK {
		t.Errorf("delete: %d", rec.Code)
	}
	if n, _ := f.svc.ListGallery(ctx, w.ID); len(n) != 1 {
		t.Errorf("setelah hapus = %d", len(n))
	}
}

func TestGalleryRoutesOwnerOnly(t *testing.T) {
	f := newFixture(t, nil, 500<<20)
	e := newTestServer(t, f)
	_, w := f.newWedding(t, "alice@example.com")
	bob, wb := f.newWedding(t, "bob@example.com")
	it, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(photo(80, 80)))
	if err != nil {
		t.Fatal(err)
	}
	item := w.DashboardURL("/gallery/items/" + it.ID.String())

	reqs := []*http.Request{
		httptest.NewRequest(http.MethodGet, w.DashboardURL("/gallery"), nil),
		uploadReq(t, w.DashboardURL("/gallery/items"), CategoryWedding, photo(10, 10), "application/json"),
		formReq(http.MethodPatch, item, url.Values{"caption": {"x"}, "category": {"wedding"}}),
		httptest.NewRequest(http.MethodDelete, item, nil),
		formReq(http.MethodPatch, item+"/position", url.Values{"direction": {"up"}}),
		formReq(http.MethodPost, item+"/cover", url.Values{}),
		// ID foto alice lewat URL wedding bob sendiri.
		httptest.NewRequest(http.MethodDelete, wb.DashboardURL("/gallery/items/"+it.ID.String()), nil),
		formReq(http.MethodPost, wb.DashboardURL("/gallery/items/"+it.ID.String()+"/cover"), url.Values{}),
	}
	for _, r := range reqs {
		desc := r.Method + " " + r.URL.Path
		if rec := send(e, bob, r, true); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s: %d, want 404", desc, rec.Code)
		}
	}
	if items, _ := f.svc.ListGallery(ctx, w.ID); len(items) != 1 {
		t.Error("foto alice berubah")
	}
	if got, _ := f.weddings.GetWedding(ctx, wb.ID); got.MainPhotoURL != nil {
		t.Error("bob berhasil memakai foto alice sebagai foto utama")
	}
	if u := f.usage(t, wb.ID); u != 0 {
		t.Errorf("pemakaian storage bob = %d", u)
	}
}
