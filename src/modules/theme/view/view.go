// Package view adalah kontrak data antara penyusun halaman undangan (public
// site, T09) dan tema. Tema hanya boleh membaca View; tidak memanggil service.
package view

import (
	"time"

	"github.com/a-h/templ"
)

// View adalah seluruh data yang dibutuhkan satu halaman undangan.
type View struct {
	ThemeID     string
	Slug        string
	Title       string
	Description string
	Date        time.Time
	DateText    string // "Sabtu, 12 Desember 2026"
	TZAbbr      string // "WIB"
	MainPhoto   string

	Couple  Couple
	Guest   *Guest // nil bila dibuka lewat /w/:slug
	Events  []Event
	Stories []Story
	Gallery []Photo

	// Settings: pengaturan tampilan milik wedding (kosong = default tema).
	Settings Settings
	// Tokens: token final (default tema + Settings), diisi theme.Render.
	Tokens Tokens
	// CSS & FontsURL: blok CSS variable dan URL Google Fonts, diisi theme.Render.
	CSS      string
	FontsURL string
	// SectionTitle: judul bagian milik tema (T21), diisi theme.Prepare supaya
	// bagian bersama (RSVP, ucapan, hadiah, …) memakai gaya tema tanpa
	// mengenal ID tema. nil → judul bawaan.
	SectionTitle func(title, subtitle string) templ.Component
	// CoverThumb / CoverWidth: thumbnail & lebar foto sampul untuk srcset
	// (kosong bila sampul bukan foto galeri).
	CoverThumb string
	CoverWidth int

	// OG: meta Open Graph / Twitter untuk preview link (WhatsApp dll.), diisi public site.
	OG OG

	// Status lifecycle (dari guard wedding, T12): Memory → banner terima kasih;
	// AllowRSVP / AllowGuestbook menentukan apakah form tampil (T10/T11).
	Memory         bool
	AllowRSVP      bool
	AllowGuestbook bool
	// Archived: undangan diarsipkan, tampil read-only (T19). Memory || Archived →
	// tata letak kenangan: foto hari-H (MemoryPhotos) & ucapan favorit di atas.
	Archived     bool
	MemoryPhotos []Photo

	// RSVP: form konfirmasi kehadiran tamu (T10); Action kosong → form nonaktif.
	RSVP RSVPForm
	// Guestbook: form & daftar ucapan (T11); Gifts: amplop digital (T11).
	Guestbook GuestbookView
	Gifts     []Gift

	// Personalisasi (T20). Music, Quote, Greeting & Closing diisi theme.Render
	// dari Settings; Countdown diisi penyusun view (butuh acara & jam sekarang).
	Music     Music
	Countdown Countdown
	Quote     Quote
	Greeting  string
	Closing   string

	// Preview: tampilkan banner "Preview" (owner melihat draft / dashboard).
	Preview bool
	// Sample: data contoh dipakai untuk bagian yang masih kosong (preview dashboard).
	Sample bool
}

// Music: musik latar — diputar setelah tamu menekan "Buka Undangan", tidak autoplay.
type Music struct {
	URL     string
	Enabled bool
}

// On: tombol musik & <audio> dirender.
func (m Music) On() bool { return m.Enabled && m.URL != "" }

// Countdown: hitung mundur menuju acara pertama (zona waktu wedding).
type Countdown struct {
	Show        bool      // false setelah hari acara lewat / tanpa tanggal
	Target      time.Time // mulai acara pertama (UTC)
	DaysLeft    int       // sisa hari kalender (teks tanpa JS)
	Today       bool      // hari H → "Hari ini!"
	EventID     string    // acara pertama (untuk tautan kalender)
	CalendarURL string    // .ics acara pertama, diisi public site
}

// Quote: kutipan / ayat pembuka pilihan pasangan.
type Quote struct {
	Text, Source string
}

type Couple struct {
	GroomName, BrideName   string
	GroomPhoto, BridePhoto string
	GroomDesc, BrideDesc   string
}

// Names: "Khamdan & Sarah" (nama depan).
func (c Couple) Names() string { return first(c.GroomName) + " & " + first(c.BrideName) }

func first(s string) string {
	for i, r := range s {
		if r == ' ' {
			return s[:i]
		}
	}
	return s
}

type Guest struct {
	Name        string
	Code        string
	MaxPax      int
	RSVPStatus  string // pending | attending | declined
	RSVPPax     int
	RSVPMessage string
}

