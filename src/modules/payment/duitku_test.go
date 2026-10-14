package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func hmacHex(key, msg string) string {
	m := hmac.New(sha256.New, []byte(key))
	m.Write([]byte(msg))
	return hex.EncodeToString(m.Sum(nil))
}

// fakeDuitku meniru dua endpoint Duitku: Create Invoice & Check Transaction.
type fakeDuitku struct {
	srv          *httptest.Server
	headers      http.Header
	invoice      map[string]any
	status       map[string]string
	statusCode   string
	statusHTTP   int
	statusAmount string
}

func newFakeDuitku(t *testing.T) (*fakeDuitku, *Duitku) {
	f := &fakeDuitku{statusCode: "00", statusHTTP: http.StatusOK, statusAmount: "149000"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch r.URL.Path {
		case "/invoice":
			f.headers = r.Header.Clone()
			_ = json.Unmarshal(body, &f.invoice)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"merchantCode": "DS0001", "reference": "DS000125ABCD", "statusCode": "00", "statusMessage": "SUCCESS",
				"paymentUrl": "https://app-sandbox.duitku.com/redirect_checkout?reference=DS000125ABCD",
			})
		case "/status":
			_ = json.Unmarshal(body, &f.status)
			w.WriteHeader(f.statusHTTP)
			if f.statusHTTP == http.StatusOK {
				_ = json.NewEncoder(w).Encode(map[string]string{
					"merchantOrderId": f.status["merchantOrderId"], "reference": "DS000125ABCD",
					"amount": f.statusAmount, "statusCode": f.statusCode, "statusMessage": "x",
				})
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	d := NewDuitku("DS0001", "kunci-api", "https://lunovia.test/", false)
	d.InvoiceURL, d.StatusURL, d.HTTP = f.srv.URL+"/invoice", f.srv.URL+"/status", f.srv.Client()
	d.now = func() time.Time { return time.UnixMilli(1773728479616) }
	return f, d
}

func TestDuitkuEndpoints(t *testing.T) {
	if d := NewDuitku("m", "k", "https://x", true); !strings.HasPrefix(d.InvoiceURL, "https://api-prod.duitku.com/") || !strings.HasPrefix(d.StatusURL, "https://passport.duitku.com/") {
		t.Errorf("produksi: %s %s", d.InvoiceURL, d.StatusURL)
	}
	if d := NewDuitku("m", "k", "https://x", false); !strings.HasPrefix(d.InvoiceURL, "https://api-sandbox.duitku.com/") || !strings.HasPrefix(d.StatusURL, "https://sandbox.duitku.com/") || d.Name() != "duitku" {
		t.Errorf("sandbox: %s %s", d.InvoiceURL, d.StatusURL)
	}
}

func TestDuitkuCreateCheckout(t *testing.T) {
	f, d := newFakeDuitku(t)
	got, err := d.CreateCheckout(ctx, CheckoutRequest{
		OrderNumber: "LVR-20261014-000001", Amount: 149000, ItemName: "Publikasi undangan Samuel & Sarah",
		Customer:  Customer{Name: "Samuel Pratama Wijayakusuma", Email: "samuel@example.com"},
		ExpiresIn: 24 * time.Hour, ReturnURL: "https://lunovia.test/dashboard/weddings/x/payment/return",
	})
	if err != nil || got != "https://app-sandbox.duitku.com/redirect_checkout?reference=DS000125ABCD" {
		t.Fatalf("checkout: %q %v", got, err)
	}
	// Header: kode merchant, waktu (ms), tanda tangan HMAC(merchantCode+timestamp).
	if f.headers.Get("x-duitku-merchantcode") != "DS0001" || f.headers.Get("x-duitku-timestamp") != "1773728479616" ||
		f.headers.Get("x-duitku-signature") != hmacHex("kunci-api", "DS00011773728479616") {
		t.Errorf("header: %v", f.headers)
	}
	b := f.invoice
	if b["paymentAmount"] != float64(149000) || b["merchantOrderId"] != "LVR-20261014-000001" || b["email"] != "samuel@example.com" ||
		b["callbackUrl"] != "https://lunovia.test/webhooks/duitku" || b["returnUrl"] != "https://lunovia.test/dashboard/weddings/x/payment/return" ||
		b["expiryPeriod"] != float64(1440) || len([]rune(b["customerVaName"].(string))) > 20 {
		t.Errorf("isi invoice: %v", b)
	}
	items := b["itemDetails"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["price"] != float64(149000) {
		t.Errorf("itemDetails harus berjumlah sama dengan paymentAmount: %v", items)
	}
}

func duitkuCallback(key string, v url.Values) []byte {
	v.Set("signature", hmacHex(key, v.Get("merchantCode")+v.Get("amount")+v.Get("merchantOrderId")))
	return []byte(v.Encode())
}

func TestDuitkuParseNotification(t *testing.T) {
	_, d := newFakeDuitku(t)
	base := func() url.Values {
		return url.Values{"merchantCode": {"DS0001"}, "amount": {"149000"}, "merchantOrderId": {"LVR-20261014-000001"},
			"resultCode": {"00"}, "reference": {"DS000125ABCD"}, "paymentCode": {"SP"}}
	}
	n, err := d.ParseNotification(duitkuCallback("kunci-api", base()))
	if err != nil || n.Status != StatusPaid || n.Amount != 149000 || n.OrderNumber != "LVR-20261014-000001" || n.TransactionID != "DS000125ABCD" || n.Method != "SP" {
		t.Fatalf("callback lunas: %+v %v", n, err)
	}
	failed := base()
	failed.Set("resultCode", "01")
	if n, err := d.ParseNotification(duitkuCallback("kunci-api", failed)); err != nil || n.Status != StatusFailed {
		t.Errorf("callback gagal: %+v %v", n, err)
	}
	// Tanda tangan dengan kunci lain, isi diubah setelah ditandatangani, atau
	// merchant lain → ditolak.
	if _, err := d.ParseNotification(duitkuCallback("kunci-lain", base())); !errors.Is(err, ErrBadSignature) {
		t.Errorf("kunci lain: %v", err)
	}
	tampered, _ := url.ParseQuery(string(duitkuCallback("kunci-api", base())))
	tampered.Set("amount", "1000")
	if _, err := d.ParseNotification([]byte(tampered.Encode())); !errors.Is(err, ErrBadSignature) {
		t.Errorf("nominal diubah: %v", err)
	}
	other := base()
	other.Set("merchantCode", "DS9999")
	if _, err := d.ParseNotification(duitkuCallback("kunci-api", other)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("merchant lain: %v", err)
	}
	if _, err := d.ParseNotification([]byte("bukan=form&lengkap=1")); !errors.Is(err, ErrBadNotification) {
		t.Errorf("isi tidak lengkap: %v", err)
	}
}

func TestDuitkuFetchStatus(t *testing.T) {
	f, d := newFakeDuitku(t)
	for code, want := range map[string]string{"00": StatusPaid, "01": StatusPending, "02": StatusFailed} {
		f.statusCode = code
		n, found, err := d.FetchStatus(ctx, "LVR-20261014-000001")
		if err != nil || !found || n.Status != want || n.Amount != 149000 || n.OrderNumber != "LVR-20261014-000001" {
			t.Errorf("status %s: %+v found=%v err=%v", code, n, found, err)
		}
	}
	if f.status["merchantCode"] != "DS0001" || f.status["signature"] != hmacHex("kunci-api", "DS0001LVR-20261014-000001") {
		t.Errorf("permintaan cek status: %v", f.status)
	}
	f.statusHTTP = http.StatusBadRequest // transaksi belum ada
	if _, found, err := d.FetchStatus(ctx, "LVR-X"); found || err != nil {
		t.Errorf("belum ada: found=%v err=%v", found, err)
	}
	f.statusHTTP = http.StatusBadGateway
	if _, _, err := d.FetchStatus(ctx, "LVR-X"); err == nil {
		t.Error("5xx harus error")
	}
}

// Alur penuh dengan Duitku: order → callback lunas bertanda tangan → status
// dikonfirmasi ulang ke Check Transaction → wedding lunas. Callback ulang
// idempoten; callback "lunas" yang tidak dikonfirmasi gateway ditolak.
func TestDuitkuEndToEnd(t *testing.T) {
	f := newFixture(t)
	fd, d := newFakeDuitku(t)
	f.svc.gw = d
	user, w := f.newWedding(t, "duitku@example.com")
	o := f.order(t, w, user)
	if !strings.HasPrefix(o.CheckoutURL, "https://app-sandbox.duitku.com/") || fd.invoice["merchantOrderId"] != o.Number {
		t.Fatalf("order: %+v", o)
	}
	cb := func() []byte {
		return duitkuCallback("kunci-api", url.Values{"merchantCode": {"DS0001"}, "amount": {"149000"}, "merchantOrderId": {o.Number},
			"resultCode": {"00"}, "reference": {"DS000125ABCD"}, "paymentCode": {"SP"}})
	}
	// Gateway masih "diproses" → callback lunas tidak diterapkan.
	fd.statusCode = "01"
	if _, err := f.svc.HandleNotification(ctx, cb()); !errors.Is(err, ErrUnconfirmed) {
		t.Fatalf("belum dikonfirmasi: %v", err)
	}
	if f.wedding(t, w.ID).IsPaid() {
		t.Fatal("tidak boleh lunas sebelum dikonfirmasi")
	}
	fd.statusCode = "00"
	if _, err := f.svc.HandleNotification(ctx, cb()); err != nil {
		t.Fatalf("callback lunas: %v", err)
	}
	if !f.wedding(t, w.ID).IsPaid() {
		t.Fatal("wedding harus lunas")
	}
	if _, err := f.svc.HandleNotification(ctx, cb()); err != nil {
		t.Errorf("callback ulang harus idempoten: %v", err)
	}
}
