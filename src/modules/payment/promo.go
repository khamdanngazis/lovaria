package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	paymentdb "github.com/khamdanngazis/lovaria/src/modules/payment/db"
	"github.com/khamdanngazis/lovaria/src/platform/db"
)

// T39: harga dasar yang bisa diganti admin + kode promo (persen / nominal).

// MinAmount: harga akhir paling rendah setelah potongan. Promo gratis tidak
// didukung — semua pembayaran lewat gateway, dan VA sebagian gateway menolak
// nominal di bawah Rp10.000.
const MinAmount int64 = 10000

// Jenis potongan kode promo.
const (
	DiscountPercent = "percent"
	DiscountAmount  = "amount"
)

var (
	// ErrPromoInvalid: kode tidak dikenal atau sedang nonaktif.
	ErrPromoInvalid = errors.New("payment: kode promo tidak berlaku")
	// ErrPromoTooLow: potongan membuat harga di bawah MinAmount.
	ErrPromoTooLow = errors.New("payment: potongan promo melebihi batas")
	// ErrPromoNotFound: id promo tidak ada (panel admin).
	ErrPromoNotFound = errors.New("payment: kode promo tidak ditemukan")
	// ErrPromoExists: kode sudah dipakai promo lain.
	ErrPromoExists = errors.New("payment: kode promo sudah ada")
)

// Promo adalah satu kode promo.
type Promo struct {
	ID        uuid.UUID
	Code      string
	Type      string // DiscountPercent | DiscountAmount
	Value     int64  // persen (1–99) atau rupiah
	Active    bool
	IsDefault bool
	CreatedAt time.Time
}

// Apply: harga setelah potongan dari harga dasar base.
func (p Promo) Apply(base int64) int64 {
	if p.Type == DiscountPercent {
		return base - base*p.Value/100
	}
	return base - p.Value
}

// Label: "75%" atau "Rp181.000".
func (p Promo) Label() string {
	if p.Type == DiscountPercent {
		return strconv.FormatInt(p.Value, 10) + "%"
	}
	return Rupiah(p.Value)
}

func toPromo(r paymentdb.PromoCode) Promo {
	return Promo{ID: r.ID, Code: r.Code, Type: r.DiscountType, Value: r.DiscountValue, Active: r.Active, IsDefault: r.IsDefault, CreatedAt: r.CreatedAt}
}

// NormalizeCode: kode promo tidak peka huruf besar/kecil dan spasi tepi.
func NormalizeCode(code string) string { return strings.ToUpper(strings.TrimSpace(code)) }

// Quote adalah harga untuk satu pembayaran.
type Quote struct {
	Base  int64  // harga dasar
	Final int64  // harga yang ditagih
	Code  string // kode promo yang dipakai ("" = tanpa promo)
	Promo *Promo
}

// Discounted: ada potongan.
func (q Quote) Discounted() bool { return q.Final < q.Base }

// Price: harga dasar publikasi per wedding (rupiah).
func (s *Service) Price() int64 { return s.price.Load() }

// SetPrice mengganti harga dasar (panel admin). Nilai < 1 diabaikan.
func (s *Service) SetPrice(p int64) {
	if p >= 1 {
		s.price.Store(p)
	}
}

// Quote menghitung harga dengan kode promo (kosong = harga dasar).
func (s *Service) Quote(ctx context.Context, code string) (Quote, error) {
	base := s.Price()
	code = NormalizeCode(code)
	if code == "" {
		return Quote{Base: base, Final: base}, nil
	}
	row, err := s.q.GetPromoByCode(ctx, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return Quote{Base: base, Final: base}, ErrPromoInvalid
	}
	if err != nil {
		return Quote{}, err
	}
	return quoteFor(base, toPromo(row))
}

func quoteFor(base int64, p Promo) (Quote, error) {
	q := Quote{Base: base, Final: base}
	if !p.Active {
		return q, ErrPromoInvalid
	}
	final := p.Apply(base)
	if final < MinAmount || final < 1 {
		return q, ErrPromoTooLow
	}
	return Quote{Base: base, Final: final, Code: p.Code, Promo: &p}, nil
}

