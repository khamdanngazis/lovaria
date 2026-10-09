package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

// TestWiring merakit aplikasi persis seperti `lovoria serve` (newApp + routes)
// lalu menjalankan alur utama end-to-end lewat HTTP.
func TestWiring(t *testing.T) {
	pool := dbtest.Pool(t)
	dir := t.TempDir()
	cfg, err := config.LoadFrom(func(k string) string {
		return map[string]string{"APP_ENV": config.EnvTest, "STORAGE_DRIVER": "local", "STORAGE_LOCAL_DIR": dir}[k]
	})
	if err != nil {
		t.Fatal(err)
	}
	a, err := newApp(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), pool)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(a.routes())
	defer srv.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	do := func(method, path, contentType string, body io.Reader, accept string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, body)
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	form := func(v url.Values) (string, io.Reader) {
		return "application/x-www-form-urlencoded", strings.NewReader(v.Encode())
	}

	if resp := do(http.MethodGet, "/healthz", "", nil, ""); resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %d", resp.StatusCode)
	}
	ct, body := form(url.Values{"name": {"W"}, "email": {"wiring@example.com"}, "password": {"password123"}, "password_confirmation": {"password123"}})
	if resp := do(http.MethodPost, "/register", ct, body, ""); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	ct, body = form(url.Values{"groom_name": {"A"}, "bride_name": {"B"}, "title": {"T"}, "wedding_date": {"2026-12-12"}})
	resp := do(http.MethodPost, "/dashboard/weddings", ct, body, "")
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusSeeOther || !strings.HasPrefix(loc.Path, "/dashboard/weddings/") {
		t.Fatalf("create wedding: %d %s", resp.StatusCode, loc)
	}
	// Wizard (T29) mendarat di halaman pratinjau pertama: …/<id>/start.
	if !strings.HasSuffix(loc.Path, "/start") {
		t.Fatalf("create wedding harus mendarat di /start: %s", loc)
	}
	wpath := strings.TrimSuffix(loc.Path, "/start")

	for _, p := range []string{"", "/start", "/theme/preview", "/info", "/couple", "/events", "/stories", "/gallery"} {
		if resp := do(http.MethodGet, wpath+p, "", nil, ""); resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s%s: %d", wpath, p, resp.StatusCode)
		}
	}

	// Upload foto → tersimpan di storage & tersaji lewat URL publiknya.
	var img bytes.Buffer
	_ = jpeg.Encode(&img, image.NewRGBA(image.Rect(0, 0, 64, 48)), nil)
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	_ = mw.WriteField("category", "wedding")
	fw, _ := mw.CreateFormFile("file", "a.jpg")
	_, _ = fw.Write(img.Bytes())
	_ = mw.Close()
	resp = do(http.MethodPost, wpath+"/gallery/items", mw.FormDataContentType(), &mp, "application/json")
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload: %d %s", resp.StatusCode, b)
	}
	var item struct{ URL string }
	_ = json.NewDecoder(resp.Body).Decode(&item)
	u, _ := url.Parse(item.URL)
	resp = do(http.MethodGet, u.Path, "", nil, "")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" {
		t.Errorf("GET %s: %d %s", u.Path, resp.StatusCode, resp.Header.Get("Content-Type"))
	}
}
