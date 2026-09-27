package publicsite

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestSingleResolver menegakkan Arsitektur §3 aturan 5: resolusi wedding dari
// Host header / slug / kode undangan hanya di resolver.go.
func TestSingleResolver(t *testing.T) {
	bad := regexp.MustCompile(`Request\(\)\.Host\b|\breq\.Host\b|\br\.Host\b|Param\("(slug|code)"\)|Header\.Get\("Host"\)`)
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_templ.go") {
			return nil
		}
		if filepath.ToSlash(path) == "../public-site/resolver.go" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if loc := bad.FindIndex(b); loc != nil {
			t.Errorf("%s: membaca Host/slug/kode di luar resolver: %q", path, b[loc[0]:loc[1]])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
