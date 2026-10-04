package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	paymentdb "github.com/khamdanngazis/lovaria/src/modules/payment/db"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/db"
)

var (
	// ErrUnavailable: gateway belum dikonfigurasi (PAYMENT_GATEWAY kosong).
	ErrUnavailable = errors.New("payment: pembayaran belum tersedia")
	// ErrAlreadyPaid: wedding sudah lunas — tidak perlu order baru.
	ErrAlreadyPaid = errors.New("payment: undangan ini sudah lunas")
	// ErrOrderNotFound: nomor order di notifikasi tidak dikenal.
	ErrOrderNotFound = errors.New("payment: order tidak ditemukan")
	// ErrAmountMismatch: nominal / mata uang notifikasi tidak sama dengan order.
	ErrAmountMismatch = errors.New("payment: nominal tidak sesuai order")
	// ErrUnconfirmed: gateway tidak mengonfirmasi status lunas saat dicek ulang.
	ErrUnconfirmed = errors.New("payment: status lunas tidak terkonfirmasi gateway")
)

// Hasil pemrosesan notifikasi (kolom payment_events.outcome).
const (
	OutcomeApplied   = "applied"   // status order berubah
	OutcomeDuplicate = "duplicate" // notifikasi ulang untuk status yang sama
	OutcomeIgnored   = "ignored"   // order sudah final dengan status lain
)

// Order adalah satu percobaan bayar.
type Order struct {
	ID            uuid.UUID
	WeddingID     uuid.UUID
	UserID        uuid.UUID
	Number        string
	Amount        int64
	Currency      string
	Status        string
	Method        string
	Gateway       string
	TransactionID string
	CheckoutURL   string
	ExpiredAt     time.Time
	PaidAt        *time.Time
	CreatedAt     time.Time
}

// Payable: masih bisa dibayar (pending dan belum lewat batas waktu).
func (o Order) Payable(now time.Time) bool {
	return o.Status == StatusPending && now.Before(o.ExpiredAt) && o.CheckoutURL != ""
}

func toOrder(r paymentdb.PaymentOrder) Order {
	o := Order{
		ID: r.ID, WeddingID: r.WeddingID, UserID: r.UserID, Number: r.OrderNumber,
		Amount: r.Amount, Currency: r.Currency, Status: r.Status, Method: r.PaymentMethod,
		Gateway: r.Gateway, CheckoutURL: r.CheckoutUrl, ExpiredAt: r.ExpiredAt, PaidAt: r.PaidAt, CreatedAt: r.CreatedAt,
	}
	if r.GatewayTransactionID != nil {
		o.TransactionID = *r.GatewayTransactionID
	}
	return o
}

// Weddings adalah bagian service wedding yang dibutuhkan payment (kolom
// weddings.paid_at milik modul wedding — payment tidak menulis tabel itu).
type Weddings interface {
	MarkPaid(ctx context.Context, weddingID uuid.UUID, source string) (bool, error)
}

type Service struct {
	pool     *pgxpool.Pool
	q        *paymentdb.Queries
	gw       Gateway // nil → pembayaran belum tersedia
	weddings Weddings
	price    int64
	expiry   time.Duration
	baseURL  string
	now      func() time.Time
	log      *slog.Logger
}

// Config: setelan Service.
type Config struct {
	Gateway Gateway // nil → pembayaran belum tersedia
	Price   int64
	Expiry  time.Duration
	BaseURL string
}

func NewService(pool *pgxpool.Pool, weddings Weddings, cfg Config, log *slog.Logger) *Service {
	return &Service{
		pool: pool, q: paymentdb.New(pool), gw: cfg.Gateway, weddings: weddings,
		price: cfg.Price, expiry: cfg.Expiry, baseURL: strings.TrimSuffix(cfg.BaseURL, "/"), now: time.Now, log: log,
	}
}

// Enabled: gateway terpasang.
func (s *Service) Enabled() bool { return s.gw != nil }

// Price: harga publikasi per wedding (rupiah).
func (s *Service) Price() int64 { return s.price }

// GatewayName: nama gateway aktif ("" bila belum ada).
func (s *Service) GatewayName() string {
	if s.gw == nil {
		return ""
	}
	return s.gw.Name()
}

