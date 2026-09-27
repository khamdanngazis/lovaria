package gallery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db"
)

// MaxAudioBytes: batas ukuran unggahan musik latar (T20).
const MaxAudioBytes = 8 << 20

// Audio adalah berkas musik unggahan wedding di storage.
type Audio struct {
	Key       string
	URL       string
	SizeBytes int64
}

// audioPrefix: semua unggahan musik wedding ada di bawah prefix ini (isolasi tenant).
func audioPrefix(weddingID uuid.UUID) string { return fmt.Sprintf("weddings/%s/music/", weddingID) }

// isMP3 mengenali MP3 dari magic bytes: tag ID3v2 atau frame sync MPEG audio
// (11 bit pertama menyala, layer ≠ reserved).
func isMP3(b []byte) bool {
	if len(b) >= 3 && string(b[:3]) == "ID3" {
		return true
	}
	return len(b) >= 2 && b[0] == 0xFF && b[1]&0xE0 == 0xE0 && b[1]&0x06 != 0
}

// UploadAudio memvalidasi (MP3, ≤ 8 MB), memesan kuota storage paket yang sama
// dengan foto, lalu menyimpan weddings/{id}/music/{uuid}.mp3.
func (s *Service) UploadAudio(ctx context.Context, weddingID uuid.UUID, r io.Reader) (Audio, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxAudioBytes+1))
	if err != nil {
		return Audio{}, fmt.Errorf("gallery: baca audio: %w", err)
	}
	switch {
	case len(data) > MaxAudioBytes:
		return Audio{}, &UploadError{Msg: "Ukuran musik maksimal 8 MB"}
	case !isMP3(data):
		return Audio{}, &UploadError{Msg: "Berkas harus MP3"}
	}
	size := int64(len(data))
	quota, err := s.quotaFor(ctx, weddingID)
	if err != nil {
		return Audio{}, err
	}
	if err := s.weddings.ReserveStorage(ctx, weddingID, size, quota); err != nil {
		if errors.Is(err, wedding.ErrQuotaExceeded) {
			return Audio{}, ErrQuotaExceeded
		}
		return Audio{}, fmt.Errorf("gallery: reserve storage: %w", err)
	}
	key := audioPrefix(weddingID) + db.NewID().String() + ".mp3"
	if err := s.store.Put(ctx, key, bytes.NewReader(data), size, "audio/mpeg"); err != nil {
		if rerr := s.weddings.ReleaseStorage(context.WithoutCancel(ctx), weddingID, size); rerr != nil {
			s.log.WarnContext(ctx, "gallery: rollback release audio gagal", slog.String("error", rerr.Error()))
		}
		return Audio{}, fmt.Errorf("gallery: simpan audio: %w", err)
	}
	return Audio{Key: key, URL: s.store.PublicURL(key), SizeBytes: size}, nil
}

// DeleteAudio menghapus unggahan musik wedding dan mengembalikan kuotanya. Key
// di luar prefix wedding ditolak (tidak bisa menghapus berkas wedding lain).
func (s *Service) DeleteAudio(ctx context.Context, weddingID uuid.UUID, key string, size int64) error {
	if !strings.HasPrefix(key, audioPrefix(weddingID)) {
		return fmt.Errorf("gallery: key audio %q bukan milik wedding %s", key, weddingID)
	}
	if err := s.store.Delete(ctx, key); err != nil {
		return fmt.Errorf("gallery: hapus audio: %w", err)
	}
	return s.weddings.ReleaseStorage(ctx, weddingID, size)
}

// PublicURL: URL publik objek storage (mis. lagu bawaan "music/…").
func (s *Service) PublicURL(key string) string { return s.store.PublicURL(key) }
