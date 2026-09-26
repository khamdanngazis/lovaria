package gallery

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/storage"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

type fixture struct {
	svc      *Service
	weddings *wedding.Service
	auth     *auth.Service
	dir      string
}

func newFixture(t *testing.T, store storage.Storage, quota int64) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	dir := t.TempDir()
	if store == nil {
		local, err := storage.NewLocal(dir, "http://localhost/media")
		if err != nil {
			t.Fatal(err)
		}
		store = local
	}
	ws := wedding.NewService(wedding.NewRepository(pool))
	return fixture{
		svc:      NewService(NewRepository(pool), store, ws, quota, log),
		weddings: ws,
		auth:     auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
		dir:      dir,
	}
}

func (f fixture) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func (f fixture) usage(t *testing.T, weddingID uuid.UUID) int64 {
	t.Helper()
	u, err := f.svc.StorageUsage(ctx, weddingID)
	if err != nil {
		t.Fatal(err)
	}
	return u.UsedBytes
}

func photo(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, nil)
	return b.Bytes()
}

func fileCount(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			n++
		}
		return nil
	})
	return n
}

func TestUploadAndDelete(t *testing.T) {
	f := newFixture(t, nil, 500<<20)
	_, w := f.newWedding(t, "a@example.com")

	it, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(photo(3000, 2000)))
	if err != nil {
		t.Fatal(err)
	}
	if it.Width != 2048 || it.Height != 1365 || it.SizeBytes <= 0 {
		t.Errorf("item = %+v", it)
	}
	wantPrefix := "http://localhost/media/weddings/" + w.ID.String() + "/wedding/" + it.ID.String()
	if it.URL != wantPrefix+".jpg" || it.ThumbURL != wantPrefix+"_thumb.jpg" {
		t.Errorf("url = %s / %s", it.URL, it.ThumbURL)
	}
	if fileCount(t, f.dir) != 2 {
		t.Errorf("objek tersimpan = %d, want 2 (foto + thumbnail)", fileCount(t, f.dir))
	}
	if got := f.usage(t, w.ID); got != it.SizeBytes {
		t.Errorf("storage_used_bytes = %d, want %d", got, it.SizeBytes)
	}

	if err := f.svc.DeleteItem(ctx, w.ID, it.ID); err != nil {
		t.Fatal(err)
	}
	if fileCount(t, f.dir) != 0 {
		t.Error("objek harus terhapus")
	}
	if got := f.usage(t, w.ID); got != 0 {
		t.Errorf("storage_used_bytes setelah hapus = %d", got)
	}
	if err := f.svc.DeleteItem(ctx, w.ID, it.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("hapus 2x: %v", err)
	}
}

func TestUploadRejectsNonImage(t *testing.T) {
	f := newFixture(t, nil, 500<<20)
	_, w := f.newWedding(t, "a@example.com")
	for name, data := range map[string][]byte{
		"teks.jpg": []byte("bukan gambar, cuma di-rename .jpg"),
		"heic":     append([]byte{0, 0, 0, 0x18}, []byte("ftypheic0000mif1")...),
	} {
		_, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(data))
		var ue *UploadError
		if !errors.As(err, &ue) {
			t.Errorf("%s: err = %v, want UploadError", name, err)
		}
	}
	if _, err := f.svc.Upload(ctx, w.ID, "selfie", bytes.NewReader(photo(10, 10))); err == nil {
		t.Error("kategori tidak dikenal harus ditolak")
	}
	if fileCount(t, f.dir) != 0 || f.usage(t, w.ID) != 0 {
		t.Error("upload gagal tidak boleh menyimpan apa pun")
	}
}

func TestQuota(t *testing.T) {
	f := newFixture(t, nil, 1) // placeholder, diganti di bawah
	_, w := f.newWedding(t, "a@example.com")

	// Ukur satu upload dengan kuota longgar, lalu set kuota = 1 upload + sedikit.
	f.svc.quota = 500 << 20
	first, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(photo(800, 600)))
	if err != nil {
		t.Fatal(err)
	}
	f.svc.quota = first.SizeBytes + 100

	if _, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(photo(800, 600))); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("err = %v, want ErrQuotaExceeded", err)
	}
	if fileCount(t, f.dir) != 2 || f.usage(t, w.ID) != first.SizeBytes {
		t.Errorf("upload yang ditolak kuota tidak boleh menyimpan objek/menambah pemakaian")
	}
	// Setelah foto dihapus, upload bisa lagi.
	if err := f.svc.DeleteItem(ctx, w.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(photo(800, 600))); err != nil {
		t.Errorf("setelah hapus: %v", err)
	}
}

// failingStore gagal pada Put ke-n untuk menguji kompensasi.
type failingStore struct {
	storage.Storage
	failOn, puts int
	deleted      []string
}