// DefaultQuote: harga dengan promo default (landing & isian awal halaman
// publikasi). Tanpa promo default yang berlaku → harga dasar.
func (s *Service) DefaultQuote(ctx context.Context) Quote {
	base := s.Price()
	row, err := s.q.GetDefaultPromo(ctx)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			s.log.ErrorContext(ctx, "payment: baca promo default", slog.String("error", err.Error()))
		}
		return Quote{Base: base, Final: base}
	}
	q, err := quoteFor(base, toPromo(row))
	if err != nil {
		return Quote{Base: base, Final: base}
	}
	return q
}

// ---------- Panel admin ----------

// PromoInput: isian form tambah promo.
type PromoInput struct {
	Code      string
	Type      string
	Value     int64
	Active    bool
	IsDefault bool
}

// ValidatePromo memeriksa isian terhadap harga dasar saat ini. Kunci map =
// nama field form.
func (s *Service) ValidatePromo(in PromoInput) map[string]string {
	errs := map[string]string{}
	code := NormalizeCode(in.Code)
	switch {
	case code == "":
		errs["code"] = "Kode wajib diisi."
	case len(code) > 32:
		errs["code"] = "Kode paling panjang 32 karakter."
	default:
		for _, r := range code {
			if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' {
				errs["code"] = "Kode hanya boleh huruf, angka, - dan _."
				break
			}
		}
	}
	switch in.Type {
	case DiscountPercent:
		if in.Value < 1 || in.Value > 99 {
			errs["value"] = "Persen potongan 1–99."
		}
	case DiscountAmount:
		if in.Value < 1 {
			errs["value"] = "Potongan minimal Rp1."
		}
	default:
		errs["type"] = "Pilih jenis potongan."
	}
	if _, ok := errs["value"]; !ok && errs["type"] == "" {
		if final := (Promo{Type: in.Type, Value: in.Value}).Apply(s.Price()); final < MinAmount {
			errs["value"] = fmt.Sprintf("Harga setelah potongan (%s) di bawah batas minimal %s.", Rupiah(max(final, 0)), Rupiah(MinAmount))
		}
	}
	return errs
}

// ListPromos: semua kode promo (default lebih dulu).
func (s *Service) ListPromos(ctx context.Context) ([]Promo, error) {
	rows, err := s.q.ListPromos(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Promo, len(rows))
	for i, r := range rows {
		out[i] = toPromo(r)
	}
	return out, nil
}

// CreatePromo menyimpan kode baru (sudah divalidasi lewat ValidatePromo).
func (s *Service) CreatePromo(ctx context.Context, in PromoInput) (Promo, error) {
	var p Promo
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		row, err := q.InsertPromo(ctx, paymentdb.InsertPromoParams{
			ID: db.NewID(), Code: NormalizeCode(in.Code), DiscountType: in.Type, DiscountValue: in.Value, Active: in.Active,
		})
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrPromoExists
		}
		if err != nil {
			return err
		}
		if in.IsDefault {
			if err := q.ClearDefaultPromo(ctx); err != nil {
				return err
			}
			if row, err = q.SetDefaultPromo(ctx, row.ID); err != nil {
				return err
			}
		}
		p = toPromo(row)
		return nil
	})
	return p, err
}

// SetPromoActive menyalakan / mematikan kode.
func (s *Service) SetPromoActive(ctx context.Context, id uuid.UUID, active bool) (Promo, error) {
	row, err := s.q.SetPromoActive(ctx, paymentdb.SetPromoActiveParams{ID: id, Active: active})
	if errors.Is(err, pgx.ErrNoRows) {
		return Promo{}, ErrPromoNotFound
	}
	return toPromo(row), err
}

// SetDefaultPromo menjadikan id satu-satunya promo default; uuid.Nil =
// tanpa promo default.
func (s *Service) SetDefaultPromo(ctx context.Context, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		if err := q.ClearDefaultPromo(ctx); err != nil {
			return err
		}
		if id == uuid.Nil {
			return nil
		}
		if _, err := q.SetDefaultPromo(ctx, id); errors.Is(err, pgx.ErrNoRows) {
			return ErrPromoNotFound
		} else if err != nil {
			return err
		}
		return nil
	})
}

// DeletePromo menghapus kode. Order lama tetap mencatat kodenya sebagai teks.
func (s *Service) DeletePromo(ctx context.Context, id uuid.UUID) (Promo, error) {
	row, err := s.q.DeletePromo(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Promo{}, ErrPromoNotFound
	}
	return toPromo(row), err
}
