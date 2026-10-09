package publicsite

import (
	"encoding/json"
	"github.com/khamdanngazis/lovaria/src/platform/web"
	"net/http"
	"regexp"
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

// T26: Kebijakan Privasi & Syarat & Ketentuan adalah teks final — tanpa label
// draf, bertanggal berlaku, memuat bagian wajib, dan menautkan kontak bantuan.
func TestLegalPagesFinal(t *testing.T) {
	f := newFixture(t)
	t.Cleanup(func() { web.SetSupport(web.Support{}) })
	web.SetSupport(web.Support{})
	pages := map[string][]string{
		"/privacy": {"Kebijakan Privasi", "Data yang kami kumpulkan", "Data tamu", "Midtrans", "Google Fonts", "Berapa lama data disimpan", "Hak Anda", `href="/terms"`},
		"/terms":   {"Syarat &amp; Ketentuan", "Harga dan pembayaran", "satu kali per undangan", "Pengembalian dana", "Masa aktif undangan", "Penggunaan yang dilarang", "Batasan tanggung jawab", "hukum Republik Indonesia", `href="/privacy"`},
	}
	for path, wants := range pages {
		rec := f.get(path, nil)
		body := rec.Body.String()
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		if strings.Contains(strings.ToLower(body), "draf") {
			t.Errorf("%s masih berlabel draf", path)
		}
		for _, want := range append(wants, "Berlaku sejak "+legalEffective, "kontak bantuan yang tercantum") {
			if !strings.Contains(body, want) {
				t.Errorf("%s tidak memuat %q", path, want)
			}
		}
		if strings.Count(body, "<h1") != 1 {
			t.Errorf("%s: harus tepat satu h1", path)
		}
	}
	// Nomor bantuan diatur admin (T25) → kontak menjadi tautan WhatsApp.
	web.SetSupport(web.Support{Phone: "6281234567890"})
	if body := f.get("/terms", nil).Body.String(); !strings.Contains(body, `href="https://wa.me/6281234567890"`) || !strings.Contains(body, "WhatsApp Bantuan Lunovia") {
		t.Error("kontak di halaman legal harus menautkan WhatsApp bantuan")
	}
}

// T27: SEO — judul & deskripsi berkata kunci, data terstruktur JSON-LD yang
// valid, etalase tema yang boleh diindeks, sitemap, llms.txt, kode verifikasi.
func TestSEO(t *testing.T) {
	f := newFixture(t)
	f.handler.GoogleVerification, f.handler.BingVerification = "g-kode", "b-kode"
	body := f.get("/", nil).Body.String()
	for _, want := range []string{
		"<title>Undangan Pernikahan Digital &amp; Website Pernikahan · Lunovia</title>",
		`name="description" content="Buat undangan pernikahan digital`,
		`property="og:locale" content="id_ID"`, `name="twitter:image"`,
		`name="google-site-verification" content="g-kode"`, `name="msvalidate.01" content="b-kode"`,
		`href="/tema"`, `href="/tema/signature"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("landing tidak memuat %q", want)
		}
	}
	// JSON-LD: JSON valid, memuat organisasi, aplikasi + harga, dan FAQ.
	m := regexp.MustCompile(`(?s)<script id="ld-site" type="application/ld\+json">(.*?)</script>`).FindStringSubmatch(body)
	if m == nil {
		t.Fatal("landing tanpa JSON-LD")
	}
	var doc struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(m[1]), &doc); err != nil {
		t.Fatalf("JSON-LD tidak valid: %v", err)
	}
	types := map[string]map[string]any{}
	for _, n := range doc.Graph {
		types[n["@type"].(string)] = n
	}
	for _, typ := range []string{"Organization", "WebSite", "SoftwareApplication", "FAQPage"} {
		if types[typ] == nil {
			t.Errorf("JSON-LD tanpa %s", typ)
		}
	}
	if offer, _ := types["SoftwareApplication"]["offers"].(map[string]any); offer["priceCurrency"] != "IDR" || offer["price"] != float64(149000) {
		t.Errorf("JSON-LD harga: %v", types["SoftwareApplication"]["offers"])
	}

	// Etalase tema: boleh diindeks, kanonik, satu h1, breadcrumb JSON-LD.
	for path, wants := range map[string][]string{
		"/tema":         {`rel="canonical" href="https://lovoria.test/tema"`, "Tema undangan pernikahan digital", `href="/tema/elegant"`, `href="/#fitur"`},
		"/tema/elegant": {`rel="canonical" href="https://lovoria.test/tema/elegant"`, "Tema undangan pernikahan Elegan", "Cormorant Garamond", "BreadcrumbList", `href="/register?next=%2Fdashboard%2Fweddings%2Fnew%3Ftema%3Delegant"`},
	} {
		rec := f.get(path, nil)
		b := rec.Body.String()
		if rec.Code != http.StatusOK || strings.Count(b, "<h1") != 1 || strings.Contains(b, `content="noindex"`) {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		for _, want := range wants {
			if !strings.Contains(b, want) {
				t.Errorf("%s tidak memuat %q", path, want)
			}
		}
	}
	if rec := f.get("/tema/tidak-ada", nil); rec.Code != http.StatusNotFound {
		t.Errorf("tema tak dikenal: %d", rec.Code)
	}
	// Tema yang dinonaktifkan admin hilang dari etalase, sitemap, dan llms.txt.
	if err := f.themes.SetEnabled(ctx, "modern", false); err != nil {
		t.Fatal(err)
	}
	if rec := f.get("/tema/modern", nil); rec.Code != http.StatusNotFound {
		t.Errorf("tema nonaktif: %d", rec.Code)
	}
	sm := f.get("/sitemap.xml", nil).Body.String()
	if !strings.Contains(sm, "<loc>https://lovoria.test/tema</loc>") || !strings.Contains(sm, "<loc>https://lovoria.test/tema/elegant</loc>") || strings.Contains(sm, "/tema/modern") {
		t.Errorf("sitemap: %s", sm)
	}
	rec := f.get("/llms.txt", nil)
	llm := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.HasPrefix(llm, "# Lunovia\n") || !strings.Contains(llm, "Rp149.000") || !strings.Contains(llm, "https://lovoria.test/tema/elegant") || strings.Contains(llm, "/tema/modern") {
		t.Errorf("llms.txt: %d %s", rec.Code, llm)
	}
	if rb := f.get("/robots.txt", nil).Body.String(); !strings.Contains(rb, "Allow: /tema") || !strings.Contains(rb, "Disallow: /w/") {
		t.Errorf("robots: %s", rb)
	}
	// Custom domain pasangan: tanpa etalase, sitemap, maupun llms.txt.
	_, w := f.newWedding(t, "a@example.com", "Samuel", "Sarah")
	f.publish(w.ID)
	f.domains["www.samuelsarah.com"] = w.ID
	custom := map[string]string{"Host": "www.samuelsarah.com"}
	for _, p := range []string{"/tema", "/tema/elegant", "/llms.txt", "/sitemap.xml"} {
		if rec := f.get(p, custom); rec.Code != http.StatusNotFound {
			t.Errorf("custom domain %s: %d", p, rec.Code)
		}
	}
	// Penutup undangan publik menautkan situs Lunovia (tetap noindex).
	inv := f.get("/", custom).Body.String() // undangan di custom domain pasangan
	if !strings.Contains(inv, `<a href="https://lovoria.test/" target="_blank" rel="noopener"`) || !strings.Contains(inv, `content="noindex"`) {
		t.Error("undangan: tautan Dibuat dengan Lunovia / noindex")
	}
}
