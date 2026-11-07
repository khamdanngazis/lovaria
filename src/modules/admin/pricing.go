package admin

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	admindb "github.com/khamdanngazis/lovaria/src/modules/admin/db"
	"github.com/khamdanngazis/lovaria/src/modules/payment"
)

// Harga & promo (T39): harga dasar publikasi disimpan di app_settings
// (pengganti PUBLISH_PRICE_IDR tanpa deploy); kode promo milik modul payment
// dan diubah lewat service-nya.

const settingPublishPrice = "publish_price"

// maxPrice: batas atas harga dasar (salah ketik nol berlebih).
const maxPrice int64 = 100_000_000

// errNoPayments: panel admin dipasang tanpa service pembayaran.
var errNoPayments = errors.New("admin: pembayaran belum dipasang")

// LoadPricing memberlakukan harga dasar tersimpan; dipanggil sekali saat
// start. Tanpa setelan → harga dari env (PUBLISH_PRICE_IDR) tetap dipakai.
func (s *Service) LoadPricing(ctx context.Context) error {
	if s.Payments == nil {
		return nil
	}
	rows, err := s.q.ListSettings(ctx)
	if err != nil {
		return fmt.Errorf("admin: baca setelan: %w", err)
	}
	for _, r := range rows {
		if r.Key != settingPublishPrice {
			continue
		}
		p, err := strconv.ParseInt(r.Value, 10, 64)
		if err != nil || p < payment.MinAmount {
			return fmt.Errorf("admin: harga tersimpan tidak valid: %q", r.Value)
		}
		s.Payments.SetPrice(p)
	}
	return nil
}

// parseRupiah: "240.000", "Rp 240000", "240,000" → 240000.
func parseRupiah(v string) (int64, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(strings.ToLower(v)), "rp"))
	v = strings.NewReplacer(".", "", ",", "", " ", "").Replace(v)
	n, err := strconv.ParseInt(v, 10, 64)
	return n, err == nil
}

// SavePrice menyimpan harga dasar dan langsung memberlakukannya. Promo aktif
// yang potongannya menjadi di bawah batas minimal ditolak dulu supaya harga
// promo di landing tidak tiba-tiba hilang.
func (s *Service) SavePrice(ctx context.Context, raw string) error {
	if s.Payments == nil {
		return errNoPayments
	}
	p, ok := parseRupiah(raw)
	switch {
	case !ok:
		return ValidationError{"price": "Harga harus berupa angka rupiah, mis. 240000"}
	case p < payment.MinAmount || p > maxPrice:
		return ValidationError{"price": fmt.Sprintf("Harga %s–%s", payment.Rupiah(payment.MinAmount), payment.Rupiah(maxPrice))}
	}
	promos, err := s.Payments.ListPromos(ctx)
	if err != nil {
		return err
	}
	for _, pr := range promos {
		if pr.Active && pr.Apply(p) < payment.MinAmount {
			return ValidationError{"price": fmt.Sprintf("Potongan promo aktif %s (%s) membuat harga di bawah %s. Nonaktifkan atau hapus promo itu dulu.", pr.Code, pr.Label(), payment.Rupiah(payment.MinAmount))}
		}
	}
	old := s.Payments.Price()
	if err := s.q.UpsertSetting(ctx, admindb.UpsertSettingParams{Key: settingPublishPrice, Value: strconv.FormatInt(p, 10), UpdatedAt: s.now()}); err != nil {
		return fmt.Errorf("admin: simpan harga: %w", err)
	}
	if err := s.audit(ctx, "pricing.update", "setting", settingPublishPrice, map[string]string{"from": strconv.FormatInt(old, 10), "to": strconv.FormatInt(p, 10)}); err != nil {
		return err
	}
	s.Payments.SetPrice(p)
	return nil
}

// Pricing: isi halaman Harga & promo.
type Pricing struct {
	Enabled bool // service pembayaran terpasang
	Price   int64
	Default payment.Quote
	Promos  []payment.Promo
}

func (s *Service) Pricing(ctx context.Context) (Pricing, error) {
	if s.Payments == nil {
		return Pricing{}, nil
	}
	promos, err := s.Payments.ListPromos(ctx)
	if err != nil {
		return Pricing{}, err
	}
	return Pricing{Enabled: true, Price: s.Payments.Price(), Default: s.Payments.DefaultQuote(ctx), Promos: promos}, nil
}

// CreatePromo memvalidasi lalu menyimpan kode promo baru.
func (s *Service) CreatePromo(ctx context.Context, in payment.PromoInput, rawValue string) error {
	if s.Payments == nil {
		return errNoPayments
	}
	v, ok := parseRupiah(rawValue)
	if !ok {
		return ValidationError{"value": "Nilai potongan harus berupa angka"}
	}
	in.Value = v
	if errs := s.Payments.ValidatePromo(in); len(errs) > 0 {
		return ValidationError(errs)
	}
	p, err := s.Payments.CreatePromo(ctx, in)
	if errors.Is(err, payment.ErrPromoExists) {
		return ValidationError{"code": "Kode " + payment.NormalizeCode(in.Code) + " sudah ada"}
	}
	if err != nil {
		return err
	}
	return s.audit(ctx, "promo.create", "promo", p.ID.String(), map[string]string{
		"code": p.Code, "discount": p.Label(), "active": strconv.FormatBool(p.Active), "default": strconv.FormatBool(p.IsDefault),
	})
}

func (s *Service) promoID(raw string) (uuid.UUID, error) {
	if s.Payments == nil {
		return uuid.Nil, errNoPayments
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, ErrNotFound
	}
	return id, nil
}

func promoErr(err error) error {
	if errors.Is(err, payment.ErrPromoNotFound) {
		return ErrNotFound
	}
	return err
}

// SetPromoActive menyalakan / mematikan kode promo.
func (s *Service) SetPromoActive(ctx context.Context, rawID string, active bool) error {
	id, err := s.promoID(rawID)
	if err != nil {
		return err
	}
	p, err := s.Payments.SetPromoActive(ctx, id, active)
	if err != nil {
		return promoErr(err)
	}
	return s.audit(ctx, "promo.toggle", "promo", p.ID.String(), map[string]string{"code": p.Code, "active": strconv.FormatBool(active)})
}

// SetDefaultPromo menjadikan kode promo default (tampil di landing & terisi
// otomatis di halaman publikasi); rawID kosong = tanpa promo default.
func (s *Service) SetDefaultPromo(ctx context.Context, rawID string) error {
	if s.Payments == nil {
		return errNoPayments
	}
	id := uuid.Nil
	if rawID != "" {
		var err error
		if id, err = s.promoID(rawID); err != nil {
			return err
		}
	}
	if err := s.Payments.SetDefaultPromo(ctx, id); err != nil {
		return promoErr(err)
	}
	target := ""
	if id != uuid.Nil {
		target = id.String()
	}
	return s.audit(ctx, "promo.default", "promo", target, nil)
}

// DeletePromo menghapus kode promo (order lama tetap mencatat kodenya).
func (s *Service) DeletePromo(ctx context.Context, rawID string) error {
	id, err := s.promoID(rawID)
	if err != nil {
		return err
	}
	p, err := s.Payments.DeletePromo(ctx, id)
	if err != nil {
		return promoErr(err)
	}
	return s.audit(ctx, "promo.delete", "promo", p.ID.String(), map[string]string{"code": p.Code})
}
