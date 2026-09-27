package publicsite

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/theme/view"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// openedEvery: selang minimum pembaruan last_opened_at per tamu.
const openedEvery = 15 * time.Minute

type Handler struct {
	// BaseURL: URL utama Lovoria (kanonik & gambar OG landing page).
	BaseURL string
	// Packages: paket yang ditampilkan di landing (admin.Service); nil → tanpa harga.
	Packages  LandingPackages
	Views     *ViewBuilder
	Guests    *guest.Service
	Guestbook *guestbook.Service
	Events    *event.Service
	Log       *slog.Logger
	// Secret kunci HMAC token form RSVP (config APP_SECRET).
	Secret []byte
	now    func() time.Time
}

func (h *Handler) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

// Home: "/" — undangan bila Host adalah custom domain, selain itu landing page.
func (h *Handler) Home(c echo.Context) error {
	if _, ok := FromContext(c.Request().Context()); ok {
		return h.Invitation(c)
	}
	d, err := h.landing(c)
	if err != nil {
		return err
	}
	// Konten landing sama untuk semua pengunjung kecuali tombol login/dashboard.
	c.Response().Header().Set("Cache-Control", "private, max-age=300")
	return web.Render(c, http.StatusOK, landingPage(d))
}

// Invitation: GET /i/:code, /w/:slug (dan "/" di custom domain).
func (h *Handler) Invitation(c echo.Context) error {
	ctx := c.Request().Context()
	res, _ := FromContext(ctx)
	// Arsip (T19): tampil read-only bila publik atau dibuka pemiliknya;
	// arsip privat untuk publik tetap halaman ringkas.
	if res.Wedding.IsArchived() && !res.Wedding.ArchivePublic() && !h.isOwner(ctx, res) {
		return h.archived(c, res)
	}
	build := h.Views.BuildPublic
	if res.Preview {
		build = h.Views.Build // pemilik melihat perubahan terbaru, tanpa cache
	}
	v, err := build(ctx, res.Wedding, res.Guest)
	if err != nil {
		return err
	}
	v.Preview = res.Preview
	if !res.Wedding.ShowsMemoryLayout() { // setelah hari H kalender tidak relevan
		for i := range v.Events {
			v.Events[i].CalendarURL = res.Prefix + "/events/" + v.Events[i].ID + ".ics"
			if v.Events[i].ID == v.Countdown.EventID {
				v.Countdown.CalendarURL = v.Events[i].CalendarURL
			}
		}
	}
	v.OG = h.og(res, v)
	if !res.Preview {
		h.setGuestbookForm(&v, res)
		if c.QueryParam("guestbook") == "ok" { // kembali dari form tanpa JS
			v.Guestbook.Notice = guestbookNotice
		}
	}
	if res.Guest != nil && !res.Preview {
		v.RSVP.Action = res.Prefix + "/rsvp"
		v.RSVP.Token = h.rsvpToken(res.Guest.InvitationCode, h.clock())
		if c.QueryParam("rsvp") == "ok" { // kembali dari form tanpa JS
			v.RSVP.Notice = rsvpNotice(res.Guest.RSVPStatus)
		}
	}

	// Catat "sudah dibuka" paling sering tiap 15 menit per tamu: menghindari
	// UPDATE di setiap page view saat undangan ramai dibuka (load test T17).
	if g := res.Guest; g != nil && !res.Preview && (g.LastOpenedAt == nil || h.clock().Sub(*g.LastOpenedAt) > openedEvery) {
		if err := h.Guests.MarkOpened(ctx, res.Wedding.ID, res.Guest.ID); err != nil {
			h.Log.WarnContext(ctx, "public: mark opened", slog.String("error", err.Error()))
		}
	}

	var buf bytes.Buffer
	if err := theme.Render(v).Render(ctx, &buf); err != nil {
		return err
	}
	hdr := c.Response().Header()
	hdr.Set("X-Robots-Tag", "noindex")
	if res.Preview {
		hdr.Set("Cache-Control", "no-store")
	} else {
		if res.Wedding.IsArchived() && !res.Wedding.ArchivePublic() {
			// Arsip privat yang dibuka pemilik: jangan sampai tersimpan di cache bersama.
			hdr.Set("Cache-Control", "private, no-store")
		} else if res.Guest != nil {
			// Halaman tamu memuat status RSVP-nya: selalu validasi ulang (ETag → 304)
			// supaya jawaban yang baru dikirim langsung terlihat saat link dibuka lagi.
			hdr.Set("Cache-Control", "private, no-cache")
		} else {
			hdr.Set("Cache-Control", "public, max-age=60")
		}
		sum := sha256.Sum256(buf.Bytes())
		etag := `W/"` + hex.EncodeToString(sum[:12]) + `"`
		hdr.Set("ETag", etag)
		if match := c.Request().Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
			return c.NoContent(http.StatusNotModified)
		}
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

// isOwner: request dari pemilik wedding yang sedang login.
func (h *Handler) isOwner(ctx context.Context, res Resolved) bool {
	u, ok := web.CurrentUser(ctx)
	return ok && u.ID == res.Wedding.OwnerUserID
}

// archived: halaman ringkas untuk undangan yang telah diarsipkan.
func (h *Handler) archived(c echo.Context, res Resolved) error {
	couple, err := h.Views.Weddings.GetCouple(c.Request().Context(), res.Wedding.ID)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Cache-Control", "public, max-age=300")
	c.Response().Header().Set("X-Robots-Tag", "noindex")
	return web.Render(c, http.StatusOK, archivedPage(firstName(couple.GroomName)+" & "+firstName(couple.BrideName), web.FormatDateID(res.Wedding.WeddingDate)))
}

// og menyusun meta preview link (WhatsApp dll.): nama pasangan, tanggal, foto sampul.
func (h *Handler) og(res Resolved, v view.View) view.OG {
	desc := v.DateText
	if len(v.Events) > 0 && v.Events[0].Venue != "" {
		desc += " · " + v.Events[0].Venue
	}
	if res.Guest != nil {
		desc = "Kepada Yth. " + res.Guest.Name + " — " + desc
	}
	image := v.Settings.CoverImage
	if image == "" {
		image = v.MainPhoto
	}
	return view.OG{
		Title:       "The Wedding of " + v.Couple.Names(),
		Description: desc,
		Image:       absolute(res.Origin, image),
		URL:         res.Origin + res.Prefix,
	}
}

// absolute menjadikan URL relatif (/media/…, storage lokal dev) absolut.
func absolute(origin, u string) string {
	if u == "" || strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}
	return origin + "/" + strings.TrimPrefix(u, "/")
}

