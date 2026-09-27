// Package gallery: foto wedding (upload oleh couple/admin) yang disimpan di R2.
// Semua operasi menerima weddingID yang sudah diotorisasi (RequireWeddingOwner).
package gallery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/imageproc"
	"github.com/khamdanngazis/lovaria/src/platform/order"
	"github.com/khamdanngazis/lovaria/src/platform/storage"

	gallerydb "github.com/khamdanngazis/lovaria/src/modules/gallery/db"
)

const (
	CategoryCover      = "cover"
	CategoryCouple     = "couple"
	CategoryPrewedding = "prewedding"
	CategoryWedding    = "wedding"

	maxCaptionLen = 300
)

// Categories berisi kategori foto beserta labelnya.
var Categories = []struct{ ID, Label string }{
	{CategoryWedding, "Pernikahan"},
	{CategoryPrewedding, "Prewedding"},
	{CategoryCouple, "Pasangan"},
	{CategoryCover, "Sampul"},
}

func CategoryLabel(c string) string {
	for _, x := range Categories {
		if x.ID == c {
			return x.Label
		}
	}
	return ""
}

var (
	ErrNotFound = errors.New("foto tidak ditemukan")
	// ErrQuotaExceeded: kuota penyimpanan wedding penuh.
	ErrQuotaExceeded = wedding.ErrQuotaExceeded
)

// UploadError adalah error upload yang pesannya aman ditampilkan ke user.
type UploadError struct{ Msg string }

func (e *UploadError) Error() string { return e.Msg }

// Item adalah satu foto gallery.
type Item struct {
	ID        uuid.UUID
	WeddingID uuid.UUID
	Category  string
	URL       string
	ThumbURL  string
	Width     int
	Height    int
	SizeBytes int64
	SortOrder int
	Caption   string
}

// Usage adalah pemakaian storage wedding.
type Usage struct {
	UsedBytes  int64
	QuotaBytes int64
}

// WeddingStorage adalah bagian service wedding yang dibutuhkan gallery
// (kolom weddings.storage_used_bytes & main_photo_url milik modul wedding).
type WeddingStorage interface {
	ReserveStorage(ctx context.Context, weddingID uuid.UUID, bytes, quota int64) error
	ReleaseStorage(ctx context.Context, weddingID uuid.UUID, bytes int64) error
	StorageUsage(ctx context.Context, weddingID uuid.UUID) (int64, error)
	SetMainPhotoURL(ctx context.Context, weddingID uuid.UUID, url string) error
}

type Service struct {
	repo     *Repository
	store    storage.Storage
	weddings WeddingStorage
	quota    int64
	quotaSrc func(ctx context.Context, weddingID uuid.UUID) (int64, bool, error)
	log      *slog.Logger
}

func NewService(repo *Repository, store storage.Storage, weddings WeddingStorage, quotaBytes int64, log *slog.Logger) *Service {
	return &Service{repo: repo, store: store, weddings: weddings, quota: quotaBytes, log: log}
}

func toItem(r gallerydb.GalleryItem) Item {
	return Item{
		ID: r.ID, WeddingID: r.WeddingID, Category: r.Category, URL: r.Url, ThumbURL: r.ThumbUrl,
		Width: int(r.Width), Height: int(r.Height), SizeBytes: r.SizeBytes,
		SortOrder: int(r.SortOrder), Caption: r.Caption,
	}
}

// ---------- Upload ----------

