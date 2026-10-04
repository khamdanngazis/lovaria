package publicsite

import (
	"net/http"
	"strings"
	"testing"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
)

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
	// Harga tunggal (T23) selalu tampil; tanpa harga (0) bagian & menunya hilang.
	if !strings.Contains(body, `id="harga"`) || !strings.Contains(body, "Rp149.000") {
		t.Error("bagian harga harus tampil")
	}
	free := newFixture(t)
	free.handler.PublishPrice = 0
	if b := free.get("/", nil).Body.String(); strings.Contains(b, `id="harga"`) || strings.Contains(b, `href="#harga"`) || strings.Contains(b, "Berapa biayanya?") {
		t.Error("tanpa harga: bagian, menu, dan FAQ harga disembunyikan")
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

	body := f.get("/", nil).Body.String()
	if !strings.Contains(body, `href="/w/contoh-elegant"`) {
		t.Error("link contoh tema elegan")
	}
	if strings.Contains(body, ">Modern</h3>") || strings.Contains(body, "img/themes/modern.webp") {
		t.Error("tema nonaktif tidak boleh ditawarkan")
	}
	// Etalase (T21): thumbnail asli tiap tema; hanya tema bawaan berlabel.
	if !strings.Contains(body, `src="/static/img/themes/signature.webp`) || !strings.Contains(body, `src="/static/img/themes/elegant.webp`) || strings.Count(body, "Pilihan Lunovia") != 1 {
		t.Error("etalase tema: thumbnail / label Pilihan Lunovia")
	}
	// Harga tunggal (T23): sekali bayar saat publikasi, semua fitur termasuk.
	for _, want := range []string{`id="harga"`, "Rp149.000", "sekali bayar per undangan", "tanpa langganan", "Custom domain", "Berapa biayanya?", `href="#harga"`} {
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
		t.Errorf("robots domain Lunovia: %d %q", rec.Code, body)
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
