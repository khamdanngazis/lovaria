// Package payment: pembayaran sekali per wedding yang membuka publikasi (T23).
// Satu baris payment_orders per percobaan bayar; hanya order "paid" yang
// membuka publikasi (wedding.MarkPaid). Sumber kebenaran status adalah webhook
// gateway yang tanda tangannya diverifikasi — bukan redirect dari browser.
package payment

import (
	"context"
	"errors"
	"time"
)

// Status order (kolom payment_orders.status). "Belum bayar" bukan status order:
// artinya wedding belum punya order paid.
const (
	StatusPending   = "pending"
	StatusPaid      = "paid"
	StatusExpired   = "expired"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Currency: satu-satunya mata uang yang didukung.
const Currency = "IDR"

var (
	// ErrBadSignature: tanda tangan webhook tidak cocok (bukan dari gateway).
	ErrBadSignature = errors.New("payment: tanda tangan notifikasi tidak valid")
	// ErrBadNotification: isi notifikasi tidak bisa dibaca.
	ErrBadNotification = errors.New("payment: notifikasi tidak valid")
)

// Customer: pembeli, diteruskan ke halaman bayar gateway.
type Customer struct {
	Name, Email string
}

// CheckoutRequest: data membuat sesi bayar di gateway.
type CheckoutRequest struct {
	OrderNumber string
	Amount      int64
	ItemName    string
	Customer    Customer
	ExpiresIn   time.Duration
	// ReturnURL: halaman Lovoria tujuan setelah pembayar selesai di gateway.
	// Hanya untuk kenyamanan — status tetap ditentukan webhook.
	ReturnURL string
}

// Notification: status transaksi dari gateway (webhook atau cek status).
type Notification struct {
	OrderNumber   string
	TransactionID string
	Method        string // qris, bank_transfer, gopay, …
	Status        string // Status* di atas
	Amount        int64
	Currency      string
	PaidAt        time.Time // nol bila gateway tidak mengirim
}

// Gateway adalah payment gateway di belakang Service. Implementasi: Midtrans
// (produksi & sandbox) dan Fake (simulasi untuk dev/test).
type Gateway interface {
	Name() string
	// CreateCheckout membuat sesi bayar dan mengembalikan URL halaman bayar.
	CreateCheckout(ctx context.Context, req CheckoutRequest) (checkoutURL string, err error)
	// ParseNotification memverifikasi tanda tangan webhook lalu membacanya.
	// Tanda tangan salah → ErrBadSignature.
	ParseNotification(body []byte) (Notification, error)
	// FetchStatus menanyakan status transaksi langsung ke gateway (panggilan
	// terautentikasi). found=false bila gateway belum mengenal order itu.
	FetchStatus(ctx context.Context, orderNumber string) (n Notification, found bool, err error)
}
