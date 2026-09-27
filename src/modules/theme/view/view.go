// Package view adalah kontrak data antara penyusun halaman undangan (public
// site, T09) dan tema. Tema hanya boleh membaca View; tidak memanggil service.
package view

import "time"

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

	// Preview: tampilkan banner "Preview" (owner melihat draft / dashboard).
	Preview bool
	// Sample: data contoh dipakai untuk bagian yang masih kosong (preview dashboard).
	Sample bool
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
}

// BackgroundIsImage: latar berupa gambar (bukan warna).
func (t Tokens) BackgroundIsImage() bool {
	return len(t.Background) > 0 && t.Background[0] != '#'
}