func (s *failingStore) Put(ctx context.Context, key string, r io.Reader, size int64, ct string) error {
	s.puts++
	if s.puts == s.failOn {
		return errors.New("R2 down")
	}
	return s.Storage.Put(ctx, key, r, size, ct)
}

func (s *failingStore) Delete(ctx context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.Storage.Delete(ctx, key)
}

func TestUploadRollbackOnStorageFailure(t *testing.T) {
	dir := t.TempDir()
	local, _ := storage.NewLocal(dir, "http://x/media")
	fs := &failingStore{Storage: local, failOn: 2} // thumbnail gagal
	f := newFixture(t, fs, 500<<20)
	_, w := f.newWedding(t, "a@example.com")

	if _, err := f.svc.Upload(ctx, w.ID, CategoryWedding, bytes.NewReader(photo(100, 100))); err == nil {
		t.Fatal("harus error")
	}
	if len(fs.deleted) != 1 || fileCount(t, dir) != 0 {
		t.Errorf("foto utama harus dihapus kembali: deleted=%v files=%d", fs.deleted, fileCount(t, dir))
	}
	if f.usage(t, w.ID) != 0 {
		t.Error("kuota harus dikembalikan")
	}
	if items, _ := f.svc.ListGallery(ctx, w.ID); len(items) != 0 {
		t.Error("tidak boleh ada item")
	}
}

// Hapus item juga menghapus objek di bucket S3/R2.
func TestDeleteRemovesObjectsFromS3(t *testing.T) {
	backend := s3mem.New()
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	defer srv.Close()
	_ = backend.CreateBucket("lovoria")
	s3, err := storage.NewS3(ctx, storage.S3Config{Endpoint: srv.URL, Bucket: "lovoria", PublicURL: "https://media.test", AccessKeyID: "k", SecretAccessKey: "s", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, s3, 500<<20)
	_, w := f.newWedding(t, "a@example.com")

	objects := func() int {
		res, err := backend.ListBucket("lovoria", nil, gofakes3.ListBucketPage{})
		if err != nil {
			t.Fatal(err)
		}
		return len(res.Contents)
	}
	it, err := f.svc.Upload(ctx, w.ID, CategoryPrewedding, bytes.NewReader(photo(640, 480)))
	if err != nil {
		t.Fatal(err)
	}
	if objects() != 2 || !strings.HasPrefix(it.URL, "https://media.test/weddings/") {
		t.Fatalf("objects=%d url=%s", objects(), it.URL)
	}
	if err := f.svc.DeleteItem(ctx, w.ID, it.ID); err != nil {
		t.Fatal(err)
	}
	if objects() != 0 {
		t.Errorf("objek R2 tersisa: %d", objects())
	}
}

func TestOrderingCoverAndIsolation(t *testing.T) {
	f := newFixture(t, nil, 500<<20)
	_, wa := f.newWedding(t, "a@example.com")
	_, wb := f.newWedding(t, "b@example.com")
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		it, err := f.svc.Upload(ctx, wa.ID, CategoryWedding, bytes.NewReader(photo(50+i, 50)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, it.ID)
	}
	if err := f.svc.MoveItem(ctx, wa.ID, ids[2], true); err != nil {
		t.Fatal(err)
	}
	items, _ := f.svc.ListGallery(ctx, wa.ID)
	if items[1].ID != ids[2] || items[2].ID != ids[1] {
		t.Errorf("urutan setelah move salah")
	}

	up, err := f.svc.UpdateItem(ctx, wa.ID, ids[0], "  Momen akad ", CategoryCover)
	if err != nil || up.Caption != "Momen akad" || up.Category != CategoryCover {
		t.Errorf("update: %+v %v", up, err)
	}
	if err := f.svc.SetCover(ctx, wa.ID, ids[0]); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.weddings.GetWedding(ctx, wa.ID); got.MainPhotoURL == nil || *got.MainPhotoURL != up.URL {
		t.Errorf("main photo = %v", got.MainPhotoURL)
	}

	// Wedding B tidak bisa menyentuh foto A.
	if _, err := f.svc.GetItem(ctx, wb.ID, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := f.svc.UpdateItem(ctx, wb.ID, ids[0], "x", CategoryWedding); !errors.Is(err, ErrNotFound) {
		t.Errorf("update: %v", err)
	}
	if err := f.svc.DeleteItem(ctx, wb.ID, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete: %v", err)
	}
	if err := f.svc.MoveItem(ctx, wb.ID, ids[0], false); !errors.Is(err, ErrNotFound) {
		t.Errorf("move: %v", err)
	}
	if err := f.svc.SetCover(ctx, wb.ID, ids[0]); !errors.Is(err, ErrNotFound) {
		t.Errorf("cover: %v", err)
	}
	if items, _ := f.svc.ListGallery(ctx, wb.ID); len(items) != 0 {
		t.Error("wedding B melihat foto A")
	}
	if f.usage(t, wb.ID) != 0 {
		t.Error("pemakaian storage B harus 0")
	}
}
