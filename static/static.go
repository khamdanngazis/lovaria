// Package static menyajikan aset statis (CSS hasil Tailwind, htmx, Alpine).
//
// Production: aset di-embed ke binary dan disajikan dengan URL ber-hash konten
// (?v=<hash>) + Cache-Control immutable. Development: dibaca langsung dari disk
// tanpa cache supaya hasil `tailwind --watch` langsung terlihat.
package static

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
)

// Prefix adalah path URL tempat aset disajikan.
const Prefix = "/static"

//go:embed all:css js img
var embedded embed.FS

var (
	mu      sync.RWMutex
	current fs.FS = embedded
	fromDsk bool
	hashes  = map[string]string{}
)

// Configure memilih sumber aset. dir hanya dipakai bila fromDisk true.
func Configure(fromDisk bool, dir string) {
	mu.Lock()
	defer mu.Unlock()
	fromDsk = fromDisk
	hashes = map[string]string{}
	if fromDisk {
		current = os.DirFS(dir)
	} else {
		current = embedded
	}
}

// URL mengembalikan URL publik untuk aset, mis. URL("css/app.css") →
// "/static/css/app.css?v=1a2b3c4d". Di mode disk tidak ada hash.
func URL(name string) string {
	name = strings.TrimPrefix(name, "/")
	u := Prefix + "/" + name

	mu.RLock()
	h, ok := hashes[name]
	disk := fromDsk
	fsys := current
	mu.RUnlock()
	if disk {
		return u
	}
	if !ok {
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return u
		}
		sum := sha256.Sum256(b)
		h = hex.EncodeToString(sum[:4])
		mu.Lock()
		hashes[name] = h
		mu.Unlock()
	}
	return u + "?v=" + h
}

// Exists: apakah aset ada (mis. thumbnail tema yang dibuat `make theme-thumbs`).
func Exists(name string) bool {
	mu.RLock()
	fsys := current
	mu.RUnlock()
	_, err := fs.Stat(fsys, strings.TrimPrefix(name, "/"))
	return err == nil
}

// Register memasang handler aset statis di Prefix.
func Register(e *echo.Echo) {
	fileServer := http.StripPrefix(Prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		fsys, disk := current, fromDsk
		mu.RUnlock()

		name := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if st, err := fs.Stat(fsys, name); err != nil || st.IsDir() {
			http.NotFound(w, r)
			return
		}
		switch {
		case disk:
			w.Header().Set("Cache-Control", "no-cache")
		case r.URL.Query().Get("v") != "":
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		http.FileServerFS(fsys).ServeHTTP(w, r)
	}))
	h := echo.WrapHandler(fileServer)
	e.GET(Prefix+"/*", h)
	e.HEAD(Prefix+"/*", h)
}
