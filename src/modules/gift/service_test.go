package gift

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

type fixture struct {
	svc      *Service
	weddings *wedding.Service
	auth     *auth.Service
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return fixture{
		svc:      NewService(pool),
		weddings: wedding.NewService(wedding.NewRepository(pool)),
		auth:     auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
	}
}

func (f fixture) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "U", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "A", BrideName: "B", Title: "T", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func bank(provider, number, name string) Input {
	return Input{Type: TypeBank, Provider: provider, AccountNumber: number, AccountName: name}
}

func TestValidation(t *testing.T) {
	cases := map[string]struct {
		in     Input
		fields []string
	}{
		"jenis":          {Input{Type: "crypto"}, []string{"type"}},
		"bank kosong":    {Input{Type: TypeBank}, []string{"provider", "account_number", "account_name"}},
		"nomor huruf":    {bank("BCA", "12ab3456", "Budi"), []string{"account_number"}},
		"nomor pendek":   {bank("BCA", "123", "Budi"), []string{"account_number"}},
		"ewallet kosong": {Input{Type: TypeEwallet, AccountNumber: "0812345678", AccountName: "Budi"}, []string{"provider"}},
		"alamat kosong":  {Input{Type: TypeAddress}, []string{"address"}},
	}
	for name, c := range cases {
		_, err := validate(c.in)
		var v ValidationError
		if !errors.As(err, &v) {
			t.Errorf("%s: %v", name, err)
			continue
		}
		for _, f := range c.fields {
			if v[f] == "" {
				t.Errorf("%s: field %s harus error (%v)", name, f, v)
			}
		}
	}
	// Kolom yang tidak relevan dibuang; spasi dirapikan.
	p, err := validate(Input{Type: TypeBank, Provider: " BCA ", AccountNumber: " 123  456 7890 ", AccountName: "Budi", Address: "Jl. X"})
	if err != nil || p.Address != "" || p.AccountNumber != "123 456 7890" || p.Provider != "BCA" {
		t.Errorf("bank = %+v %v", p, err)
	}
	p, err = validate(Input{Type: TypeAddress, Provider: "BCA", AccountNumber: "123", Address: "Jl. Mawar 1"})
	if err != nil || p.Provider != "" || p.AccountNumber != "" {
		t.Errorf("alamat = %+v %v", p, err)
	}
	if _, err := validate(Input{Type: TypeEwallet, Provider: "GoPay", AccountNumber: "+62 812-3456-7890", AccountName: "Budi"}); err != nil {
		t.Errorf("e-wallet dengan +62: %v", err)
	}
}

func TestCRUDAndOrder(t *testing.T) {
	f := newFixture(t)
	_, w := f.newWedding(t, "a@example.com")
	a1, err := f.svc.Create(ctx, w.ID, bank("BCA", "1234567890", "Budi"))
	if err != nil {
		t.Fatal(err)
	}
	a2, _ := f.svc.Create(ctx, w.ID, Input{Type: TypeEwallet, Provider: "GoPay", AccountNumber: "081234567890", AccountName: "Sari"})
	a3, _ := f.svc.Create(ctx, w.ID, Input{Type: TypeAddress, AccountName: "Sari", Address: "Jl. Mawar 1\nJakarta"})

	order := func() []uuid.UUID {
		as, _ := f.svc.List(ctx, w.ID)
		out := make([]uuid.UUID, len(as))
		for i, a := range as {
			out[i] = a.ID
		}
		return out
	}
	if got := order(); len(got) != 3 || got[0] != a1.ID || got[2] != a3.ID {
		t.Fatalf("urutan awal %v", got)
	}
	if err := f.svc.Move(ctx, w.ID, a3.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := order(); got[1] != a3.ID || got[2] != a2.ID {
		t.Errorf("setelah naik %v", got)
	}
	up, err := f.svc.Update(ctx, w.ID, a1.ID, bank("Mandiri", "999 888 777", "Budi S"))
	if err != nil || up.Provider != "Mandiri" || up.AccountNumber != "999 888 777" {
		t.Errorf("update %+v %v", up, err)
	}
	if err := f.svc.Delete(ctx, w.ID, a2.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(); len(got) != 2 {
		t.Errorf("setelah hapus %v", got)
	}
}

func TestTenantIsolation(t *testing.T) {
	f := newFixture(t)
	_, a := f.newWedding(t, "a@example.com")
	_, b := f.newWedding(t, "b@example.com")
	acc, _ := f.svc.Create(ctx, a.ID, bank("BCA", "1234567890", "Budi"))

	if as, _ := f.svc.List(ctx, b.ID); len(as) != 0 {
		t.Errorf("list b = %d", len(as))
	}
	if _, err := f.svc.Get(ctx, b.ID, acc.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := f.svc.Update(ctx, b.ID, acc.ID, bank("X", "1111111", "Y")); !errors.Is(err, ErrNotFound) {
		t.Errorf("update: %v", err)
	}
	if err := f.svc.Move(ctx, b.ID, acc.ID, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("move: %v", err)
	}
	if err := f.svc.Delete(ctx, b.ID, acc.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete: %v", err)
	}
	if got, _ := f.svc.Get(ctx, a.ID, acc.ID); got.Provider != "BCA" {
		t.Error("akun a berubah")
	}
}
