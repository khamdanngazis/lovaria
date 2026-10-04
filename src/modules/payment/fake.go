package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// Fake: gateway simulasi untuk development, test, dan e2e — tidak ada uang
// sungguhan. Halaman bayarnya dilayani Lunovia sendiri (FakeCheckout di
// handler) dan "webhook"-nya ditandatangani HMAC dengan kunci aplikasi.
// Config menolak gateway ini di production.
type Fake struct {
	Secret  []byte
	BaseURL string // basis URL Lunovia (halaman bayar simulasi)

	mu     sync.Mutex
	status map[string]Notification // status terakhir per nomor order
}

func NewFake(secret []byte, baseURL string) *Fake {
	return &Fake{Secret: secret, BaseURL: baseURL, status: map[string]Notification{}}
}

func (f *Fake) Name() string { return "fake" }

func (f *Fake) CreateCheckout(_ context.Context, req CheckoutRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.status[req.OrderNumber] = Notification{OrderNumber: req.OrderNumber, Status: StatusPending, Amount: req.Amount, Currency: Currency}
	return f.BaseURL + FakeCheckoutPath + req.OrderNumber, nil
}

// FakeCheckoutPath: awalan halaman bayar simulasi.
const FakeCheckoutPath = "/payment/simulasi/"

type fakePayload struct {
	Notification
	Signature string `json:"signature"`
}

func (f *Fake) sign(n Notification) string {
	b, _ := json.Marshal(n)
	mac := hmac.New(sha256.New, f.Secret)
	mac.Write(b)
	return hex.EncodeToString(mac.Sum(nil))
}

// Notify membuat badan webhook bertanda tangan untuk hasil simulasi dan
// mencatatnya sebagai status terakhir order (dipakai halaman simulasi & test).
func (f *Fake) Notify(orderNumber, status string, amount int64, at time.Time) []byte {
	n := Notification{
		OrderNumber: orderNumber, TransactionID: "fake-" + orderNumber, Method: "simulasi",
		Status: status, Amount: amount, Currency: Currency,
	}
	if status == StatusPaid {
		n.PaidAt = at.UTC().Truncate(time.Second)
	}
	f.mu.Lock()
	f.status[orderNumber] = n
	f.mu.Unlock()
	b, _ := json.Marshal(fakePayload{Notification: n, Signature: f.sign(n)})
	return b
}

func (f *Fake) ParseNotification(body []byte) (Notification, error) {
	var p fakePayload
	if err := json.Unmarshal(body, &p); err != nil || p.OrderNumber == "" {
		return Notification{}, ErrBadNotification
	}
	if !hmac.Equal([]byte(f.sign(p.Notification)), []byte(p.Signature)) {
		return Notification{OrderNumber: p.OrderNumber}, ErrBadSignature
	}
	return p.Notification, nil
}

func (f *Fake) FetchStatus(_ context.Context, orderNumber string) (Notification, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.status[orderNumber]
	return n, ok, nil
}