// Upload memvalidasi & memproses satu gambar lalu menyimpannya ke storage:
// weddings/{wedding_id}/{category}/{uuid}.jpg (+ _thumb.jpg).
func (s *Service) Upload(ctx context.Context, weddingID uuid.UUID, category string, r io.Reader) (Item, error) {
	if CategoryLabel(category) == "" {
		return Item{}, &UploadError{Msg: "Kategori foto tidak dikenal"}
	}
	res, err := imageproc.Process(r)
	if err != nil {
		var known = []error{imageproc.ErrTooLarge, imageproc.ErrNotImage, imageproc.ErrHEIC, imageproc.ErrTooManyPx, imageproc.ErrCorrupt}
		for _, k := range known {
			if errors.Is(err, k) {
				return Item{}, &UploadError{Msg: k.Error()}
			}
		}
		return Item{}, fmt.Errorf("gallery: proses gambar: %w", err)
	}

	size := int64(len(res.Main) + len(res.Thumb))
	quota, err := s.quotaFor(ctx, weddingID)
	if err != nil {
		return Item{}, err
	}
	if err := s.weddings.ReserveStorage(ctx, weddingID, size, quota); err != nil {
		if errors.Is(err, wedding.ErrQuotaExceeded) {
			return Item{}, ErrQuotaExceeded
		}
		return Item{}, fmt.Errorf("gallery: reserve storage: %w", err)
	}

	id := db.NewID()
	prefix := fmt.Sprintf("weddings/%s/%s/%s", weddingID, category, id)
	mainKey, thumbKey := prefix+".jpg", prefix+"_thumb.jpg"

	// Kompensasi bila langkah berikutnya gagal: hapus objek & kembalikan kuota.
	var stored []string
	rollback := func(cause error) error {
		ctx := context.WithoutCancel(ctx)
		for _, k := range stored {
			if err := s.store.Delete(ctx, k); err != nil {
				s.log.WarnContext(ctx, "gallery: rollback delete gagal", slog.String("key", k), slog.String("error", err.Error()))
			}
		}
		if err := s.weddings.ReleaseStorage(ctx, weddingID, size); err != nil {
			s.log.WarnContext(ctx, "gallery: rollback release gagal", slog.String("error", err.Error()))
		}
		return cause
	}

	for _, obj := range []struct {
		key  string
		data []byte
	}{{mainKey, res.Main}, {thumbKey, res.Thumb}} {
		if err := s.store.Put(ctx, obj.key, bytes.NewReader(obj.data), int64(len(obj.data)), imageproc.ContentType); err != nil {
			return Item{}, rollback(fmt.Errorf("gallery: simpan objek: %w", err))
		}
		stored = append(stored, obj.key)
	}

	next, err := s.repo.q.NextSortOrder(ctx, weddingID)
	if err != nil {
		return Item{}, rollback(err)
	}
	row, err := s.repo.q.CreateItem(ctx, gallerydb.CreateItemParams{
		ID: id, WeddingID: weddingID, Category: category,
		ObjectKey: mainKey, ThumbKey: thumbKey,
		Url: s.store.PublicURL(mainKey), ThumbUrl: s.store.PublicURL(thumbKey),
		Width: int32(res.Width), Height: int32(res.Height), //nolint:gosec // G115: maks. 2048
		SizeBytes: size, SortOrder: next,
	})
	if err != nil {
		return Item{}, rollback(fmt.Errorf("gallery: simpan item: %w", err))
	}
	return toItem(row), nil
}

// ---------- Ubah / hapus / urutan ----------

// UpdateItem mengubah caption & kategori (objek di storage tidak dipindah).
func (s *Service) UpdateItem(ctx context.Context, weddingID, id uuid.UUID, caption, category string) (Item, error) {
	caption = strings.TrimSpace(caption)
	if CategoryLabel(category) == "" {
		return Item{}, &UploadError{Msg: "Kategori foto tidak dikenal"}
	}
	if utf8.RuneCountInString(caption) > maxCaptionLen {
		return Item{}, &UploadError{Msg: fmt.Sprintf("Keterangan maksimal %d karakter", maxCaptionLen)}
	}
	row, err := s.repo.q.UpdateItem(ctx, gallerydb.UpdateItemParams{ID: id, WeddingID: weddingID, Caption: caption, Category: category})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return toItem(row), nil
}

