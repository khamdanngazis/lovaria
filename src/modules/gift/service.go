// Package gift: amplop digital — rekening bank, e-wallet, dan alamat kirim
// hadiah (Produk §13). Operasi menerima weddingID yang sudah diotorisasi
// (RequireWeddingOwner) atau hasil resolver public site.
package gift

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/order"

	giftdb "github.com/khamdanngazis/lovaria/src/modules/gift/db"
)

const (
	TypeBank    = "bank"
	TypeEwallet = "ewallet"
	TypeAddress = "address"
)

// Types: jenis akun beserta labelnya.
var Types = []struct{ ID, Label string }{
	{TypeBank, "Rekening bank"},
	{TypeEwallet, "E-wallet"},
	{TypeAddress, "Alamat kirim hadiah"},
}

func TypeLabel(t string) string {
	for _, x := range Types {
		if x.ID == t {
			return x.Label
		}
	}
	return t
}

// Saran penyedia (input tetap bebas).
var (
	Banks    = []string{"BCA", "BRI", "BNI", "Mandiri", "BSI", "CIMB Niaga", "Permata", "Danamon", "BTN", "Bank Jago", "SeaBank", "blu by BCA Digital"}
	Ewallets = []string{"GoPay", "OVO", "DANA", "ShopeePay", "LinkAja"}
)

var ErrNotFound = errors.New("akun hadiah tidak ditemukan")

type ValidationError map[string]string

func (v ValidationError) Error() string {
	parts := make([]string, 0, len(v))
	for f, m := range v {
		parts = append(parts, f+": "+m)
	}
	return "validasi gagal: " + strings.Join(parts, "; ")
}

type Account struct {
	ID            uuid.UUID
	WeddingID     uuid.UUID
	Type          string
	Provider      string
	AccountNumber string
	AccountName   string
	Address       string
	SortOrder     int
}

// Input adalah nilai mentah form.
type Input struct {
	Type, Provider, AccountNumber, AccountName, Address string
}

func toAccount(r giftdb.GiftAccount) Account {
	return Account{
		ID: r.ID, WeddingID: r.WeddingID, Type: r.Type, Provider: r.Provider, AccountNumber: r.AccountNumber,
		AccountName: r.AccountName, Address: r.AddressText, SortOrder: int(r.SortOrder),
	}
}

func maxLen(v ValidationError, field, s string, n int, label string) {
	if utf8.RuneCountInString(s) > n {
		v[field] = fmt.Sprintf("%s maksimal %d karakter", label, n)
	}
}

// validate merapikan input; kolom yang tidak relevan untuk jenisnya dikosongkan.
func validate(in Input) (Input, error) {
	p := Input{
		Type: strings.TrimSpace(in.Type), Provider: strings.TrimSpace(in.Provider),
		AccountNumber: strings.Join(strings.Fields(in.AccountNumber), " "),
		AccountName:   strings.TrimSpace(in.AccountName), Address: strings.TrimSpace(in.Address),
	}
	v := ValidationError{}
	switch p.Type {
	case TypeBank, TypeEwallet:
		p.Address = ""
		if p.Provider == "" {
			v["provider"] = map[string]string{TypeBank: "Nama bank wajib diisi", TypeEwallet: "Nama e-wallet wajib diisi"}[p.Type]
		}
		maxLen(v, "provider", p.Provider, 50, "Nama")
		digits := 0
		for _, r := range p.AccountNumber {
			switch {
			case r >= '0' && r <= '9':
				digits++
			case r == ' ' || r == '-' || r == '+' || r == '.':
			default:
				v["account_number"] = "Nomor hanya boleh berisi angka"
			}
		}
		if v["account_number"] == "" && (digits < 5 || len(p.AccountNumber) > 40) {
			v["account_number"] = "Nomor rekening / HP tidak valid"
		}
		if p.AccountName == "" {
			v["account_name"] = "Nama pemilik wajib diisi"
		}
	case TypeAddress:
		p.Provider, p.AccountNumber = "", ""
		if p.Address == "" {
			v["address"] = "Alamat wajib diisi"
		}
		maxLen(v, "address", p.Address, 500, "Alamat")
	default:
		v["type"] = "Pilih jenis"
	}
	maxLen(v, "account_name", p.AccountName, 100, "Nama")
	if len(v) > 0 {
		return Input{}, v
	}
	return p, nil
}

type Service struct {
	pool *pgxpool.Pool
	q    *giftdb.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: giftdb.New(pool)}
}

func (s *Service) Create(ctx context.Context, weddingID uuid.UUID, in Input) (Account, error) {
	p, err := validate(in)
	if err != nil {
		return Account{}, err
	}
	row, err := s.q.CreateAccount(ctx, giftdb.CreateAccountParams{
		ID: db.NewID(), WeddingID: weddingID, Type: p.Type, Provider: p.Provider,
		AccountNumber: p.AccountNumber, AccountName: p.AccountName, AddressText: p.Address,
	})
	if err != nil {
		return Account{}, fmt.Errorf("gift: create: %w", err)
	}
	return toAccount(row), nil
}

func (s *Service) Update(ctx context.Context, weddingID, id uuid.UUID, in Input) (Account, error) {
	p, err := validate(in)
	if err != nil {
		return Account{}, err
	}
	row, err := s.q.UpdateAccount(ctx, giftdb.UpdateAccountParams{
		ID: id, WeddingID: weddingID, Type: p.Type, Provider: p.Provider,
		AccountNumber: p.AccountNumber, AccountName: p.AccountName, AddressText: p.Address,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("gift: update: %w", err)
	}
	return toAccount(row), nil
}

func (s *Service) Delete(ctx context.Context, weddingID, id uuid.UUID) error {
	n, err := s.q.DeleteAccount(ctx, giftdb.DeleteAccountParams{ID: id, WeddingID: weddingID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Get(ctx context.Context, weddingID, id uuid.UUID) (Account, error) {
	row, err := s.q.GetAccount(ctx, giftdb.GetAccountParams{ID: id, WeddingID: weddingID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	return toAccount(row), nil
}

// List mengembalikan akun sesuai urutan tampil (dipakai public site).
func (s *Service) List(ctx context.Context, weddingID uuid.UUID) ([]Account, error) {
	rows, err := s.q.ListAccounts(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	out := make([]Account, len(rows))
	for i, r := range rows {
		out[i] = toAccount(r)
	}
	return out, nil
}

// Move menggeser akun satu posisi ke atas/bawah.
func (s *Service) Move(ctx context.Context, weddingID, id uuid.UUID, up bool) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		rows, err := q.ListAccountsForUpdate(ctx, weddingID)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(rows))
		found := false
		for i, r := range rows {
			ids[i] = r.ID
			found = found || r.ID == id
		}
		if !found {
			return ErrNotFound
		}
		next, changed := order.Move(ids, id, up)
		if !changed {
			return nil
		}
		for i, id := range next {
			if err := q.SetAccountSortOrder(ctx, giftdb.SetAccountSortOrderParams{ID: id, WeddingID: weddingID, SortOrder: int32(i)}); err != nil { //nolint:gosec // G115: jumlah akun kecil
				return err
			}
		}
		return nil
	})
}
