package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Duitku: mode "POP" — halaman bayar di domain Duitku tempat pembayar memilih
// metode (QRIS, e-wallet, virtual account), tanpa script pihak ketiga di
// halaman Lunovia. Dokumentasi: docs.duitku.com/pop/en (Create Invoice,
// Callback) dan docs.duitku.com/api/en (Check Transaction).
//
// Semua tanda tangan memakai HMAC-SHA256 dengan API key (hex huruf kecil);
// skema MD5/SHA256 lama sudah tidak berlaku menurut dokumentasi Duitku.
type Duitku struct {
	MerchantCode string
	APIKey       string
	// InvoiceURL: endpoint Create Invoice; StatusURL: endpoint Check
	// Transaction (sandbox/produksi; diganti di test).
	InvoiceURL string
	StatusURL  string
	// CallbackURL: alamat webhook Lunovia yang dikirim ke Duitku per transaksi.
	CallbackURL string
	HTTP        *http.Client
	now         func() time.Time
}

// NewDuitku memilih endpoint sandbox atau produksi. baseURL: alamat Lunovia
// (callback = baseURL + /webhooks/duitku).
func NewDuitku(merchantCode, apiKey, baseURL string, production bool) *Duitku {
	d := &Duitku{
		MerchantCode: merchantCode, APIKey: apiKey,
		InvoiceURL:  "https://api-sandbox.duitku.com/api/merchant/createInvoice",
		StatusURL:   "https://sandbox.duitku.com/webapi/api/merchant/transactionStatus",
		CallbackURL: strings.TrimRight(baseURL, "/") + "/webhooks/duitku",
		HTTP:        &http.Client{Timeout: 15 * time.Second},
		now:         time.Now,
	}
	if production {
		d.InvoiceURL = "https://api-prod.duitku.com/api/merchant/createInvoice"
		d.StatusURL = "https://passport.duitku.com/webapi/api/merchant/transactionStatus"
	}
	return d
}

func (d *Duitku) Name() string { return "duitku" }

