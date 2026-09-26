// Package storage menyimpan objek (foto). Production memakai Cloudflare R2
// (S3-compatible); driver local hanya untuk development/test.
package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/khamdanngazis/lovaria/src/platform/config"
)

// Storage menyimpan dan menghapus objek, serta membentuk URL publiknya.
// Objek bersifat immutable (key selalu unik), sehingga URL publik bisa di-cache selamanya.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
	PublicURL(key string) string
}

// New memilih driver dari config. Driver local memakai URL relatif /media/*
// supaya benar di host/port mana pun saat development.
func New(ctx context.Context, cfg config.Storage) (Storage, error) {
	switch cfg.Driver {
	case "local":
		return NewLocal(cfg.LocalDir, LocalRoute)
	case "r2":
		endpoint := cfg.R2Endpoint
		if endpoint == "" {
			endpoint = "https://" + cfg.R2AccountID + ".r2.cloudflarestorage.com"
		}
		return NewS3(ctx, S3Config{
			Endpoint: endpoint, Bucket: cfg.R2Bucket, PublicURL: cfg.PublicURL,
			AccessKeyID: cfg.R2AccessKeyID, SecretAccessKey: cfg.R2SecretAccessKey,
		})
	default:
		return nil, fmt.Errorf("storage: driver tidak dikenal %q", cfg.Driver)
	}
}

// validKey memastikan key relatif dan tidak keluar folder (../).
func validKey(key string) bool {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
