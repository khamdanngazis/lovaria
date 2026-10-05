package publicsite

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/payment"
	"github.com/khamdanngazis/lovaria/static"
)

// OwnHost: request datang ke domain Lunovia sendiri (bukan custom domain pasangan).
func (r *Resolver) OwnHost(req *http.Request) bool { return r.isOwnHost(r.hostOf(req)) }

// Kata kunci utama halaman publik (T27) — satu tempat supaya mudah disunting.
const (
	seoTitle = "Undangan Pernikahan Digital & Website Pernikahan"
	seoDesc  = "Buat undangan pernikahan digital dan website pernikahan yang personal di Lunovia: pilihan tema, RSVP online, buku ucapan, amplop digital, galeri, dan link pribadi per tamu. Gratis dibuat, bayar sekali saat dipublikasikan."
)

// robots: domain Lunovia → landing, halaman tema & legal boleh diindeks,
// undangan & dashboard tidak; custom domain pasangan → tidak diindeks sama sekali.
func robots(r *Resolver) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=3600")
		if !r.OwnHost(c.Request()) {
			return c.String(http.StatusOK, "User-agent: *\nDisallow: /\n")
		}
		base := strings.TrimRight(r.BaseURL, "/")
		return c.String(http.StatusOK, "User-agent: *\nAllow: /$\nAllow: /tema\nAllow: /privacy\nAllow: /terms\nAllow: /llms.txt\n"+
			"Disallow: /dashboard\nDisallow: /admin\nDisallow: /i/\nDisallow: /w/\nDisallow: /login\nDisallow: /register\n\n"+
			"Sitemap: "+base+"/sitemap.xml\n")
	}
}

// activeThemes: tema yang ditawarkan (tidak dinonaktifkan admin), berikut
// undangan contohnya bila ada.
func (h *Handler) activeThemes(c echo.Context) ([]landingTheme, error) {
	d, err := h.landing(c)
	return d.Themes, err
}

// Sitemap: landing, etalase tema (hanya tema aktif), dan halaman legal.
func (h *Handler) Sitemap(c echo.Context) error {
	if !h.ownHost(c.Request()) {
		return notFound(c)
	}
	themes, err := h.activeThemes(c)
	if err != nil {
		return err
	}
	base := strings.TrimRight(h.BaseURL, "/")
	paths := []string{"/", "/tema"}
	for _, t := range themes {
		paths = append(paths, "/tema/"+t.ID)
	}
	paths = append(paths, "/privacy", "/terms")
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, p := range paths {
		b.WriteString("  <url><loc>" + base + p + "</loc></url>\n")
	}
	b.WriteString("</urlset>\n")
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.Blob(http.StatusOK, "application/xml; charset=utf-8", []byte(b.String()))
}

// LLMs: ringkasan situs untuk asisten AI & mesin pencari berbasis AI
// (konvensi llms.txt) — apa itu Lunovia, harga, tema, dan halaman utamanya.
func (h *Handler) LLMs(c echo.Context) error {
	if !h.ownHost(c.Request()) {
		return notFound(c)
	}
	themes, err := h.activeThemes(c)
	if err != nil {
		return err
	}
	base := strings.TrimRight(h.BaseURL, "/")
	var b strings.Builder
	b.WriteString("# Lunovia\n\n> Lunovia adalah layanan berbahasa Indonesia untuk membuat undangan pernikahan digital dan website pernikahan: pasangan menyusun undangan dari ponsel, membagikan link pribadi ke tiap tamu lewat WhatsApp, menerima RSVP dan ucapan, lalu menyimpannya sebagai halaman kenangan setelah hari H.\n\n")
	b.WriteString("## Ringkasan\n\n")
	b.WriteString("- Fitur: pilihan tema, cerita cinta, rangkaian acara dengan peta & simpan ke kalender, galeri foto, RSVP online, buku ucapan & doa, amplop digital, link undangan pribadi per tamu, musik latar, hitung mundur, domain kustom, halaman kenangan.\n")
	if h.PublishPrice > 0 {
		b.WriteString("- Harga: membuat, menyunting, dan melihat pratinjau undangan gratis; " + payment.Rupiah(h.PublishPrice) + " sekali bayar per undangan saat dipublikasikan, tanpa langganan.\n")
	}
	b.WriteString("- Tamu tidak perlu memasang aplikasi; undangan dibuka di browser.\n- Bahasa: Indonesia.\n\n")
	b.WriteString("## Halaman\n\n")
	b.WriteString("- [Beranda](" + base + "/): penjelasan layanan, cara kerja, fitur, harga, dan FAQ\n")
	b.WriteString("- [Tema undangan](" + base + "/tema): semua tema beserta contoh undangan\n")
	for _, t := range themes {
		b.WriteString("- [Tema " + t.Name + "](" + base + "/tema/" + t.ID + "): " + t.Description + "\n")
	}
	b.WriteString("- [Daftar](" + base + "/register): buat akun gratis\n")
	b.WriteString("- [Syarat & Ketentuan](" + base + "/terms)\n- [Kebijakan Privasi](" + base + "/privacy)\n")
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}

// ---------- Data terstruktur (JSON-LD, schema.org) ----------

type ld map[string]any

// landingLD: Organization + WebSite + SoftwareApplication (dengan harga) +
// FAQPage — membantu mesin pencari & asisten AI memahami layanan.
func landingLD(d landingData) ld {
	base := strings.TrimSuffix(d.URL, "/")
	org := ld{"@type": "Organization", "@id": base + "/#organization", "name": "Lunovia", "url": d.URL, "logo": base + static.URL("img/brand/apple-touch-icon.png")}
	site := ld{"@type": "WebSite", "@id": base + "/#website", "url": d.URL, "name": "Lunovia", "description": seoDesc, "inLanguage": "id-ID", "publisher": ld{"@id": base + "/#organization"}}
	app := ld{
		"@type": "SoftwareApplication", "name": "Lunovia", "url": d.URL, "description": seoDesc, "image": d.OGImage,
		"applicationCategory": "LifestyleApplication", "operatingSystem": "Web", "inLanguage": "id-ID",
	}
	if d.PriceIDR > 0 {
		app["offers"] = ld{"@type": "Offer", "price": d.PriceIDR, "priceCurrency": "IDR", "description": "Sekali bayar per undangan saat dipublikasikan; membuat dan pratinjau gratis."}
	}
	graph := []ld{org, site, app}
	var qs []ld
	for _, q := range faqs(d) {
		qs = append(qs, ld{"@type": "Question", "name": q[0], "acceptedAnswer": ld{"@type": "Answer", "text": q[1]}})
	}
	if len(qs) > 0 {
		graph = append(graph, ld{"@type": "FAQPage", "@id": base + "/#faq", "mainEntity": qs})
	}
	return ld{"@context": "https://schema.org", "@graph": graph}
}

// breadcrumbLD: jejak halaman (Beranda › Tema › …) untuk hasil pencarian.
func breadcrumbLD(base string, crumbs ...[2]string) ld {
	items := make([]ld, 0, len(crumbs))
	for i, c := range crumbs {
		items = append(items, ld{"@type": "ListItem", "position": i + 1, "name": c[0], "item": base + c[1]})
	}
	return ld{"@context": "https://schema.org", "@type": "BreadcrumbList", "itemListElement": items}
}
