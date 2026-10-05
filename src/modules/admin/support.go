package admin

import (
	"context"
	"fmt"
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

// SaveSupport menyimpan nomor WhatsApp bantuan & pesan pembuka, lalu langsung
// memberlakukannya. Nomor kosong = tautan bantuan disembunyikan.
func (s *Service) SaveSupport(ctx context.Context, phone, message string) error {
	n, err := guest.NormalizePhone(phone)
	if err != nil {
		return ValidationError{"phone": "Nomor WhatsApp tidak valid (contoh: 0812-3456-7890 atau +62 812 3456 7890)"}
	}
	message = strings.TrimSpace(message)
	if utf8.RuneCountInString(message) > maxSupportMessage {
		return ValidationError{"message": fmt.Sprintf("Pesan pembuka maksimal %d karakter", maxSupportMessage)}
	}
	now := s.now()
	for k, v := range map[string]string{settingSupportPhone: n, settingSupportMessage: message} {
		if err := s.q.UpsertSetting(ctx, admindb.UpsertSettingParams{Key: k, Value: v, UpdatedAt: now}); err != nil {
			return fmt.Errorf("admin: simpan setelan: %w", err)
		}
	}
	if err := s.audit(ctx, "support.update", "setting", settingSupportPhone, map[string]string{"phone": n}); err != nil {
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
