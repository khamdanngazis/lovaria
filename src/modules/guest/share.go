package guest

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/khamdanngazis/lovaria/src/platform/web"

	guestdb "github.com/khamdanngazis/lovaria/src/modules/guest/db"
)

// Bahasa template pesan undangan (T14).
const (
	LangID = "id"
	LangEN = "en"

	maxTemplateLen = 2000
)

// DefaultTemplates: template bawaan per bahasa. Placeholder: {guest_name},
// {couple}, {date}, {link}.
var DefaultTemplates = map[string]string{
	// Nama tamu sering sudah memuat sapaan ("Bapak Budi & Keluarga"), jadi
	// template bawaan tidak menambah "Bapak/Ibu" sendiri.
	LangID: "Kepada Yth.\n*{guest_name}*\n\n" +
		"Tanpa mengurangi rasa hormat, kami mengundang Anda untuk hadir di acara pernikahan kami, *{couple}*, pada {date}.\n\n" +
		"Info lengkap & konfirmasi kehadiran:\n{link}\n\n" +
		"Merupakan suatu kebahagiaan bagi kami apabila Anda berkenan hadir dan memberikan doa restu. 🙏\n\nTerima kasih.",
	LangEN: "Dear *{guest_name}*,\n\n" +
		"We would be honored to have you at our wedding, *{couple}*, on {date}.\n\n" +
		"Details & RSVP:\n{link}\n\n" +
		"Your presence and prayers would mean the world to us. 🙏\n\nThank you.",
}

// genericGuest: pengganti {guest_name} untuk link umum (tanpa tamu tertentu).
var genericGuest = map[string]string{LangID: "Bapak/Ibu/Saudara/i", LangEN: "Guest"}

// ShareTemplate adalah template pesan wedding.
type ShareTemplate struct {
	Language string
	Body     string
	Custom   bool // false = template bawaan
}

// ShareVars adalah nilai placeholder satu pesan.
type ShareVars struct {
	GuestName, Couple, Date, Link string
}

// Render mengganti placeholder template dengan nilai.
func Render(body string, v ShareVars) string {
	return strings.NewReplacer(
		"{guest_name}", v.GuestName, "{couple}", v.Couple, "{date}", v.Date, "{link}", v.Link,
	).Replace(body)
}

// WhatsAppURL: link wa.me dengan pesan terisi. Nomor sudah dinormalisasi
// "62…" oleh T07; tanpa nomor → WhatsApp meminta memilih kontak.
func WhatsAppURL(phone, message string) string {
	// QueryEscape menulis spasi sebagai "+", yang tidak selalu dibaca WhatsApp
	// sebagai spasi — pakai %20. Emoji & baris baru ter-encode sebagai UTF-8.
	return "https://wa.me/" + phone + "?text=" + strings.ReplaceAll(url.QueryEscape(message), "+", "%20")
}

var monthsEN = [...]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}

// DateText: tanggal pernikahan sesuai bahasa template.
func DateText(t time.Time, lang string) string {
	if lang == LangEN {
		return fmt.Sprintf("%s, %d %s %d", t.Weekday(), t.Day(), monthsEN[t.Month()-1], t.Year())
	}
	return web.FormatDateID(t)
}

// ShareTemplate mengembalikan template wedding (bawaan Bahasa Indonesia bila belum diatur).
func (s *Service) ShareTemplate(ctx context.Context, weddingID uuid.UUID) (ShareTemplate, error) {
	r, err := s.repo.q.GetShareTemplate(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ShareTemplate{Language: LangID, Body: DefaultTemplates[LangID]}, nil
	}
	if err != nil {
		return ShareTemplate{}, err
	}
	return ShareTemplate{Language: r.Language, Body: r.Body, Custom: true}, nil
}

// SaveShareTemplate menyimpan template; wajib memuat {link} supaya pesan
// selalu berisi link undangan.
func (s *Service) SaveShareTemplate(ctx context.Context, weddingID uuid.UUID, lang, body string) (ShareTemplate, error) {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	v := ValidationError{}
	if _, ok := DefaultTemplates[lang]; !ok {
		v["language"] = "Pilih bahasa"
	}
	switch n := utf8.RuneCountInString(body); {
	case n == 0:
		v["body"] = "Pesan wajib diisi"
	case n > maxTemplateLen:
		v["body"] = fmt.Sprintf("Pesan maksimal %d karakter", maxTemplateLen)
	case !strings.Contains(body, "{link}"):
		v["body"] = "Pesan harus memuat {link} supaya tamu mendapat link undangan"
	}
	if len(v) > 0 {
		return ShareTemplate{}, v
	}
	if err := s.repo.q.UpsertShareTemplate(ctx, guestdb.UpsertShareTemplateParams{WeddingID: weddingID, Language: lang, Body: body}); err != nil {
		return ShareTemplate{}, err
	}
	return ShareTemplate{Language: lang, Body: body, Custom: true}, nil
}

// ResetShareTemplate kembali ke template bawaan.
func (s *Service) ResetShareTemplate(ctx context.Context, weddingID uuid.UUID) error {
	return s.repo.q.DeleteShareTemplate(ctx, weddingID)
}

// MarkShared mencatat waktu undangan tamu dibagikan (Kirim WA / Salin).
func (s *Service) MarkShared(ctx context.Context, weddingID, id uuid.UUID) error {
	now := s.now()
	n, err := s.repo.q.MarkShared(ctx, guestdb.MarkSharedParams{ID: id, WeddingID: weddingID, SharedAt: &now})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetOrigins memasang sumber basis URL link undangan per wedding (custom
// domain aktif, T15). Tanpa ini: BASE_URL.
func (s *Service) SetOrigins(f func(ctx context.Context, weddingID uuid.UUID) (string, error)) {
	s.origins = f
}

// Origin: basis URL link undangan wedding (https://custom-domain atau BASE_URL).
func (s *Service) Origin(ctx context.Context, weddingID uuid.UUID) (string, error) {
	if s.origins == nil {
		return s.baseURL, nil
	}
	return s.origins(ctx, weddingID)
}

// Link: link undangan pribadi tamu dengan basis origin.
func Link(origin, code string) string { return origin + "/i/" + code }