// DeleteItem menghapus item, objek foto & thumbnail di storage, dan mengurangi
// pemakaian storage wedding.
func (s *Service) DeleteItem(ctx context.Context, weddingID, id uuid.UUID) error {
	row, err := s.repo.q.DeleteItem(ctx, gallerydb.DeleteItemParams{ID: id, WeddingID: weddingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	for _, k := range []string{row.ObjectKey, row.ThumbKey} {
		if err := s.store.Delete(ctx, k); err != nil {
			// Baris sudah terhapus; objek yatim hanya memakan ruang di bucket.
			s.log.ErrorContext(ctx, "gallery: hapus objek gagal", slog.String("key", k), slog.String("error", err.Error()))
		}
	}
	return s.weddings.ReleaseStorage(ctx, weddingID, row.SizeBytes)
}

// MoveItem menggeser foto satu posisi ke depan (up) atau belakang.
func (s *Service) MoveItem(ctx context.Context, weddingID, id uuid.UUID, up bool) error {
	return s.repo.inTx(ctx, func(q *gallerydb.Queries) error {
		rows, err := q.ListItemsForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(rows))
		found := false
		for i, r := range rows {
			ids[i] = r.ID
			found = found || r.ID == id
		}
		if !found {
			return ErrNotFound
		}
		next, changed := order.Move(ids, id, up)
		if !changed {
			return nil
		}
		for i, itemID := range next {
			if err := q.SetItemSortOrder(ctx, gallerydb.SetItemSortOrderParams{ID: itemID, WeddingID: weddingID, SortOrder: int32(i)}); err != nil { //nolint:gosec // G115: jumlah foto kecil
				return err
			}
		}
		return nil
	})
}

// SetCover menjadikan foto sebagai foto utama wedding.
func (s *Service) SetCover(ctx context.Context, weddingID, id uuid.UUID) error {
	it, err := s.GetItem(ctx, weddingID, id)
	if err != nil {
		return err
	}
	return s.weddings.SetMainPhotoURL(ctx, weddingID, it.URL)
}

// ---------- Baca ----------

func (s *Service) GetItem(ctx context.Context, weddingID, id uuid.UUID) (Item, error) {
	row, err := s.repo.q.GetItem(ctx, gallerydb.GetItemParams{ID: id, WeddingID: weddingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	return toItem(row), nil
}

// ListGallery mengembalikan semua foto wedding sesuai urutan tampil (T09).
func (s *Service) ListGallery(ctx context.Context, weddingID uuid.UUID) ([]Item, error) {
	rows, err := s.repo.q.ListItems(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	out := make([]Item, len(rows))
	for i, r := range rows {
		out[i] = toItem(r)
	}
	return out, nil
}

// StorageUsage mengembalikan pemakaian & kuota storage wedding (T16).
func (s *Service) StorageUsage(ctx context.Context, weddingID uuid.UUID) (Usage, error) {
	used, err := s.weddings.StorageUsage(ctx, weddingID)
	if err != nil {
		return Usage{}, err
	}
	quota, err := s.quotaFor(ctx, weddingID)
	if err != nil {
		return Usage{}, err
	}
	return Usage{UsedBytes: used, QuotaBytes: quota}, nil
}

// RebaseMediaURLs mengganti basis URL foto & thumbnail gallery (objek di storage
// tidak berubah; hanya URL publik tersimpan). apply=false hanya menghitung.
func (s *Service) RebaseMediaURLs(ctx context.Context, oldPrefix, newPrefix string, apply bool) (int64, error) {
	if !apply {
		return s.repo.q.CountItemURLPrefix(ctx, oldPrefix)
	}
	return s.repo.q.RebaseItemURLs(ctx, gallerydb.RebaseItemURLsParams{OldPrefix: oldPrefix, NewPrefix: newPrefix})
}

// Summary: ringkasan galeri untuk beranda dashboard (T13).
type Summary struct {
	Count  int
	Thumbs []string // thumbnail n foto pertama sesuai urutan tampil
	Usage  Usage
}

func (s *Service) Summary(ctx context.Context, weddingID uuid.UUID, n int) (Summary, error) {
	items, err := s.ListGallery(ctx, weddingID)
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{Count: len(items)}
	for _, it := range items {
		if len(sum.Thumbs) == n {
			break
		}
		sum.Thumbs = append(sum.Thumbs, it.ThumbURL)
	}
	sum.Usage, err = s.StorageUsage(ctx, weddingID)
	return sum, err
}

// SetQuotaSource memasang kuota storage per wedding (paket, T16). ok=false →
// kuota default dari config (STORAGE_QUOTA_MB).
func (s *Service) SetQuotaSource(f func(ctx context.Context, weddingID uuid.UUID) (bytes int64, ok bool, err error)) {
	s.quotaSrc = f
}

func (s *Service) quotaFor(ctx context.Context, weddingID uuid.UUID) (int64, error) {
	if s.quotaSrc == nil {
		return s.quota, nil
	}
	q, ok, err := s.quotaSrc(ctx, weddingID)
	if err != nil || !ok {
		return s.quota, err
	}
	return q, nil
}
