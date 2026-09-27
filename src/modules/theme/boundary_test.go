package theme

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoThemeLogicOutsideModule menegakkan aturan wajib #3: keputusan berbasis
// ID tema hanya boleh ada di modul theme (registry), bukan tersebar di kode lain.
func TestNoThemeLogicOutsideModule(t *testing.T) {
	root := filepath.Join("..", "..") // src/
	bad := regexp.MustCompile(`(?i)(switch\s+[\w.]*themeid\b|themeid\s*==|==\s*"(elegant|minimal|romantic|modern)")`)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasSuffix(filepath.ToSlash(path), "modules/theme") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".templ") {
			return nil
		}
		if strings.HasSuffix(path, "_templ.go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if loc := bad.FindIndex(b); loc != nil {
			t.Errorf("%s: logic tema di luar modul theme: %q", path, b[loc[0]:loc[1]])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