// Calendar: GET …/events/:file (file = "<event-id>.ics").
func (h *Handler) Calendar(c echo.Context) error {
	ctx := c.Request().Context()
	res, _ := FromContext(ctx)
	id, err := uuid.Parse(strings.TrimSuffix(c.Param("file"), ".ics"))
	if err != nil || !strings.HasSuffix(c.Param("file"), ".ics") {
		return notFound(c)
	}
	e, err := h.Events.GetEvent(ctx, res.Wedding.ID, id)
	if errors.Is(err, event.ErrNotFound) {
		return notFound(c)
	}
	if err != nil {
		return err
	}
	start, end, err := eventTimes(e.Date, e.StartTime, e.EndTime, res.Wedding.Timezone)
	if err != nil {
		return err
	}
	couple, err := h.Views.Weddings.GetCouple(ctx, res.Wedding.ID)
	if err != nil {
		return err
	}
	loc := e.Venue
	if e.Address != "" {
		loc += ", " + e.Address
	}
	body := buildICS(icsEvent{
		UID:         e.ID.String() + "@lovoria",
		Summary:     fmt.Sprintf("%s — %s & %s", e.Name, firstName(couple.GroomName), firstName(couple.BrideName)),
		Location:    loc,
		Description: strings.TrimSpace(e.Description + "\n" + e.MapsURL),
		URL:         res.Origin + res.Prefix,
		Start:       start, End: end,
	}, h.clock())

	hdr := c.Response().Header()
	hdr.Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s.ics"`, slugFile(e.Name)))
	hdr.Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, "text/calendar; charset=utf-8", []byte(body))
}

func firstName(s string) string {
	if f := strings.Fields(s); len(f) > 0 {
		return f[0]
	}
	return s
}

func slugFile(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0:
			b.WriteByte('-')
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "acara"
}

// GET /privacy, /terms — halaman legal (draf).
func (h *Handler) Privacy(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return web.Render(c, http.StatusOK, privacyPage())
}

func (h *Handler) Terms(c echo.Context) error {
	c.Response().Header().Set("Cache-Control", "public, max-age=3600")
	return web.Render(c, http.StatusOK, termsPage())
}
