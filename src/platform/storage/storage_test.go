package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/labstack/echo/v4"
)

var ctx = context.Background()

func TestValidKey(t *testing.T) {
	for _, k := range []string{"weddings/a/b.jpg", "x.jpg"} {
		if !validKey(k) {
			t.Errorf("%q harus valid", k)
		}
	}
	for _, k := range []string{"", "/etc/passwd", "../x", "a/../../x", "a//b", "a/./b", `a\b`} {
		if validKey(k) {
			t.Errorf("%q harus invalid", k)
		}
	}
}

func TestLocal(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLocal(dir, "http://localhost:8080/media")
	if err != nil {
		t.Fatal(err)
	}
	key := "weddings/w1/wedding/abc.jpg"
	if err := l.Put(ctx, key, strings.NewReader("data"), 4, "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "weddings", "w1", "wedding", "abc.jpg")); string(b) != "data" {
		t.Errorf("isi file = %q", b)
	}
	if got := l.PublicURL(key); got != "http://localhost:8080/media/"+key {
		t.Errorf("PublicURL = %s", got)
	}
	if err := l.Put(ctx, "../escape.jpg", strings.NewReader("x"), 1, "image/jpeg"); err == nil {
		t.Error("path traversal harus ditolak")
	}

	e := echo.New()
	l.Register(e)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+key, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "data" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("serve: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/weddings/", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("directory listing: %d", rec.Code)
	}

	if err := l.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := l.Delete(ctx, key); err != nil {
		t.Errorf("hapus objek yang sudah tidak ada harus nil: %v", err)
	}
}

// TestS3 menguji driver S3/R2 terhadap server S3 palsu (HTTP sungguhan).
func TestS3(t *testing.T) {
	backend := s3mem.New()
	fake := gofakes3.New(backend).Server()
	var putCacheControl string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCacheControl = r.Header.Get("Cache-Control")
		}
		fake.ServeHTTP(w, r)
	}))
	defer srv.Close()
	if err := backend.CreateBucket("lovoria"); err != nil {
		t.Fatal(err)
	}

	st, err := NewS3(ctx, S3Config{
		Endpoint: srv.URL, Bucket: "lovoria", PublicURL: "https://media.lovoria.test/",
		AccessKeyID: "k", SecretAccessKey: "s", PathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	key := "weddings/w1/cover/abc.jpg"
	body := bytes.Repeat([]byte{0xFF}, 1024)
	if err := st.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "image/jpeg"); err != nil {
		t.Fatal(err)
	}

	obj, err := st.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String("lovoria"), Key: aws.String(key)})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(obj.Body)
	_ = obj.Body.Close()
	if !bytes.Equal(got, body) || aws.ToString(obj.ContentType) != "image/jpeg" {
		t.Errorf("objek: len=%d type=%s", len(got), aws.ToString(obj.ContentType))
	}
	// gofakes3 tidak menyimpan Cache-Control; cukup pastikan header dikirim ke S3.
	if putCacheControl != "public, max-age=31536000, immutable" {
		t.Errorf("Cache-Control yang dikirim = %q", putCacheControl)
	}
	if u := st.PublicURL(key); u != "https://media.lovoria.test/"+key {
		t.Errorf("PublicURL = %s", u)
	}

	if err := st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := st.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String("lovoria"), Key: aws.String(key)}); err == nil {
		t.Error("objek masih ada setelah Delete")
	}
	if err := st.Put(ctx, "../x", bytes.NewReader(nil), 0, "image/jpeg"); err == nil {
		t.Error("key tidak valid harus ditolak")
	}
}