type Event struct {
	ID              string
	Name, TypeLabel string
	DateText        string // "Sabtu, 12 Desember 2026"
	TimeText        string // "08.00–10.00 WIB"
	Venue, Address  string
	MapsURL         string
	CalendarURL     string // .ics (T09)
	Description     string
}

// RSVPForm adalah keadaan form RSVP di halaman undangan.
type RSVPForm struct {
	Action string // URL POST, mis. "/i/KODE/rsvp"
	Token  string // token HMAC anti-spam (pengganti CSRF di halaman yang di-cache)
	Notice string // pesan sukses
	Error  string // pesan umum (mis. RSVP ditutup, halaman kedaluwarsa)
	Errors map[string]string
	// Nilai isian (dipakai lagi saat validasi gagal).
	Status  string
	Pax     int
	Message string
	// ToGuestbook: tamu memilih menyalin pesan ke buku ucapan.
	ToGuestbook bool
}

// GuestbookView adalah keadaan section buku ucapan.
type GuestbookView struct {
	Action  string // URL POST (GET ?before= untuk muat lebih banyak); kosong → form nonaktif
	Token   string
	Entries []GuestbookEntry
	// MoreBefore: ID entri terakhir bila masih ada pesan berikutnya.
	MoreBefore string
	Notice     string
	Error      string
	Errors     map[string]string
	Name       string // isian (terisi nama tamu bila lewat /i/:code)
	Message    string
	// Favorites: ucapan favorit pilihan pasangan (tampil di bagian kenangan).
	Favorites []GuestbookEntry
}

type GuestbookEntry struct {
	Name, Message, DateText string
}

// Gift adalah satu rekening / e-wallet / alamat. Nomor rekening hanya boleh
// tampil di section hadiah (bukan OG meta atau halaman lain).
type Gift struct {
	Type          string // bank | ewallet | address
	TypeLabel     string
	Provider      string
	AccountNumber string
	AccountName   string
	Address       string
}

// ShowsMemory: halaman tampil sebagai kenangan (setelah hari H).
func (v View) ShowsMemory() bool { return v.Memory || v.Archived }

// OG adalah meta preview link. URL wajib absolut.
type OG struct {
	Title       string
	Description string
	Image       string
	URL         string // URL kanonik halaman
}

type Story struct {
	DateText, Title, Description, PhotoURL string
}

type Photo struct {
	URL, ThumbURL, Caption string
	Width, Height          int
}

// Settings adalah override tampilan per wedding. String kosong = pakai default tema.
type Settings struct {
	PrimaryColor string // #rrggbb
	FontHeading  string // nama font dari whitelist
	FontBody     string
	Background   string // #rrggbb atau URL gambar https
	CoverImage   string // URL gambar https

	// Personalisasi (T20).
	MusicURL     string // lagu bawaan (R2 music/) atau unggahan wedding
	MusicEnabled bool
	QuoteText    string // ≤ 500 karakter; kosong → bagian kutipan tidak tampil
	QuoteSource  string // mis. "QS. Ar-Rum: 21"
	Greeting     string // teks sapaan di pembuka; kosong → bawaan tema
	Closing      string // kalimat penutup; kosong → bawaan tema
	// HiddenSections & SectionOrder: ID bagian dari registry theme (SectionIDs).
	HiddenSections []string
	SectionOrder   []string
}

// Tokens adalah nilai tampilan final yang dipakai CSS & komponen.
type Tokens struct {
	Primary     string // #rrggbb
	Surface     string // warna latar
	Ink         string // warna teks
	FontHeading string
	FontBody    string
	Background  string // #rrggbb atau URL gambar
	CoverImage  string

	// Token desain bawaan tema (T21) — tidak bisa diubah per wedding.
	Accent string // ornamen, garis, ikon (bukan teks isi / tombol utama)
	Deep   string // bagian gelap & penutup (teks di atasnya putih)
	Muted  string // teks sekunder di atas Surface
	Border string // garis & tepi kartu
	// Radius kartu/foto dan tombol (panjang CSS: "0", "0.75rem", "9999px").
	Radius       string
	ButtonRadius string
}

// BackgroundIsImage: latar berupa gambar (bukan warna).
func (t Tokens) BackgroundIsImage() bool {
	return len(t.Background) > 0 && t.Background[0] != '#'
}
