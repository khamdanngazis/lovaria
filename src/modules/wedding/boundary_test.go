package wedding

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestStatusGuardsOnly: modul lain memakai guard (IsPublic, AllowsRSVP,
// AllowsGuestbook, InMemory, IsArchived, IsDraft) — bukan membandingkan status.
func TestStatusGuardsOnly(t *testing.T) {
	bad := regexp.MustCompile(`wedding\.Status(Draft|Published|WeddingDay|Memory|Archived)\b|\.Status\s*[!=]=\s*"(draft|published|wedding_day|memory|archived)"`)
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasSuffix(filepath.ToSlash(path), "modules/wedding") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, ".templ") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_templ.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if loc := bad.FindIndex(b); loc != nil {
			t.Errorf("%s: cek status wedding langsung, pakai guard: %q", path, b[loc[0]:loc[1]])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
