package payment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func notif(m *Midtrans, order, code, amount, status string, extra map[string]string) []byte {
	p := map[string]string{
		"order_id": order, "status_code": code, "gross_amount": amount, "transaction_status": status,
		"transaction_id": "tx-123", "payment_type": "qris", "currency": "IDR",
		"signature_key": m.Sign(order, code, amount),
	}
	for k, v := range extra {
		p[k] = v
	}
	b, _ := json.Marshal(p)
	return b
}

func TestMidtransNotification(t *testing.T) {
	m := NewMidtrans("SB-Mid-server-rahasia", false)
	// Tanda tangan sesuai dokumentasi: SHA512(order_id + status_code + gross_amount + server_key).
	n, err := m.ParseNotification(notif(m, "LVR-20261004-000001", "200", "149000.00", "settlement", map[string]string{"settlement_time": "2026-10-04 10:15:00"}))
	if err != nil {
		t.Fatal(err)
	}
	if n.OrderNumber != "LVR-20261004-000001" || n.Status != StatusPaid || n.Amount != 149000 || n.Currency != "IDR" || n.Method != "qris" || n.TransactionID != "tx-123" {
		t.Errorf("notifikasi = %+v", n)
	}
	// Waktu Midtrans = WIB.
	if want := time.Date(2026, 10, 4, 3, 15, 0, 0, time.UTC); !n.PaidAt.Equal(want) {
		t.Errorf("paid_at = %v, want %v", n.PaidAt, want)
	}

	for status, want := range map[string]string{
		"settlement": StatusPaid, "pending": StatusPending, "expire": StatusExpired,
		"cancel": StatusCancelled, "deny": StatusFailed, "failure": StatusFailed,
	} {
		n, err := m.ParseNotification(notif(m, "O1", "200", "149000.00", status, nil))
		if err != nil || n.Status != want {
			t.Errorf("%s → %q %v, want %q", status, n.Status, err, want)
		}
	}
	// Kartu: capture hanya lunas bila lolos fraud.
	if n, _ := m.ParseNotification(notif(m, "O1", "200", "149000.00", "capture", map[string]string{"fraud_status": "accept"})); n.Status != StatusPaid {
		t.Errorf("capture/accept = %q", n.Status)
	}
	if n, _ := m.ParseNotification(notif(m, "O1", "200", "149000.00", "capture", map[string]string{"fraud_status": "challenge"})); n.Status != StatusPending {
		t.Errorf("capture/challenge = %q", n.Status)
	}

	// Tanda tangan salah: dibuat dengan kunci lain, atau isi diubah setelah ditandatangani.
	other := NewMidtrans("kunci-lain", false)
	if _, err := m.ParseNotification(notif(other, "O1", "200", "149000.00", "settlement", nil)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("kunci lain: %v", err)
	}
	tampered := strings.Replace(string(notif(m, "O1", "200", "149000.00", "settlement", nil)), `"149000.00"`, `"1.00"`, 1)
	if _, err := m.ParseNotification([]byte(tampered)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("nominal diubah: %v", err)
	}
	for _, bad := range []string{``, `bukan json`, `{}`, `{"order_id":""}`} {
		if _, err := m.ParseNotification([]byte(bad)); !errors.Is(err, ErrBadNotification) {
			t.Errorf("badan %q: %v", bad, err)
		}
	}
	// Nominal pecahan & status tak dikenal ditolak walau tanda tangannya sah.
	if _, err := m.ParseNotification(notif(m, "O1", "200", "149000.50", "settlement", nil)); !errors.Is(err, ErrBadNotification) {
		t.Errorf("nominal pecahan: %v", err)
	}
	if _, err := m.ParseNotification(notif(m, "O1", "200", "149000.00", "refund", nil)); !errors.Is(err, ErrBadNotification) {
		t.Errorf("status refund: %v", err)
	}
}

func TestMidtransCheckoutAndStatus(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, _ := r.BasicAuth()
		gotAuth = user
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/snap/v1/transactions":
			b, _ := io.ReadAll(r.Body)
			gotBody = string(b)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"token":"tok","redirect_url":"https://app.sandbox.midtrans.com/snap/v4/redirection/tok"}`))
		case r.URL.Path == "/v2/LVR-1/status":
			_, _ = w.Write([]byte(`{"order_id":"LVR-1","status_code":"200","gross_amount":"149000.00","transaction_status":"settlement","transaction_id":"tx-9","payment_type":"bank_transfer"}`))
		case r.URL.Path == "/v2/LVR-BARU/status":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"status_code":"404","status_message":"Transaction doesn't exist."}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	m := NewMidtrans("server-key", false)
	m.SnapURL, m.APIURL = srv.URL, srv.URL
	ctx := context.Background()

	url, err := m.CreateCheckout(ctx, CheckoutRequest{
		OrderNumber: "LVR-1", Amount: 149000, ItemName: "Publikasi undangan", Customer: Customer{Name: "Sarah", Email: "s@example.com"},
		ExpiresIn: 24 * time.Hour, ReturnURL: "https://lovoria.test/dashboard/weddings/x/payment/return",
	})
	if err != nil || !strings.HasSuffix(url, "/redirection/tok") || gotAuth != "server-key" {
		t.Fatalf("checkout: %q %v auth=%q", url, err, gotAuth)
	}
	for _, want := range []string{`"order_id":"LVR-1"`, `"gross_amount":149000`, `"duration":24`, `"unit":"hours"`, `"finish":"https://lovoria.test/dashboard/weddings/x/payment/return"`, `"email":"s@example.com"`} {
		if !strings.Contains(gotBody, want) {
			t.Errorf("badan Snap tidak memuat %s: %s", want, gotBody)
		}
	}
	n, found, err := m.FetchStatus(ctx, "LVR-1")
	if err != nil || !found || n.Status != StatusPaid || n.Amount != 149000 || n.TransactionID != "tx-9" {
		t.Errorf("status: %+v %v %v", n, found, err)
	}
	// Belum ada transaksi (pembayar belum memilih metode) → found=false, bukan error.
	if _, found, err := m.FetchStatus(ctx, "LVR-BARU"); err != nil || found {
		t.Errorf("order baru: found=%v err=%v", found, err)
	}
	if _, _, err := m.FetchStatus(ctx, "LVR-ERROR"); err == nil {
		t.Error("5xx gateway harus menjadi error")
	}
	// Endpoint produksi vs sandbox.
	if p := NewMidtrans("k", true); p.SnapURL != "https://app.midtrans.com" || p.APIURL != "https://api.midtrans.com" {
		t.Errorf("endpoint produksi: %s %s", p.SnapURL, p.APIURL)
	}
}