// sign: HMAC-SHA256(pesan, API key), hex huruf kecil.
func (d *Duitku) sign(msg string) string {
	mac := hmac.New(sha256.New, []byte(d.APIKey))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func (d *Duitku) post(ctx context.Context, url string, headers map[string]string, body, out any) (int, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := d.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("duitku: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("duitku: baca respons: %w", err)
	}
	if len(raw) > 0 && json.Unmarshal(raw, out) != nil {
		return resp.StatusCode, fmt.Errorf("duitku: respons bukan JSON (HTTP %d): %.200s", resp.StatusCode, raw)
	}
	return resp.StatusCode, nil
}

// CreateCheckout: Create Invoice → paymentUrl (halaman bayar Duitku).
func (d *Duitku) CreateCheckout(ctx context.Context, req CheckoutRequest) (string, error) {
	ts := strconv.FormatInt(d.now().UnixMilli(), 10)
	name := truncate(req.ItemName, 255)
	body := map[string]any{
		"paymentAmount":   req.Amount,
		"merchantOrderId": req.OrderNumber,
		"productDetails":  name,
		"email":           req.Customer.Email,
		"customerVaName":  truncate(req.Customer.Name, 20),
		"callbackUrl":     d.CallbackURL,
		"returnUrl":       req.ReturnURL,
		"expiryPeriod":    max(5, int(req.ExpiresIn.Minutes())),
		"itemDetails":     []map[string]any{{"name": truncate(name, 50), "price": req.Amount, "quantity": 1}},
	}
	var out struct {
		Reference     string `json:"reference"`
		PaymentURL    string `json:"paymentUrl"`
		StatusCode    string `json:"statusCode"`
		StatusMessage string `json:"statusMessage"`
		Message       string `json:"Message"`
	}
	code, err := d.post(ctx, d.InvoiceURL, map[string]string{
		"x-duitku-merchantcode": d.MerchantCode,
		"x-duitku-timestamp":    ts,
		"x-duitku-signature":    d.sign(d.MerchantCode + ts),
	}, body, &out)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK || out.StatusCode != "00" || out.PaymentURL == "" {
		msg := out.StatusMessage
		if msg == "" {
			msg = out.Message
		}
		return "", fmt.Errorf("duitku: buat invoice gagal (HTTP %d, status %q): %s", code, out.StatusCode, msg)
	}
	return out.PaymentURL, nil
}

// parseAmount: nominal rupiah bulat ("149000" atau "149000.00").
func parseAmount(s string) (int64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 0 || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

// ParseNotification: callback Duitku (form-urlencoded). Tanda tangan =
// HMAC-SHA256(merchantCode + amount + merchantOrderId). resultCode "00" =
// berhasil; kode lain = gagal (callback hanya dikirim untuk hasil akhir).
func (d *Duitku) ParseNotification(body []byte) (Notification, error) {
	v, err := url.ParseQuery(string(body))
	if err != nil || v.Get("merchantOrderId") == "" || v.Get("amount") == "" {
		return Notification{}, ErrBadNotification
	}
	n := Notification{OrderNumber: v.Get("merchantOrderId")}
	want := d.sign(v.Get("merchantCode") + v.Get("amount") + v.Get("merchantOrderId"))
	if v.Get("merchantCode") != d.MerchantCode || subtle.ConstantTimeCompare([]byte(want), []byte(strings.ToLower(v.Get("signature")))) != 1 {
		return n, ErrBadSignature
	}
	amount, ok := parseAmount(v.Get("amount"))
	if !ok {
		return n, ErrBadNotification
	}
	n.Amount, n.Currency = amount, Currency
	n.TransactionID, n.Method = v.Get("reference"), v.Get("paymentCode")
	if v.Get("resultCode") == "00" {
		n.Status = StatusPaid
		n.PaidAt = d.now()
	} else {
		n.Status = StatusFailed
	}
	return n, nil
}

// FetchStatus: Check Transaction. statusCode 00 = lunas, 01 = masih diproses,
// 02 = gagal/kedaluwarsa. Duitku memblokir sementara pemanggil yang terlalu
// sering — Service membatasi frekuensinya (lihat statusCheckEvery).
func (d *Duitku) FetchStatus(ctx context.Context, orderNumber string) (Notification, bool, error) {
	var out struct {
		MerchantOrderID string `json:"merchantOrderId"`
		Reference       string `json:"reference"`
		Amount          string `json:"amount"`
		StatusCode      string `json:"statusCode"`
		StatusMessage   string `json:"statusMessage"`
		Message         string `json:"Message"`
	}
	code, err := d.post(ctx, d.StatusURL, nil, map[string]string{
		"merchantCode": d.MerchantCode, "merchantOrderId": orderNumber,
		"signature": d.sign(d.MerchantCode + orderNumber),
	}, &out)
	if err != nil {
		return Notification{}, false, err
	}
	if code >= 500 {
		return Notification{}, false, fmt.Errorf("duitku: cek status gagal (HTTP %d)", code)
	}
	if code != http.StatusOK || out.MerchantOrderID == "" {
		return Notification{}, false, nil // transaksi belum ada (pembayar belum memilih metode)
	}
	amount, ok := parseAmount(out.Amount)
	if !ok {
		return Notification{}, false, fmt.Errorf("duitku: nominal tidak terbaca %q", out.Amount)
	}
	n := Notification{OrderNumber: out.MerchantOrderID, TransactionID: out.Reference, Amount: amount, Currency: Currency}
	switch out.StatusCode {
	case "00":
		n.Status = StatusPaid
	case "01":
		n.Status = StatusPending
	case "02":
		n.Status = StatusFailed
	default:
		return Notification{}, false, fmt.Errorf("duitku: status tidak dikenal %q: %s", out.StatusCode, out.StatusMessage)
	}
	return n, true, nil
}
