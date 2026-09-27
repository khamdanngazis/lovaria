package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
)

func TestSeedDemo(t *testing.T) {
	a, _ := hardeningApp(t)
	ctx := context.Background()
	var out strings.Builder
	if err := a.seedDemo(ctx, &out); err != nil {
		t.Fatal(err)
	}
	if err := a.seedDemo(ctx, io.Discard); err != nil { // idempoten
		t.Fatalf("seed ulang: %v", err)
	}
	for _, d := range theme.All() {
		w, err := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug(d.ID))
		if err != nil || !w.IsPublic() || w.ThemeID != d.ID {
			t.Errorf("demo %s: %+v %v", d.ID, w, err)
		}
	}
	if strings.Count(out.String(), "dibuat:") != len(theme.All()) {
		t.Errorf("output seed: %s", out.String())
	}
	// Tidak dihitung laporan admin.
	if p, _ := a.weddings.AdminList(ctx, wedding.AdminFilter{}, 25); p.Total != 0 {
		t.Errorf("demo muncul di admin: %d", p.Total)
	}
	if by, _ := a.weddings.CountByStatus(ctx); len(by) != 0 {
		t.Errorf("demo terhitung status: %v", by)
	}
	// Pemilik demo tidak bisa login (akun dinonaktifkan).
	w, _ := a.weddings.GetWeddingBySlug(ctx, publicsite.DemoSlug("elegant"))
	if u, err := a.auth.GetUser(ctx, w.OwnerUserID); err != nil || u.DisabledAt == nil {
		t.Errorf("pemilik demo harus nonaktif: %+v %v", u, err)
	}
	// Scheduler lifecycle tidak menyentuh demo walau tanggalnya sudah lewat.
	if _, err := a.pool.Exec(ctx, "UPDATE weddings SET wedding_date = now() - interval '30 days' WHERE is_demo"); err != nil {
		t.Fatal(err)
	}
	if n, err := a.weddings.AdvanceDue(ctx, 365); err != nil || n != 0 {
		t.Errorf("lifecycle memproses demo: %d %v", n, err)
	}
}
