package payment

import (
	"bytes"
	"context"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Midtrans: Snap mode redirect (halaman bayar di domain Midtrans — tanpa script
// pihak ketiga di halaman Lunovia). Dokumentasi: docs.midtrans.com (Snap API,
// HTTP notification, Get Status API).
type Midtrans struct {
	ServerKey string
	// SnapURL & APIURL: basis endpoint (sandbox/produksi; diganti di test).
	SnapURL string
	APIURL  string
	HTTP    *http.Client
}

// NewMidtrans memilih endpoint sandbox atau produksi.
func NewMidtrans(serverKey string, production bool) *Midtrans {
	m := &Midtrans{
		ServerKey: serverKey,
		SnapURL:   "https://app.sandbox.midtrans.com",
		APIURL:    "https://api.sandbox.midtrans.com",
		HTTP:      &http.Client{Timeout: 15 * time.Second},
	}
	if production {
		m.SnapURL, m.APIURL = "https://app.midtrans.com", "https://api.midtrans.com"
	}
	return m
}

func (m *Midtrans) Name() string { return "midtrans" }

func (m *Midtrans) do(ctx context.Context, method, url string, body any, out any) (int, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(m.ServerKey, "")
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("midtrans: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("midtrans: baca respons: %w", err)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("midtrans: respons bukan JSON (HTTP %d)", resp.StatusCode)
		}
	}
	return resp.StatusCode, nil
}

// CreateCheckout: POST /snap/v1/transactions → redirect_url.
func (m *Midtrans) CreateCheckout(ctx context.Context, req CheckoutRequest) (string, error) {
	hours := max(1, int(req.ExpiresIn.Hours()))
	body := map[string]any{
		"transaction_details": map[string]any{"order_id": req.OrderNumber, "gross_amount": req.Amount},
		"item_details": []map[string]any{{
			"id": "publish", "name": truncate(req.ItemName, 50), "price": req.Amount, "quantity": 1,
		}},
		"customer_details": map[string]any{"first_name": truncate(req.Customer.Name, 50), "email": req.Customer.Email},
		"expiry":           map[string]any{"unit": "hours", "duration": hours},
		"callbacks":        map[string]any{"finish": req.ReturnURL, "error": req.ReturnURL, "pending": req.ReturnURL},
	}
	var out struct {
		Token       string   `json:"token"`
		RedirectURL string   `json:"redirect_url"`
		Errors      []string `json:"error_messages"`
	}
	code, err := m.do(ctx, http.MethodPost, m.SnapURL+"/snap/v1/transactions", body, &out)
	if err != nil {
		return "", err
	}
	if code != http.StatusCreated && code != http.StatusOK || out.RedirectURL == "" {
		return "", fmt.Errorf("midtrans: buat transaksi gagal (HTTP %d): %s", code, strings.Join(out.Errors, "; "))
	}
	return out.RedirectURL, nil
}

// midtransStatus: bentuk notifikasi HTTP & respons Get Status.
type midtransStatus struct {
	OrderID           string `json:"order_id"`
	StatusCode        string `json:"status_code"`
	GrossAmount       string `json:"gross_amount"`
	SignatureKey      string `json:"signature_key"`
	TransactionStatus string `json:"transaction_status"`
	FraudStatus       string `json:"fraud_status"`
	TransactionID     string `json:"transaction_id"`
	PaymentType       string `json:"payment_type"`
	Currency          string `json:"currency"`
	SettlementTime    string `json:"settlement_time"`
	TransactionTime   string `json:"transaction_time"`
}

// Sign: SHA512(order_id + status_code + gross_amount + server_key), hex.
func (m *Midtrans) Sign(orderID, statusCode, grossAmount string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + m.ServerKey))
	return hex.EncodeToString(sum[:])
}

func (m *Midtrans) ParseNotification(body []byte) (Notification, error) {
	var s midtransStatus
	if err := json.Unmarshal(body, &s); err != nil || s.OrderID == "" {
		return Notification{}, ErrBadNotification
	}
	want := m.Sign(s.OrderID, s.StatusCode, s.GrossAmount)
	if subtle.ConstantTimeCompare([]byte(want), []byte(strings.ToLower(s.SignatureKey))) != 1 {
		return Notification{OrderNumber: s.OrderID}, ErrBadSignature
	}
	return s.notification()
}

func (m *Midtrans) FetchStatus(ctx context.Context, orderNumber string) (Notification, bool, error) {
	var s midtransStatus
	code, err := m.do(ctx, http.MethodGet, m.APIURL+"/v2/"+orderNumber+"/status", nil, &s)
	if err != nil {
		return Notification{}, false, err
	}
	// Midtrans membalas HTTP 200 dengan status_code di badan; 404 = belum ada
	// transaksi (pembayar belum memilih metode di halaman Snap).
	if code == http.StatusNotFound || s.StatusCode == "404" {
		return Notification{}, false, nil
	}
	if code >= 500 || s.OrderID == "" {
		return Notification{}, false, fmt.Errorf("midtrans: cek status gagal (HTTP %d, status_code %q)", code, s.StatusCode)
	}
	n, err := s.notification()
	return n, err == nil, err
}

// midtransTime: Midtrans mengirim waktu lokal Jakarta "2006-01-02 15:04:05".
var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600)
	}
	return loc
}()

func (s midtransStatus) notification() (Notification, error) {
	// gross_amount "149000.00": harus bilangan bulat rupiah.
	f, err := strconv.ParseFloat(s.GrossAmount, 64)
	if err != nil || f != float64(int64(f)) {
		return Notification{}, ErrBadNotification
	}
	n := Notification{
		OrderNumber: s.OrderID, TransactionID: s.TransactionID, Method: s.PaymentType,
		Amount: int64(f), Currency: s.Currency,
	}
	if n.Currency == "" {
		n.Currency = Currency
	}
	switch s.TransactionStatus {
	case "settlement":
		n.Status = StatusPaid
	case "capture": // kartu: lunas hanya bila lolos pemeriksaan fraud
		if s.FraudStatus == "accept" || s.FraudStatus == "" {
			n.Status = StatusPaid
		} else {
			n.Status = StatusPending
		}
	case "pending", "authorize":
		n.Status = StatusPending
	case "expire":
		n.Status = StatusExpired
	case "cancel":
		n.Status = StatusCancelled
	case "deny", "failure":
		n.Status = StatusFailed
	default:
		// refund / chargeback / status baru: tidak mengubah order (dicatat di log event).
		return Notification{}, fmt.Errorf("%w: transaction_status %q tidak ditangani", ErrBadNotification, s.TransactionStatus)
	}
	for _, ts := range []string{s.SettlementTime, s.TransactionTime} {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", ts, jakarta); err == nil {
			n.PaidAt = t
			break
		}
	}
	return n, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