// orderNumber: LVR-YYYYMMDD-000001 (tanggal WIB + nomor urut dari sequence).
func (s *Service) orderNumber(ctx context.Context) (string, error) {
	n, err := s.q.NextOrderSeq(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("LVR-%s-%06d", s.now().In(jakarta).Format("20060102"), n), nil
}

// CreateOrder menyiapkan pembayaran untuk wedding w. Order pending yang masih
// berlaku dipakai ulang (tidak ada order ganda); setelah kedaluwarsa/gagal
// dibuat percobaan baru untuk wedding yang sama.
func (s *Service) CreateOrder(ctx context.Context, w wedding.Wedding, buyer uuid.UUID, c Customer) (Order, error) {
	if s.gw == nil {
		return Order{}, ErrUnavailable
	}
	if w.IsPaid() {
		return Order{}, ErrAlreadyPaid
	}
	now := s.now()
	if _, err := s.q.ExpireStale(ctx, now); err != nil {
		return Order{}, err
	}
	if cur, err := s.q.ActivePendingOrder(ctx, paymentdb.ActivePendingOrderParams{WeddingID: w.ID, Now: now}); err == nil {
		return toOrder(cur), nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Order{}, err
	}

	number, err := s.orderNumber(ctx)
	if err != nil {
		return Order{}, err
	}
	row, err := s.q.CreateOrder(ctx, paymentdb.CreateOrderParams{
		ID: db.NewID(), WeddingID: w.ID, UserID: buyer, OrderNumber: number,
		Amount: s.price, Gateway: s.gw.Name(), ExpiredAt: now.Add(s.expiry),
	})
	if err != nil {
		return Order{}, fmt.Errorf("payment: buat order: %w", err)
	}
	checkout, err := s.gw.CreateCheckout(ctx, CheckoutRequest{
		OrderNumber: number, Amount: s.price, ItemName: "Publikasi undangan " + w.Title,
		Customer: c, ExpiresIn: s.expiry, ReturnURL: s.baseURL + w.DashboardURL("/payment/return"),
	})
	if err != nil {
		// Gateway menolak: tandai gagal supaya percobaan berikutnya membuat order baru.
		if _, ferr := s.q.ApplyStatus(context.WithoutCancel(ctx), paymentdb.ApplyStatusParams{
			ID: row.ID, WeddingID: w.ID, Status: StatusFailed,
		}); ferr != nil {
			s.log.ErrorContext(ctx, "payment: tandai order gagal", slog.String("order", number), slog.String("error", ferr.Error()))
		}
		return Order{}, fmt.Errorf("payment: buat sesi bayar: %w", err)
	}
	row, err = s.q.SetCheckoutURL(ctx, paymentdb.SetCheckoutURLParams{ID: row.ID, WeddingID: w.ID, CheckoutUrl: checkout})
	if err != nil {
		return Order{}, err
	}
	return toOrder(row), nil
}

func (s *Service) logEvent(ctx context.Context, body []byte, orderNumber string, sigOK bool, outcome string) {
	payload := body
	if !json.Valid(payload) {
		payload, _ = json.Marshal(map[string]string{"raw": string(body)})
	}
	if err := s.q.InsertEvent(context.WithoutCancel(ctx), paymentdb.InsertEventParams{
		ID: db.NewID(), Gateway: s.GatewayName(), OrderNumber: orderNumber, SignatureOk: sigOK, Outcome: outcome, Payload: payload,
	}); err != nil {
		s.log.ErrorContext(ctx, "payment: simpan log webhook", slog.String("error", err.Error()))
	}
}

// HandleNotification memproses webhook gateway (sumber kebenaran status):
// verifikasi tanda tangan, cocokkan order & nominal, lalu terapkan status
// secara idempoten. Setiap webhook — termasuk yang ditolak — dicatat di
// payment_events.
func (s *Service) HandleNotification(ctx context.Context, body []byte) (outcome string, err error) {
	if s.gw == nil {
		return "", ErrUnavailable
	}
	n, err := s.gw.ParseNotification(body)
	if err != nil {
		reason := "rejected:invalid"
		if errors.Is(err, ErrBadSignature) {
			reason = "rejected:signature"
		}
		s.logEvent(ctx, body, n.OrderNumber, false, reason)
		return "", err
	}
	// Status lunas dikonfirmasi ulang langsung ke gateway sebelum diterapkan.
	if n.Status == StatusPaid {
		got, found, ferr := s.gw.FetchStatus(ctx, n.OrderNumber)
		switch {
		case ferr != nil:
			s.logEvent(ctx, body, n.OrderNumber, true, "rejected:status-check-error")
			return "", fmt.Errorf("payment: cek status ke gateway: %w", ferr)
		case !found || got.Status != StatusPaid || got.Amount != n.Amount:
			s.logEvent(ctx, body, n.OrderNumber, true, "rejected:unconfirmed")
			return "", ErrUnconfirmed
		}
	}
	outcome, err = s.apply(ctx, n)
	switch {
	case errors.Is(err, ErrOrderNotFound):
		s.logEvent(ctx, body, n.OrderNumber, true, "rejected:unknown-order")
	case errors.Is(err, ErrAmountMismatch):
		s.logEvent(ctx, body, n.OrderNumber, true, "rejected:amount")
	case err != nil:
		s.logEvent(ctx, body, n.OrderNumber, true, "error")
	default:
		s.logEvent(ctx, body, n.OrderNumber, true, outcome)
	}
	return outcome, err
}

// apply menerapkan status dari gateway ke order di dalam transaksi (baris
// dikunci). Order final tidak berubah lagi, kecuali expired → paid: uang yang
// sudah diterima gateway tetap dihormati.
func (s *Service) apply(ctx context.Context, n Notification) (string, error) {
	var (
		outcome string
		order   paymentdb.PaymentOrder
	)
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		cur, err := q.GetOrderByNumberForUpdate(ctx, n.OrderNumber)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		if n.Amount != cur.Amount || !strings.EqualFold(n.Currency, cur.Currency) {
			return ErrAmountMismatch
		}
		order = cur
		switch {
		case cur.Status == n.Status && n.Status != StatusPending:
			outcome = OutcomeDuplicate
			return nil
		case n.Status == StatusPending:
			outcome = OutcomeDuplicate
			if cur.Status != StatusPending {
				outcome = OutcomeIgnored
				return nil
			}
			return q.SetPendingDetails(ctx, paymentdb.SetPendingDetailsParams{
				ID: cur.ID, WeddingID: cur.WeddingID, PaymentMethod: n.Method, GatewayTransactionID: strPtr(n.TransactionID),
			})
		}
		p := paymentdb.ApplyStatusParams{
			ID: cur.ID, WeddingID: cur.WeddingID, Status: n.Status,
			PaymentMethod: n.Method, GatewayTransactionID: strPtr(n.TransactionID),
		}
		if n.Status == StatusPaid {
			at := n.PaidAt
			if at.IsZero() {
				at = s.now()
			}
			p.PaidAt = &at
		}
		updated, err := q.ApplyStatus(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) {
			outcome = OutcomeIgnored // order sudah final dengan status lain
			return nil
		}
		if err != nil {
			return err
		}
		order, outcome = updated, OutcomeApplied
		return nil
	})
	if err != nil {
		return "", err
	}
	// Buka publikasi. Idempoten, dan diulang untuk notifikasi duplikat supaya
	// kegagalan sebelumnya (order sudah paid, wedding belum ditandai) pulih
	// saat gateway mengirim ulang.
	if order.Status == StatusPaid {
		if _, err := s.weddings.MarkPaid(ctx, order.WeddingID, wedding.PaidGateway); err != nil {
			return "", fmt.Errorf("payment: buka publikasi wedding %s: %w", order.WeddingID, err)
		}
	}
	return outcome, nil
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Refresh menanyakan status order terakhir yang masih pending langsung ke
// gateway (halaman kembali dari pembayaran) supaya pasangan tidak menunggu
// webhook. Jalur penerapannya sama dengan webhook.
func (s *Service) Refresh(ctx context.Context, weddingID uuid.UUID) error {
	if s.gw == nil {
		return nil
	}
	row, err := s.q.LatestOrder(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if row.Status != StatusPending && row.Status != StatusExpired {
		return nil
	}
	n, found, err := s.gw.FetchStatus(ctx, row.OrderNumber)
	if err != nil || !found {
		return err
	}
	if _, err := s.apply(ctx, n); err != nil && !errors.Is(err, ErrAmountMismatch) {
		return err
	}
	_, err = s.q.ExpireStale(ctx, s.now())
	return err
}

// Summary: keadaan pembayaran wedding untuk dashboard.
type Summary struct {
	Latest  *Order // percobaan terakhir (nil bila belum pernah)
	Payable bool   // Latest masih bisa dibayar (lanjutkan pembayaran)
}

func (s *Service) Summary(ctx context.Context, weddingID uuid.UUID) (Summary, error) {
	row, err := s.q.LatestOrder(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, nil
	}
	if err != nil {
		return Summary{}, err
	}
	o := toOrder(row)
	now := s.now()
	if o.Status == StatusPending && !now.Before(o.ExpiredAt) {
		o.Status = StatusExpired // lewat batas waktu, belum disapu ExpireStale
	}
	return Summary{Latest: &o, Payable: o.Payable(now)}, nil
}

// ListOrders: riwayat percobaan bayar wedding, terbaru dulu.
func (s *Service) ListOrders(ctx context.Context, weddingID uuid.UUID) ([]Order, error) {
	rows, err := s.q.ListOrders(ctx, weddingID)
	if err != nil {
		return nil, err
	}
	out := make([]Order, len(rows))
	for i, r := range rows {
		out[i] = toOrder(r)
	}
	return out, nil
}

// OrderByNumber mencari order dari nomornya (halaman bayar simulasi).
func (s *Service) OrderByNumber(ctx context.Context, number string) (Order, error) {
	row, err := s.q.GetOrderByNumber(ctx, number)
	if errors.Is(err, pgx.ErrNoRows) {
		return Order{}, ErrOrderNotFound
	}
	if err != nil {
		return Order{}, err
	}
	return toOrder(row), nil
}

// Fake mengembalikan gateway simulasi bila itu yang aktif (dev/test).
func (s *Service) Fake() (*Fake, bool) {
	f, ok := s.gw.(*Fake)
	return f, ok
}
