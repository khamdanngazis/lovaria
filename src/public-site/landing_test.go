package publicsite

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/admin"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

type fakePackages struct{ list []admin.Package }

func (f *fakePackages) LandingPackages(context.Context) ([]admin.Package, error) { return f.list, nil }

func TestLandingPage(t *testing.T) {
	f := newFixture(t)
	body := f.get("/", nil).Body.String()
	for _, want := range []string{
		"Your Love.", "THE DIGITAL WEDDING EXPERIENCE", `href="/register"`, `href="/login"`,
		`id="cara-kerja"`, `id="fitur"`, `id="tema"`, `id="faq"`,
		`property="og:image" content="https://lovoria.test/static/img/brand/og-lovoria.png`,
		`rel="canonical" href="https://lovoria.test/"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("landing tidak memuat %q", want)
		}
	}
	// Tanpa paket bertanda tampil: bagian & menu harga disembunyikan.
	if strings.Contains(body, `id="harga"`) || strings.Contains(body, `href="#harga"`) {
		t.Error("bagian harga harus disembunyikan bila tidak ada paket")
	}
	// Belum ada undangan contoh: tidak ada link "Lihat contoh" yang mati.
	if strings.Contains(body, "/w/contoh-") {
		t.Error("link contoh hanya bila undangan demo ada")
	}
	// Undangan boleh diindeks? Landing ya (tanpa noindex); undangan tidak.
	if strings.Contains(body, `content="noindex"`) {
		t.Error("landing harus bisa diindeks")
	}
}

func TestLandingThemesPackagesAndLogin(t *testing.T) {
	f := newFixture(t)
	owner, w := f.newWedding(t, "demo@example.com", "Raka", "Nadia")
	if _, err := f.weddings.ChangeSlug(ctx, w.ID, DemoSlug("elegant")); err != nil {
		t.Fatal(err)
	}
	f.publish(w.ID)
	if err := f.themes.SetEnabled(ctx, "modern", false); err != nil {
		t.Fatal(err)
	}
	f.pkgs.list = []admin.Package{{ID: uuid.New(), Name: "Premium", StorageMB: 1024, ArchiveDays: 365, PriceDisplay: "Rp 149.000"}}

	body := f.get("/", nil).Body.String()
	if !strings.Contains(body, `href="/w/contoh-elegant"`) {
		t.Error("link contoh tema elegan")
	}
	if strings.Contains(body, ">Modern</h3>") {
		t.Error("tema nonaktif tidak boleh ditawarkan")
	}
	for _, want := range []string{`id="harga"`, "Rp 149.000", "Penyimpanan foto 1 GB", "aktif 1 tahun"} {
		if !strings.Contains(body, want) {
			t.Errorf("harga tidak memuat %q", want)
		}
	}
	// Undangan contoh yang ditarik ke draft tidak ditautkan.
	f.setStatus(w.ID, wedding.StatusDraft)
	if strings.Contains(f.get("/", nil).Body.String(), "/w/contoh-elegant") {
		t.Error("contoh draft tidak boleh ditautkan")
	}
	// Sudah login → tombol Dashboard.
	if !strings.Contains(f.get("/", map[string]string{"X-Test-User": owner.String()}).Body.String(), `href="/dashboard"`) {
		t.Error("user login melihat tombol Dashboard")
	}
}

func TestRobotsAndSitemap(t *testing.T) {
	f := newFixture(t)
	rec := f.get("/robots.txt", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Disallow: /i/") || !strings.Contains(body, "Disallow: /dashboard") ||
		!strings.Contains(body, "Sitemap: https://lovoria.test/sitemap.xml") {
		t.Errorf("robots domain Lovoria: %d %q", rec.Code, body)
	}
	rec = f.get("/sitemap.xml", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<loc>https://lovoria.test/privacy</loc>") || strings.Contains(rec.Body.String(), "/w/") {
		t.Errorf("sitemap: %d %s", rec.Code, rec.Body.String())
	}
	// Custom domain pasangan: tidak diindeks sama sekali, tanpa sitemap.
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	f.publish(w.ID)
	f.domains["www.samuelsarah.com"] = w.ID
	custom := map[string]string{"Host": "www.samuelsarah.com"}
	if b := f.get("/robots.txt", custom).Body.String(); b != "User-agent: *\nDisallow: /\n" {
		t.Errorf("robots custom domain: %q", b)
	}
	if rec := f.get("/sitemap.xml", custom); rec.Code != http.StatusNotFound {
		t.Errorf("sitemap custom domain: %d", rec.Code)
	}
}
