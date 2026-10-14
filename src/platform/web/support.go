package web

import (
	"net/url"
	"strings"
	"sync/atomic"
)

// Support: kontak bantuan pelanggan lewat WhatsApp (T25). Diatur admin di
// panel admin dan disimpan di database; nilai di sini adalah salinan di
// memori yang dibaca template di setiap halaman.
type Support struct {
	Phone   string // format internasional tanpa "+", mis. "628123456789"; kosong = belum diatur
	Message string // pesan pembuka yang terisi otomatis di WhatsApp
	// Kontak usaha (ditampilkan di halaman utama & halaman legal; syarat
	// verifikasi payment gateway): email support, nomor telepon (format
	// internasional tanpa "+"), dan alamat usaha. Kosong = tidak ditampilkan.
	Email, Tel, Address string
}

// Contact mengembalikan kontak usaha yang sedang berlaku.
func Contact() Support {
	if s := support.Load(); s != nil {
		return *s
	}
	return Support{}
}

// HasContact: minimal satu kontak usaha sudah diisi.
func (s Support) HasContact() bool {
	return s.Email != "" || s.Tel != "" || s.Address != "" || s.Phone != ""
}

// TelDisplay: nomor telepon untuk ditampilkan ("6281234567890" → "+6281234567890").
func (s Support) TelDisplay() string {
	if s.Tel == "" {
		return ""
	}
	return "+" + s.Tel
}

var support atomic.Pointer[Support]

// SetSupport mengganti kontak bantuan yang dipakai seluruh halaman.
func SetSupport(s Support) { support.Store(&s) }

// SupportURL: tautan wa.me ke nomor bantuan dengan pesan pembuka terisi;
// "" bila nomor belum diatur (tautan bantuan tidak ditampilkan).
func SupportURL() string {
	s := support.Load()
	if s == nil || s.Phone == "" {
		return ""
	}
	u := "https://wa.me/" + s.Phone
	if s.Message != "" {
		// QueryEscape menulis spasi sebagai "+", yang tidak selalu dibaca
		// WhatsApp sebagai spasi — pakai %20.
		u += "?text=" + strings.ReplaceAll(url.QueryEscape(s.Message), "+", "%20")
	}
	return u
}
