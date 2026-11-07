package payment

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func (f *fixture) promo(t *testing.T, in PromoInput) Promo {
	t.Helper()
	if errs := f.svc.ValidatePromo(in); len(errs) > 0 {
		t.Fatalf("validasi %+v: %v", in, errs)
	}
	p, err := f.svc.CreatePromo(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPromoQuote(t *testing.T) {
	f := newFixture(t)
	f.svc.SetPrice(240000)
	f.promo(t, PromoInput{Code: "half", Type: DiscountPercent, Value: 50, Active: true})
	f.promo(t, PromoInput{Code: "LAUNCH", Type: DiscountAmount, Value: 181000, Active: true})
	off := f.promo(t, PromoInput{Code: "MATI", Type: DiscountAmount, Value: 1000, Active: false})

	for _, c := range []struct {
		code  string
		final int64
		want  string
		err   error
	}{
		{"", 240000, "", nil},
		{" half ", 120000, "HALF", nil}, // tidak peka huruf besar/kecil & spasi
		{"launch", 59000, "LAUNCH", nil},
		{"MATI", 240000, "", ErrPromoInvalid},
		{"TIDAKADA", 240000, "", ErrPromoInvalid},
	} {
		q, err := f.svc.Quote(ctx, c.code)
		if !errors.Is(err, c.err) || q.Base != 240000 || q.Final != c.final || q.Code != c.want {
			t.Errorf("Quote(%q) = %+v, %v", c.code, q, err)
		}
	}
	if q, _ := f.svc.Quote(ctx, "LAUNCH"); !q.Discounted() || q.Promo == nil || q.Promo.Label() != "Rp181.000" {
		t.Errorf("quote LAUNCH = %+v", q)
	}

	// Tanpa default → harga dasar; default nonaktif juga → harga dasar.
	if q := f.svc.DefaultQuote(ctx); q.Discounted() || q.Final != 240000 {
		t.Errorf("tanpa default = %+v", q)
	}
	if err := f.svc.SetDefaultPromo(ctx, off.ID); err != nil {
		t.Fatal(err)
	}
	if q := f.svc.DefaultQuote(ctx); q.Discounted() {
		t.Errorf("default nonaktif = %+v", q)
	}

	// Harga dasar diturunkan admin sampai potongan nominal kebablasan → tidak berlaku.
	f.svc.SetPrice(150000)
	if _, err := f.svc.Quote(ctx, "LAUNCH"); !errors.Is(err, ErrPromoTooLow) {
		t.Errorf("potongan melebihi harga: %v", err)
	}
	f.svc.SetPrice(0) // diabaikan
	if f.svc.Price() != 150000 {
		t.Errorf("SetPrice(0) mengubah harga: %d", f.svc.Price())
	}
}

func TestPromoAdmin(t *testing.T) {
	f := newFixture(t) // harga dasar 149.000
	for _, c := range []struct {
		in    PromoInput
		field string
	}{
		{PromoInput{Code: "", Type: DiscountPercent, Value: 10}, "code"},
		{PromoInput{Code: "ada spasi", Type: DiscountPercent, Value: 10}, "code"},
		{PromoInput{Code: "OK", Type: DiscountPercent, Value: 100}, "value"},
		{PromoInput{Code: "OK", Type: DiscountPercent, Value: 95}, "value"}, // 7.450 < MinAmount
		{PromoInput{Code: "OK", Type: DiscountAmount, Value: 140000}, "value"},
		{PromoInput{Code: "OK", Type: "gratis", Value: 1}, "type"},
	} {
		if errs := f.svc.ValidatePromo(c.in); errs[c.field] == "" {
			t.Errorf("ValidatePromo(%+v) = %v, want error di %s", c.in, errs, c.field)
		}
	}

	a := f.promo(t, PromoInput{Code: "A", Type: DiscountPercent, Value: 10, Active: true, IsDefault: true})
	b := f.promo(t, PromoInput{Code: "B", Type: DiscountPercent, Value: 20, Active: true, IsDefault: true})
	if _, err := f.svc.CreatePromo(ctx, PromoInput{Code: "a", Type: DiscountPercent, Value: 5, Active: true}); !errors.Is(err, ErrPromoExists) {
		t.Errorf("kode ganda: %v", err)
	}
	// Default hanya satu: B menggantikan A.
	list, _ := f.svc.ListPromos(ctx)
	if len(list) != 2 || list[0].ID != b.ID || !list[0].IsDefault || list[1].IsDefault {
		t.Errorf("daftar = %+v", list)
	}
	if err := f.svc.SetDefaultPromo(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if q := f.svc.DefaultQuote(ctx); q.Code != "A" || q.Final != 134100 {
		t.Errorf("default A = %+v", q)
	}
	if err := f.svc.SetDefaultPromo(ctx, uuid.Nil); err != nil {
		t.Fatal(err)
	}
	if q := f.svc.DefaultQuote(ctx); q.Discounted() {
		t.Errorf("tanpa default = %+v", q)
	}
	if p, err := f.svc.SetPromoActive(ctx, a.ID, false); err != nil || p.Active {
		t.Errorf("nonaktifkan: %+v %v", p, err)
	}
	if _, err := f.svc.DeletePromo(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.DeletePromo(ctx, b.ID); !errors.Is(err, ErrPromoNotFound) {
		t.Errorf("hapus dua kali: %v", err)
	}
	if err := f.svc.SetDefaultPromo(ctx, b.ID); !errors.Is(err, ErrPromoNotFound) {
		t.Errorf("default untuk kode terhapus: %v", err)
	}
}

func TestCreateOrderWithPromo(t *testing.T) {
	f := newFixture(t)
	f.svc.SetPrice(240000)
	f.promo(t, PromoInput{Code: "LAUNCH", Type: DiscountAmount, Value: 181000, Active: true})
	f.promo(t, PromoInput{Code: "HALF", Type: DiscountPercent, Value: 50, Active: true})
	user, w := f.newWedding(t, "a@example.com")
	c := Customer{Name: "Sarah"}

	if _, err := f.svc.CreateOrder(ctx, w, user, c, "SALAH"); !errors.Is(err, ErrPromoInvalid) {
		t.Fatalf("kode salah: %v", err)
	}
	if n := f.count(t, `SELECT count(*) FROM payment_orders`); n != 0 {
		t.Fatalf("kode salah tidak boleh membuat order (%d)", n)
	}
	o, err := f.svc.CreateOrder(ctx, w, user, c, "launch")
	if err != nil || o.Amount != 59000 || o.OriginalAmount != 240000 || o.PromoCode != "LAUNCH" {
		t.Fatalf("order promo: %+v %v", o, err)
	}
	if n, _, _ := f.fake.FetchStatus(ctx, o.Number); n.Amount != 59000 {
		t.Errorf("nominal di gateway = %d, want 59000", n.Amount)
	}
	// Kode sama → order dipakai ulang; kode lain → order lama ditutup.
	if again, _ := f.svc.CreateOrder(ctx, w, user, c, "LAUNCH"); again.ID != o.ID {
		t.Error("kode sama harus memakai order yang sama")
	}
	o2, err := f.svc.CreateOrder(ctx, w, user, c, "HALF")
	if err != nil || o2.ID == o.ID || o2.Amount != 120000 {
		t.Fatalf("ganti kode: %+v %v", o2, err)
	}
	if old, _ := f.svc.OrderByNumber(ctx, o.Number); old.Status != StatusCancelled {
		t.Errorf("order lama = %s, want cancelled", old.Status)
	}
	// Ganti metode dengan kode salah: order pending tidak ikut ditutup.
	if _, err := f.svc.ChangeMethod(ctx, w, user, c, "SALAH"); !errors.Is(err, ErrPromoInvalid) {
		t.Errorf("ganti metode kode salah: %v", err)
	}
	if cur, _ := f.svc.OrderByNumber(ctx, o2.Number); cur.Status != StatusPending {
		t.Errorf("order pending = %s setelah kode salah", cur.Status)
	}
	// Webhook dengan nominal harga normal ditolak; nominal promo diterima.
	if _, err := f.svc.HandleNotification(ctx, f.fake.Notify(o2.Number, StatusPaid, 240000, f.now)); err == nil {
		t.Error("nominal harga normal harus ditolak")
	}
	if out, err := f.svc.HandleNotification(ctx, f.fake.Notify(o2.Number, StatusPaid, 120000, f.now)); err != nil || out != OutcomeApplied {
		t.Fatalf("bayar promo: %q %v", out, err)
	}
	if !f.wedding(t, w.ID).IsPaid() {
		t.Error("pembayaran promo harus membuka publikasi")
	}
}

func TestPublishPagePromoViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newServer(t, f)
	f.svc.SetPrice(240000)
	f.promo(t, PromoInput{Code: "LAUNCH", Type: DiscountAmount, Value: 181000, Active: true, IsDefault: true})
	f.promo(t, PromoInput{Code: "HALF", Type: DiscountPercent, Value: 50, Active: true})
	owner, w := f.newWedding(t, "a@example.com")
	base := w.DashboardURL("")

	// Promo default terisi otomatis: harga normal dicoret + harga promo.
	page := do(e, owner, http.MethodGet, base+"/publish", nil, nil).Body.String()
	for _, want := range []string{"line-through", "Rp240.000", "Rp59.000", `name="promo" value="LAUNCH"`, "Promo LAUNCH · hemat Rp181.000", `type="hidden" name="promo" value="LAUNCH"`} {
		if !strings.Contains(page, want) {
			t.Errorf("halaman publikasi tidak memuat %q", want)
		}
	}
	// Terapkan kode lain / kode salah / kosongkan.
	if page := do(e, owner, http.MethodGet, base+"/publish?promo=half", nil, nil).Body.String(); !strings.Contains(page, "Rp120.000") || !strings.Contains(page, `type="hidden" name="promo" value="HALF"`) {
		t.Error("?promo=half")
	}
	page = do(e, owner, http.MethodGet, base+"/publish?promo=salah", nil, nil).Body.String()
	if !strings.Contains(page, "Kode promo SALAH tidak berlaku.") || strings.Contains(page, "line-through") || !strings.Contains(page, `type="hidden" name="promo" value=""`) {
		t.Error("?promo=salah harus menampilkan pesan dan harga normal")
	}
	if page := do(e, owner, http.MethodGet, base+"/publish?promo=", nil, nil).Body.String(); strings.Contains(page, "line-through") || !strings.Contains(page, "Rp240.000") {
		t.Error("?promo= (dikosongkan) harus harga normal")
	}

	// Bayar dengan kode → order berharga promo; halaman publikasi lalu memakai
	// kode order yang masih pending.
	rec := do(e, owner, http.MethodPost, base+"/payment", url.Values{"promo": {"HALF"}}, nil)
	orders, _ := f.svc.ListOrders(ctx, w.ID)
	if rec.Code != http.StatusSeeOther || len(orders) != 1 || orders[0].Amount != 120000 || orders[0].PromoCode != "HALF" {
		t.Fatalf("bayar dengan promo: %d %+v", rec.Code, orders)
	}
	if page := do(e, owner, http.MethodGet, base+"/publish", nil, nil).Body.String(); !strings.Contains(page, "promo HALF") || !strings.Contains(page, "Rp120.000") {
		t.Error("halaman publikasi saat order pending harus memakai kode order itu")
	}
	// Kode yang tidak berlaku saat bayar → 422, tanpa order baru.
	if rec := do(e, owner, http.MethodPost, base+"/payment/change", url.Values{"promo": {"SALAH"}}, nil); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "tidak berlaku") {
		t.Errorf("bayar kode salah: %d", rec.Code)
	}
	if orders, _ := f.svc.ListOrders(ctx, w.ID); len(orders) != 1 || orders[0].Status != StatusPending {
		t.Errorf("order setelah kode salah = %+v", orders)
	}
}
