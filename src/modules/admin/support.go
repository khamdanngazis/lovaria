package admin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/platform/web"

	admindb "github.com/khamdanngazis/lovaria/src/modules/admin/db"
)

// Kontak bantuan pelanggan lewat WhatsApp (T25): nomor & pesan pembuka bisa
// diganti admin kapan saja tanpa deploy.

const (
	settingSupportPhone   = "support_whatsapp"
	settingSupportMessage = "support_message"
	settingContactEmail   = "contact_email"
	settingContactTel     = "contact_phone"
	settingContactAddress = "business_address"
	maxAddress            = 300
	// DefaultSupportMessage: pesan pembuka bila admin mengosongkannya.
	DefaultSupportMessage = "Halo Lunovia, saya butuh bantuan."
	maxSupportMessage     = 300
)

// Support membaca kontak bantuan tersimpan (Phone kosong = belum diatur).
func (s *Service) Support(ctx context.Context) (web.Support, error) {
	rows, err := s.q.ListSettings(ctx)
	if err != nil {
		return web.Support{}, fmt.Errorf("admin: baca setelan: %w", err)
	}
	out := web.Support{Message: DefaultSupportMessage}
	for _, r := range rows {
		switch r.Key {
		case settingSupportPhone:
			out.Phone = r.Value
		case settingSupportMessage:
			if r.Value != "" {
				out.Message = r.Value
			}
		case settingContactEmail:
			out.Email = r.Value
		case settingContactTel:
			out.Tel = r.Value
		case settingContactAddress:
			out.Address = r.Value
		}
	}
	return out, nil
}

// LoadSupport memuat kontak bantuan ke memori; dipanggil sekali saat start.
func (s *Service) LoadSupport(ctx context.Context) error {
	sup, err := s.Support(ctx)
	if err != nil {
		return err
	}
	web.SetSupport(sup)
	return nil
}

// SupportInput: isian halaman Kontak & bantuan.
type SupportInput struct {
	WhatsApp, Message, Email, Tel, Address string
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// SaveSupport menyimpan kontak bantuan & kontak usaha, lalu langsung
// memberlakukannya. Kolom kosong = tidak ditampilkan.
func (s *Service) SaveSupport(ctx context.Context, in SupportInput) error {
	v := ValidationError{}
	wa, err := guest.NormalizePhone(in.WhatsApp)
	if err != nil {
		v["phone"] = "Nomor WhatsApp tidak valid (contoh: 0812-3456-7890 atau +62 812 3456 7890)"
	}
	tel, err := guest.NormalizePhone(in.Tel)
	if err != nil {
		v["tel"] = "Nomor telepon tidak valid (contoh: 0812-3456-7890 atau +62 21 1234 5678)"
	}
	message := strings.TrimSpace(in.Message)
	if utf8.RuneCountInString(message) > maxSupportMessage {
		v["message"] = fmt.Sprintf("Pesan pembuka maksimal %d karakter", maxSupportMessage)
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email != "" && (!emailRe.MatchString(email) || len(email) > 254) {
		v["email"] = "Email tidak valid"
	}
	address := strings.TrimSpace(in.Address)
	if utf8.RuneCountInString(address) > maxAddress {
		v["address"] = fmt.Sprintf("Alamat maksimal %d karakter", maxAddress)
	}
	if len(v) > 0 {
		return v
	}
	now := s.now()
	for k, val := range map[string]string{
		settingSupportPhone: wa, settingSupportMessage: message,
		settingContactEmail: email, settingContactTel: tel, settingContactAddress: address,
	} {
		if err := s.q.UpsertSetting(ctx, admindb.UpsertSettingParams{Key: k, Value: val, UpdatedAt: now}); err != nil {
			return fmt.Errorf("admin: simpan setelan: %w", err)
		}
	}
	if err := s.audit(ctx, "support.update", "setting", settingSupportPhone, map[string]string{"whatsapp": wa, "email": email, "phone": tel}); err != nil {
		return err
	}
	return s.LoadSupport(ctx)
}

// displayPhone: nomor tersimpan ("628…") dalam bentuk yang enak dibaca di form.
func displayPhone(n string) string {
	if n == "" {
		return ""
	}
	return "+" + n
}
