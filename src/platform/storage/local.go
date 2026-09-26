package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

// LocalRoute adalah path URL tempat driver local menyajikan objek.
const LocalRoute = "/media"

// Local menyimpan objek di disk. HANYA untuk development/test — config menolak
// driver ini di production (filesystem Railway tidak persisten).
type Local struct {
	dir     string
	baseURL string
}

func NewLocal(dir, baseURL string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("storage: buat folder %s: %w", dir, err)
	}
	return &Local{dir: dir, baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (l *Local) path(key string) (string, error) {
	if !validKey(key) {
		return "", fmt.Errorf("storage: key tidak valid %q", key)
	}
	return filepath.Join(l.dir, filepath.FromSlash(key)), nil
}

func (l *Local) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".upload-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}

func (l *Local) Delete(_ context.Context, key string) error {
	p, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (l *Local) PublicURL(key string) string { return l.baseURL + "/" + key }

// Register menyajikan objek di /media/* (hanya dipasang bila driver local).
func (l *Local) Register(e *echo.Echo) {
	fileServer := http.StripPrefix(LocalRoute+"/", http.FileServer(http.Dir(l.dir)))
	h := echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r) // tanpa directory listing
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		fileServer.ServeHTTP(w, r)
	}))
	e.GET(LocalRoute+"/*", h)
	e.HEAD(LocalRoute+"/*", h)
}
