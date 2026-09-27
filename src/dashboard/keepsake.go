package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/text/encoding/charmap"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// Keepsake membuat PDF kenang-kenangan (T19): sampul, cerita cinta, ringkasan
// RSVP, dan semua ucapan yang tampil. Dibuat on-demand, tidak disimpan.
type Keepsake struct {
	Weddings  *wedding.Service
	Stories   *story.Service
	Guests    *guest.Service
	Guestbook *guestbook.Service
	// Photo mengambil foto sampul (JPEG); nil / error → sampul tanpa foto.
	Photo func(ctx context.Context, url string) ([]byte, error)
}

// maxKeepsakeEntries membatasi ucapan per PDF (jaga waktu & ukuran file).
const maxKeepsakeEntries = 5000

// Register: GET /dashboard/weddings/:weddingID/keepsake.pdf (grup owner).
func (k *Keepsake) Register(owned *echo.Group) {
	owned.GET("/keepsake.pdf", k.Download)
}

func (k *Keepsake) Download(c echo.Context) error {
	ctx := c.Request().Context()
	w, ok := wedding.FromContext(ctx)
	if !ok {
		return echo.ErrNotFound
	}
	var buf bytes.Buffer
	if err := k.Write(ctx, w, &buf); err != nil {
		return err
	}
	hdr := c.Response().Header()
	hdr.Set("Cache-Control", "private, no-store")
	hdr.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="kenangan-%s.pdf"`, w.Slug))
	return c.Blob(http.StatusOK, "application/pdf", buf.Bytes())
}

type keepsakeData struct {
	Couple  wedding.Couple
	Stories []story.Story
	Stats   guest.Stats
	Entries []guestbook.Entry
	Photo   []byte
}

func (k *Keepsake) load(ctx context.Context, w wedding.Wedding) (keepsakeData, error) {
	var d keepsakeData
	var err error
	if d.Couple, err = k.Weddings.GetCouple(ctx, w.ID); err != nil {
		return d, fmt.Errorf("keepsake: pasangan: %w", err)
	}
	if d.Stories, err = k.Stories.ListStories(ctx, w.ID); err != nil {
		return d, fmt.Errorf("keepsake: cerita: %w", err)
	}
	if d.Stats, err = k.Guests.Stats(ctx, w.ID); err != nil {
		return d, fmt.Errorf("keepsake: tamu: %w", err)
	}
	// Semua ucapan yang tampil (tanpa yang disembunyikan), terbaru dulu.
	before := uuid.Nil
	for len(d.Entries) < maxKeepsakeEntries {
		es, more, err := k.Guestbook.Visible(ctx, w.ID, before, 500)
		if err != nil {
			return d, fmt.Errorf("keepsake: ucapan: %w", err)
		}
		d.Entries = append(d.Entries, es...)
		if !more {
			break
		}
		before = es[len(es)-1].ID
	}
	if w.MainPhotoURL != nil && *w.MainPhotoURL != "" && k.Photo != nil {
		d.Photo, _ = k.Photo(ctx, *w.MainPhotoURL) // foto opsional: gagal → tanpa foto
	}
	return d, nil
}

// Write menulis PDF kenang-kenangan wedding w ke out.
func (k *Keepsake) Write(ctx context.Context, w wedding.Wedding, out *bytes.Buffer) error {
	d, err := k.load(ctx, w)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(w.Timezone)
	if err != nil {
		loc = time.UTC
	}
	return renderKeepsake(w, d, loc, out)
}

// Warna brand (Dusty Plum, Champagne, Muted).
var (
	plum      = [3]int{0x6B, 0x4E, 0x71}
	champagne = [3]int{0xC9, 0xA8, 0x8A}
	muted     = [3]int{0x6B, 0x66, 0x6B}
	ink       = [3]int{0x29, 0x25, 0x29}
)

// keepsakeCompress: kompresi stream PDF (dimatikan di test supaya isi teks bisa diperiksa).
var keepsakeCompress = true

func renderKeepsake(w wedding.Wedding, d keepsakeData, loc *time.Location, out *bytes.Buffer) error {
	pdf := fpdf.New("P", "mm", "A5", "")
	pdf.SetCompression(keepsakeCompress)
	pdf.SetTitle(pdfText("Kenang-kenangan "+d.Couple.GroomName+" & "+d.Couple.BrideName), false)
	pdf.SetCreator("Lovoria", false)
	pdf.SetMargins(16, 18, 16)
	pdf.SetAutoPageBreak(true, 18)
	pageW, _ := pdf.GetPageSize()
	textW := pageW - 32
	color := func(c [3]int) { pdf.SetTextColor(c[0], c[1], c[2]) }
	pdf.SetFooterFunc(func() {
		if pdf.PageNo() == 1 {
			return
		}
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		color(muted)
		pdf.CellFormat(0, 5, pdfText(fmt.Sprintf("%s & %s · %d", d.Couple.GroomName, d.Couple.BrideName, pdf.PageNo())), "", 0, "C", false, 0, "")
	})
	heading := func(title string) {
		pdf.SetFont("Times", "", 22)
		color(plum)
		pdf.CellFormat(0, 12, pdfText(title), "", 1, "L", false, 0, "")
		pdf.SetDrawColor(champagne[0], champagne[1], champagne[2])
		pdf.Line(16, pdf.GetY(), 46, pdf.GetY())
		pdf.Ln(6)
	}

	// Sampul.
	pdf.AddPage()
	y := 30.0
	if len(d.Photo) > 0 {
		opt := fpdf.ImageOptions{ImageType: "JPG", ReadDpi: false}
		info := pdf.RegisterImageOptionsReader("cover", opt, bytes.NewReader(d.Photo))
		if pdf.Ok() && info != nil && info.Width() > 0 {
			h := min(textW*info.Height()/info.Width(), 95)
			wImg := h * info.Width() / info.Height()
			pdf.ImageOptions("cover", (pageW-wImg)/2, 24, wImg, h, false, opt, 0, "")
			y = 24 + h + 12
		} else {
			pdf.ClearError() // foto rusak / bukan JPEG → sampul tanpa foto
		}
	}
	pdf.SetY(y)
	pdf.SetFont("Helvetica", "", 9)
	color(champagne)
	pdf.CellFormat(0, 6, "KENANG-KENANGAN PERNIKAHAN", "", 1, "C", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Times", "", 28)
	color(plum)
	pdf.MultiCell(0, 12, pdfText(d.Couple.GroomName+" & "+d.Couple.BrideName), "", "C", false)
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "", 11)
	color(ink)
	pdf.CellFormat(0, 7, pdfText(web.FormatDateID(w.WeddingDate)), "", 1, "C", false, 0, "")
	pdf.SetY(-30)
	pdf.SetFont("Helvetica", "", 8)
	color(muted)
	pdf.CellFormat(0, 5, pdfText("Dibuat dengan Lovoria · Your Love. Your Story. Your Forever."), "", 1, "C", false, 0, "")

	// Cerita cinta.
	if len(d.Stories) > 0 {
		pdf.AddPage()
		heading("Cerita Cinta")
		for _, s := range d.Stories {
			pdf.SetFont("Helvetica", "B", 8)
			color(champagne)
			pdf.CellFormat(0, 5, pdfText(strings.ToUpper(s.Date.String())), "", 1, "L", false, 0, "")
			pdf.SetFont("Times", "", 14)
			color(plum)
			pdf.MultiCell(0, 7, pdfText(s.Title), "", "L", false)
			if s.Description != "" {
				pdf.SetFont("Helvetica", "", 10)
				color(ink)
				pdf.MultiCell(0, 5.5, pdfText(s.Description), "", "L", false)
			}
			pdf.Ln(5)
		}
	}

	// Ringkasan RSVP.
	pdf.AddPage()
	heading("Kehadiran")
	pdf.SetFont("Helvetica", "", 11)
	color(ink)
	rows := [][2]string{
		{"Tamu diundang", fmt.Sprintf("%d undangan", d.Stats.Total)},
		{"Hadir", fmt.Sprintf("%d undangan · %d orang", d.Stats.Attending, d.Stats.PaxAttending)},
		{"Berhalangan", fmt.Sprintf("%d undangan", d.Stats.Declined)},
		{"Ucapan & doa", fmt.Sprintf("%d pesan", len(d.Entries))},
	}
	for _, r := range rows {
		color(muted)
		pdf.CellFormat(40, 8, pdfText(r[0]), "", 0, "L", false, 0, "")
		color(ink)
		pdf.CellFormat(0, 8, pdfText(r[1]), "", 1, "L", false, 0, "")
	}

	// Ucapan & doa, urut waktu (terlama dulu) seperti buku tamu.
	if len(d.Entries) > 0 {
		pdf.Ln(8)
		heading("Ucapan & Doa")
		for i := len(d.Entries) - 1; i >= 0; i-- {
			e := d.Entries[i]
			pdf.SetFont("Helvetica", "", 10)
			color(ink)
			pdf.MultiCell(0, 5.2, pdfText(e.Message), "", "L", false)
			pdf.SetFont("Helvetica", "B", 8)
			color(plum)
			pdf.CellFormat(0, 5, pdfText(e.Name+" · "+web.FormatDateID(e.CreatedAt.In(loc))), "", 1, "L", false, 0, "")
			pdf.Ln(3)
		}
	}
	return pdf.Output(out)
}

// pdfText menyesuaikan teks untuk font inti PDF (Windows-1252): karakter yang
// tidak didukung (emoji, aksara non-Latin) dihilangkan.
func pdfText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' || r == '\t' {
			b.WriteByte(byte(r))
			continue
		}
		if c, ok := charmap.Windows1252.EncodeRune(r); ok && c >= 0x20 {
			b.WriteByte(c)
		}
	}
	// Rapikan spasi ganda sisa emoji yang dihapus.
	return strings.Join(strings.FieldsFunc(b.String(), func(r rune) bool { return r == ' ' }), " ")
}

// maxKeepsakePhoto: batas ukuran foto sampul yang diunduh untuk PDF.
const maxKeepsakePhoto = 8 << 20

// HTTPPhoto mengambil foto lewat HTTP (URL R2 publik; URL relatif — storage
// lokal dev — dilengkapi baseURL) dengan timeout & batas ukuran.
func HTTPPhoto(baseURL string) func(ctx context.Context, url string) ([]byte, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	return func(ctx context.Context, url string) ([]byte, error) {
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			url = strings.TrimSuffix(baseURL, "/") + "/" + strings.TrimPrefix(url, "/")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("keepsake: foto %s: status %d", url, resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, maxKeepsakePhoto+1))
		if err != nil {
			return nil, err
		}
		if len(b) > maxKeepsakePhoto {
			return nil, fmt.Errorf("keepsake: foto %s terlalu besar", url)
		}
		return b, nil
	}
}
