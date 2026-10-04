package payment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

const price = 149000

type fixture struct {
	pool     *pgxpool.Pool
	svc      *Service
	fake     *Fake
	weddings *wedding.Service
	auth     *auth.Service
	now      time.Time
}

type countEvents int

func (c countEvents) CountEvents(context.Context, uuid.UUID) (int, error) { return int(c), nil }

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := dbtest.Pool(t)
	dbtest.Reset(t, pool)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ws := wedding.NewService(wedding.NewRepository(pool))
	ws.SetEventCounter(countEvents(1))
	fake := NewFake([]byte("rahasia-test-rahasia-test-rahasia"), "https://lovoria.test")
	f := &fixture{
		pool: pool, fake: fake, weddings: ws, now: time.Date(2026, 10, 4, 20, 0, 0, 0, time.UTC), // 5 Okt 03.00 WIB
		auth: auth.NewService(auth.NewRepository(pool), &mail.LogMailer{Log: log}, "http://x", log),
	}
	f.svc = NewService(pool, ws, Config{Gateway: fake, Price: price, Expiry: 24 * time.Hour, BaseURL: "https://lovoria.test"}, log)
	f.svc.now = func() time.Time { return f.now }
	return f
}

func (f *fixture) newWedding(t *testing.T, email string) (uuid.UUID, wedding.Wedding) {
	t.Helper()
	u, err := f.auth.Register(ctx, auth.RegisterInput{Name: "Sarah", Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	w, err := f.weddings.CreateWedding(ctx, u.ID, wedding.CreateInput{GroomName: "Samuel", BrideName: "Sarah", Title: "Pernikahan S & S", WeddingDate: "2026-12-12"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID, w
}

func (f *fixture) order(t *testing.T, w wedding.Wedding, user uuid.UUID) Order {
	t.Helper()
	o, err := f.svc.CreateOrder(ctx, w, user, Customer{Name: "Sarah", Email: "s@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func (f *fixture) wedding(t *testing.T, id uuid.UUID) wedding.Wedding {
	t.Helper()
	w, err := f.weddings.GetWedding(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := f.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestCreateOrderReusesPendingAndRetries(t *testing.T) {
	f := newFixture(t)
	user, w := f.newWedding(t, "a@example.com")

	o := f.order(t, w, user)
	// Nomor order: LVR-YYYYMMDD-NNNNNN dengan tanggal WIB (20.00 UTC = besok pagi WIB).
	if !regexp.MustCompile(`^LVR-20261005-\d{6}$`).MatchString(o.Number) {
		t.Errorf("nomor order = %q", o.Number)
	}
	if o.Status != StatusPending || o.Amount != price || o.Currency != "IDR" || o.Gateway != "fake" ||
		!o.ExpiredAt.Equal(f.now.Add(24*time.Hour)) || o.CheckoutURL != "https://lovoria.test"+FakeCheckoutPath+o.Number || o.UserID != user {
		t.Errorf("order = %+v", o)
	}
	// Klik "Bayar" lagi selagi masih berlaku → order yang sama, bukan order baru.
	if again := f.order(t, w, user); again.ID != o.ID {
		t.Error("order pending harus dipakai ulang")
	}
	// Kedaluwarsa → percobaan baru untuk wedding yang sama (wedding tidak bertambah).
	f.now = f.now.Add(25 * time.Hour)
	o2 := f.order(t, w, user)
	if o2.ID == o.ID || o2.Number == o.Number {
		t.Error("setelah kedaluwarsa harus dibuat order baru")
	}
	orders, _ := f.svc.ListOrders(ctx, w.ID)
	if len(orders) != 2 || orders[0].ID != o2.ID || orders[1].Status != StatusExpired {
		t.Errorf("riwayat = %+v", orders)
	}
	if n := f.count(t, `SELECT count(*) FROM weddings`); n != 1 {
		t.Errorf("wedding = %d, want 1", n)
	}
	// Gagal → juga boleh coba lagi.
	if _, err := f.svc.HandleNotification(ctx, f.fake.Notify(o2.Number, StatusFailed, price, f.now)); err != nil {
		t.Fatal(err)
	}
	if o3 := f.order(t, w, user); o3.ID == o2.ID {
		t.Error("setelah gagal harus dibuat order baru")
	}
	// Wedding lunas tidak perlu order lagi; tanpa gateway → belum tersedia.
	if _, err := f.weddings.MarkPaid(ctx, w.ID, wedding.PaidAdmin); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.CreateOrder(ctx, f.wedding(t, w.ID), user, Customer{}); !errors.Is(err, ErrAlreadyPaid) {
		t.Errorf("sudah lunas: %v", err)
	}
	none := NewService(f.pool, f.weddings, Config{Price: price, Expiry: time.Hour}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, w2 := f.newWedding(t, "b@example.com")
	if _, err := none.CreateOrder(ctx, w2, user, Customer{}); !errors.Is(err, ErrUnavailable) || none.Enabled() {
		t.Errorf("tanpa gateway: %v", err)
	}
}

func TestNotificationPaidIsIdempotent(t *testing.T) {
	f := newFixture(t)
	user, w := f.newWedding(t, "a@example.com")
	o := f.order(t, w, user)

	// Pending: status tetap, publikasi masih terkunci.
	if out, err := f.svc.HandleNotification(ctx, f.fake.Notify(o.Number, StatusPending, price, f.now)); err != nil || out != OutcomeDuplicate {
		t.Fatalf("pending: %q %v", out, err)
	}
	if f.wedding(t, w.ID).IsPaid() {
		t.Fatal("pending tidak boleh membuka publikasi")
	}
	if _, err := f.weddings.Transition(ctx, w.ID, wedding.StatusPublished, wedding.Actor{Kind: wedding.ActorUser, UserID: user}); !errors.Is(err, wedding.ErrPaymentRequired) {
		t.Fatalf("publish sebelum lunas: %v", err)
	}

	paidAt := f.now.Add(10 * time.Minute)
	body := f.fake.Notify(o.Number, StatusPaid, price, paidAt)
	out, err := f.svc.HandleNotification(ctx, body)
	if err != nil || out != OutcomeApplied {
		t.Fatalf("paid: %q %v", out, err)
	}
	got, _ := f.svc.OrderByNumber(ctx, o.Number)
	if got.Status != StatusPaid || got.PaidAt == nil || !got.PaidAt.Equal(paidAt) || got.TransactionID != "fake-"+o.Number || got.Method != "simulasi" {
		t.Errorf("order = %+v", got)
	}
	w1 := f.wedding(t, w.ID)
	if !w1.IsPaid() || w1.PaidSource != wedding.PaidGateway {
		t.Fatalf("wedding belum lunas: %+v", w1.PaidAt)
	}

	// Webhook yang sama dikirim ulang (gateway retry): tidak ada aktivasi ganda.
	f.now = f.now.Add(time.Hour)
	for i := 0; i < 3; i++ {
		if out, err := f.svc.HandleNotification(ctx, body); err != nil || out != OutcomeDuplicate {
			t.Fatalf("ulang %d: %q %v", i, out, err)
		}
	}
	again, _ := f.svc.OrderByNumber(ctx, o.Number)
	w2 := f.wedding(t, w.ID)
	if !again.PaidAt.Equal(*got.PaidAt) || !w2.PaidAt.Equal(*w1.PaidAt) {
		t.Error("paid_at berubah oleh notifikasi ulang")
	}
	if n := f.count(t, `SELECT count(*) FROM payment_orders WHERE status = 'paid'`); n != 1 {
		t.Errorf("order paid = %d", n)
	}
	// Expire / gagal yang datang terlambat tidak membatalkan pembayaran.
	for _, late := range []string{StatusExpired, StatusFailed, StatusCancelled} {
		if out, err := f.svc.HandleNotification(ctx, f.fake.Notify(o.Number, late, price, f.now)); err != nil || out != OutcomeIgnored {
			t.Errorf("%s setelah paid: %q %v", late, out, err)
		}
	}
	if o, _ := f.svc.OrderByNumber(ctx, o.Number); o.Status != StatusPaid || !f.wedding(t, w.ID).IsPaid() {
		t.Error("status paid harus final")
	}
	// Setelah lunas, publikasi lolos.
	if got, err := f.weddings.Transition(ctx, w.ID, wedding.StatusPublished, wedding.Actor{Kind: wedding.ActorUser, UserID: user}); err != nil || got.Status != wedding.StatusPublished {
		t.Errorf("publish setelah lunas: %v", err)
	}
	// Semua webhook tercatat, termasuk duplikat & yang diabaikan.
	if n := f.count(t, `SELECT count(*) FROM payment_events WHERE order_number = $1 AND signature_ok`, o.Number); n != 8 {
		t.Errorf("log webhook = %d, want 8", n)
	}
}

func TestNotificationRejected(t *testing.T) {
	f := newFixture(t)
	user, w := f.newWedding(t, "a@example.com")
	o := f.order(t, w, user)
	locked := func(msg string) {
		t.Helper()
		if got, _ := f.svc.OrderByNumber(ctx, o.Number); got.Status != StatusPending || f.wedding(t, w.ID).IsPaid() {
			t.Errorf("%s: order/wedding tidak boleh berubah", msg)
		}
	}

	// Tanda tangan salah (kunci lain / isi diubah).
	forged := NewFake([]byte("kunci-penyerang-kunci-penyerang-123"), "").Notify(o.Number, StatusPaid, price, f.now)
	if _, err := f.svc.HandleNotification(ctx, forged); !errors.Is(err, ErrBadSignature) {
		t.Errorf("tanda tangan palsu: %v", err)
	}
	locked("tanda tangan palsu")
	tampered := strings.Replace(string(f.fake.Notify(o.Number, StatusFailed, price, f.now)), `"failed"`, `"paid"`, 1)
	if _, err := f.svc.HandleNotification(ctx, []byte(tampered)); !errors.Is(err, ErrBadSignature) {
		t.Errorf("isi diubah: %v", err)
	}
	f.fake.Notify(o.Number, StatusPending, price, f.now) // kembalikan status di gateway
	locked("isi diubah")

	// Nominal tidak sesuai order (mis. Rp1) walau tanda tangan sah.
	if _, err := f.svc.HandleNotification(ctx, f.fake.Notify(o.Number, StatusPaid, 1, f.now)); !errors.Is(err, ErrAmountMismatch) {
		t.Errorf("nominal salah: %v", err)
	}
	f.fake.Notify(o.Number, StatusPending, price, f.now)
	locked("nominal salah")

	// Order tak dikenal.
	if _, err := f.svc.HandleNotification(ctx, f.fake.Notify("LVR-20260101-999999", StatusPaid, price, f.now)); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("order tak dikenal: %v", err)
	}
	// Badan rusak.
	if _, err := f.svc.HandleNotification(ctx, []byte(`{rusak`)); !errors.Is(err, ErrBadNotification) {
		t.Errorf("badan rusak: %v", err)
	}
	// Webhook "paid" bertanda tangan sah tetapi gateway tidak mengonfirmasinya saat dicek ulang.
	body := f.fake.Notify(o.Number, StatusPaid, price, f.now)
	f.fake.Notify(o.Number, StatusPending, price, f.now) // status di gateway: masih pending
	if _, err := f.svc.HandleNotification(ctx, body); !errors.Is(err, ErrUnconfirmed) {
		t.Errorf("tidak terkonfirmasi: %v", err)
	}
	locked("tidak terkonfirmasi")

	// Semua penolakan tercatat dengan alasannya.
	for _, want := range []string{"rejected:signature", "rejected:amount", "rejected:unknown-order", "rejected:invalid", "rejected:unconfirmed"} {
		if n := f.count(t, `SELECT count(*) FROM payment_events WHERE outcome = $1`, want); n == 0 {
			t.Errorf("log webhook tidak memuat %q", want)
		}
	}
}

// Pembayaran masuk di detik terakhir: order sudah disapu "expired", lalu gateway
// mengabarkan lunas → tetap dihormati (uang sudah diterima).
func TestPaidAfterLocalExpiry(t *testing.T) {
	f := newFixture(t)
	user, w := f.newWedding(t, "a@example.com")
	o := f.order(t, w, user)
	f.now = f.now.Add(25 * time.Hour)
	if sum, _ := f.svc.Summary(ctx, w.ID); sum.Latest == nil || sum.Latest.Status != StatusExpired || sum.Payable {
		t.Fatalf("summary setelah lewat batas: %+v", sum.Latest)
	}
	if out, err := f.svc.HandleNotification(ctx, f.fake.Notify(o.Number, StatusExpired, price, f.now)); err != nil || out != OutcomeApplied {
		t.Fatalf("expire: %q %v", out, err)
	}
	if out, err := f.svc.HandleNotification(ctx, f.fake.Notify(o.Number, StatusPaid, price, f.now)); err != nil || out != OutcomeApplied {
		t.Fatalf("paid setelah expired: %q %v", out, err)
	}
	if !f.wedding(t, w.ID).IsPaid() {
		t.Error("pembayaran yang masuk setelah kedaluwarsa tetap membuka publikasi")
	}
}

// Isolasi tenant: order satu wedding tidak terlihat dari wedding lain.
func TestOrdersAreTenantScoped(t *testing.T) {
	f := newFixture(t)
	ua, wa := f.newWedding(t, "a@example.com")
	_, wb := f.newWedding(t, "b@example.com")
	f.order(t, wa, ua)
	if orders, _ := f.svc.ListOrders(ctx, wb.ID); len(orders) != 0 {
		t.Error("order wedding lain bocor")
	}
	if sum, _ := f.svc.Summary(ctx, wb.ID); sum.Latest != nil {
		t.Error("summary wedding lain bocor")
	}
}

// ---------- HTTP ----------

func newServer(t *testing.T, f *fixture) *echo.Echo {
	t.Helper()
	cfg, _ := config.LoadFrom(func(k string) string {
		if k == "APP_ENV" {
			return config.EnvTest
		}
		return ""
	})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	e := server.New(cfg, log)
	loadUser := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if id, err := uuid.Parse(c.Request().Header.Get("X-Test-User")); err == nil {
				r := c.Request()
				c.SetRequest(r.WithContext(web.WithUser(r.Context(), web.User{ID: id, Name: "Sarah", Email: "s@example.com"})))
			}
			return next(c)
		}
	}
	e.Use(loadUser)
	requireUser := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, ok := web.CurrentUser(c.Request().Context()); !ok {
				return echo.NewHTTPError(http.StatusUnauthorized)
			}
			return next(c)
		}
	}
	owned := wedding.Register(e.Group("/dashboard/weddings"), wedding.Deps{Service: f.weddings})
	Register(owned, Deps{Service: f.svc, Weddings: f.weddings, Log: log}).RegisterPublic(e, requireUser)
	return e
}

func do(e *echo.Echo, user uuid.UUID, method, path string, form url.Values, hdr map[string]string) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	r := httptest.NewRequest(method, path, body)
	if form != nil {
		r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	}
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if user != uuid.Nil {
		r.Header.Set("X-Test-User", user.String())
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

// webhook: POST server-ke-server tanpa sesi & tanpa token CSRF (seperti gateway).
func webhook(e *echo.Echo, gateway string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, "/webhooks/"+gateway, strings.NewReader(string(body)))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, r)
	return rec
}

func TestPublishAndPaymentFlowViaHTTP(t *testing.T) {
	f := newFixture(t)
	e := newServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	bob, _ := f.newWedding(t, "b@example.com")
	base := w.DashboardURL("")

	// Halaman publikasi: harga, yang didapat, tombol bayar.
	page := do(e, owner, http.MethodGet, base+"/publish", nil, nil)
	for _, want := range []string{"Rp149.000", "sekali bayar", "tanpa langganan", "Custom domain", "Bayar &amp; Publikasikan"} {
		if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), want) {
			t.Errorf("halaman publikasi (%d) tidak memuat %q", page.Code, want)
		}
	}
	// Bayar → order dibuat → diarahkan ke halaman bayar gateway.
	rec := do(e, owner, http.MethodPost, base+"/payment", url.Values{}, nil)
	orders, _ := f.svc.ListOrders(ctx, w.ID)
	if rec.Code != http.StatusSeeOther || len(orders) != 1 || rec.Header().Get("Location") != orders[0].CheckoutURL {
		t.Fatalf("bayar: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	o := orders[0]
	// Order pending: "Lanjutkan pembayaran" adalah tautan langsung ke halaman
	// bayar gateway (bukan form — tidak bergantung pada redirect setelah POST).
	if page := do(e, owner, http.MethodGet, base+"/publish", nil, nil).Body.String(); !strings.Contains(page, "Lanjutkan pembayaran") || !strings.Contains(page, o.Number) ||
		!strings.Contains(page, `<a href="`+o.CheckoutURL+`"`) {
		t.Error("order pending: tautan lanjutkan pembayaran")
	}

	// Kembali dari gateway SEBELUM webhook: redirect browser tidak menandai lunas.
	ret := do(e, owner, http.MethodGet, base+"/payment/return", nil, nil).Body.String()
	if !strings.Contains(ret, "Menunggu konfirmasi") || !strings.Contains(ret, `hx-trigger="every 4s"`) || f.wedding(t, w.ID).IsPaid() {
		t.Error("halaman kembali sebelum webhook harus menunggu, bukan lunas")
	}

	// Webhook: gateway salah / tanda tangan palsu / nominal salah ditolak.
	if rec := webhook(e, "lain", f.fake.Notify(o.Number, StatusPending, price, f.now)); rec.Code != http.StatusNotFound {
		t.Errorf("gateway lain: %d", rec.Code)
	}
	if rec := webhook(e, "fake", NewFake([]byte("kunci-lain-kunci-lain-kunci-lain-1"), "").Notify(o.Number, StatusPaid, price, f.now)); rec.Code != http.StatusForbidden {
		t.Errorf("tanda tangan palsu: %d", rec.Code)
	}
	if rec := webhook(e, "fake", f.fake.Notify(o.Number, StatusPaid, 1, f.now)); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("nominal salah: %d", rec.Code)
	}
	// Order tak dikenal dengan tanda tangan sah (mis. "Test notification" dari
	// dashboard gateway): 200 supaya tidak dikirim ulang, tanpa efek apa pun.
	if rec := webhook(e, "fake", f.fake.Notify("LVR-20260101-000000", StatusPaid, price, f.now)); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "unknown-order") {
		t.Errorf("order tak dikenal: %d %s", rec.Code, rec.Body.String())
	}
	if f.wedding(t, w.ID).IsPaid() {
		t.Fatal("webhook yang ditolak tidak boleh membuka publikasi")
	}
	// Webhook sah (tanpa cookie & tanpa token CSRF) → lunas; kiriman ulang tetap 200.
	body := f.fake.Notify(o.Number, StatusPaid, price, f.now)
	for i, want := range []string{OutcomeApplied, OutcomeDuplicate} {
		if rec := webhook(e, "fake", body); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("webhook ke-%d: %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	if !f.wedding(t, w.ID).IsPaid() {
		t.Fatal("webhook sah harus membuka publikasi")
	}

	// Setelah lunas: halaman kembali & publikasi menampilkan Lunas + tombol Publikasikan.
	if ret := do(e, owner, http.MethodGet, base+"/payment/return", nil, nil).Body.String(); !strings.Contains(ret, "Pembayaran berhasil") || strings.Contains(ret, "hx-trigger") {
		t.Error("halaman kembali setelah lunas")
	}
	page2 := do(e, owner, http.MethodGet, base+"/publish", nil, nil).Body.String()
	if !strings.Contains(page2, "Lunas ✓") || !strings.Contains(page2, `name="status" value="published"`) || strings.Contains(page2, "Bayar &amp; Publikasikan") {
		t.Error("halaman publikasi setelah lunas")
	}
	// Bayar lagi setelah lunas → tidak ada order baru.
	if rec := do(e, owner, http.MethodPost, base+"/payment", url.Values{}, nil); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != base+"/publish" {
		t.Errorf("bayar setelah lunas: %d", rec.Code)
	}
	if orders, _ := f.svc.ListOrders(ctx, w.ID); len(orders) != 1 {
		t.Errorf("order = %d, want 1", len(orders))
	}
	// Publikasikan → terbit.
	rec = do(e, owner, http.MethodPost, base+"/status", url.Values{"_method": {"PATCH"}, "status": {"published"}}, nil)
	if rec.Code != http.StatusSeeOther || f.wedding(t, w.ID).Status != wedding.StatusPublished {
		t.Errorf("publikasi setelah lunas: %d %s", rec.Code, f.wedding(t, w.ID).Status)
	}

	// User lain tidak bisa melihat / membayar wedding ini.
	for _, r := range []struct{ method, path string }{
		{http.MethodGet, base + "/publish"}, {http.MethodPost, base + "/payment"}, {http.MethodGet, base + "/payment/return"},
	} {
		if rec := do(e, bob, r.method, r.path, url.Values{}, nil); rec.Code != http.StatusNotFound {
			t.Errorf("bob %s %s: %d", r.method, r.path, rec.Code)
		}
	}
}

func TestFakeCheckoutPages(t *testing.T) {
	f := newFixture(t)
	e := newServer(t, f)
	owner, w := f.newWedding(t, "a@example.com")
	bob, _ := f.newWedding(t, "b@example.com")
	o := f.order(t, w, owner)
	path := FakeCheckoutPath + o.Number

	if rec := do(e, owner, http.MethodGet, path, nil, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Mode simulasi") || !strings.Contains(rec.Body.String(), "Rp149.000") {
		t.Fatalf("halaman simulasi: %d", rec.Code)
	}
	// Hanya pemilik order; tanpa login ditolak.
	if rec := do(e, bob, http.MethodGet, path, nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("bob: %d", rec.Code)
	}
	if rec := do(e, uuid.Nil, http.MethodPost, path, url.Values{"result": {StatusPaid}}, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("tanpa login: %d", rec.Code)
	}
	if rec := do(e, owner, http.MethodPost, path, url.Values{"result": {"gratis"}}, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("hasil tak dikenal: %d", rec.Code)
	}
	// Simulasi gagal → halaman kembali menawarkan coba lagi; lalu berhasil dengan order baru.
	if rec := do(e, owner, http.MethodPost, path, url.Values{"result": {StatusFailed}}, nil); rec.Code != http.StatusSeeOther {
		t.Fatalf("simulasi gagal: %d", rec.Code)
	}
	if ret := do(e, owner, http.MethodGet, w.DashboardURL("/payment/return"), nil, nil).Body.String(); !strings.Contains(ret, "Pembayaran belum berhasil") || !strings.Contains(ret, "Coba lagi") {
		t.Error("halaman kembali setelah gagal")
	}
	o2 := f.order(t, w, owner)
	if rec := do(e, owner, http.MethodPost, FakeCheckoutPath+o2.Number, url.Values{"result": {StatusPaid}}, nil); rec.Code != http.StatusSeeOther || !f.wedding(t, w.ID).IsPaid() {
		t.Errorf("simulasi berhasil: %d", rec.Code)
	}

	// Gateway sungguhan: halaman simulasi tidak dipasang.
	real := newFixture(t)
	real.svc = NewService(real.pool, real.weddings, Config{Gateway: NewMidtrans("k", false), Price: price, Expiry: time.Hour}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	e2 := newServer(t, real)
	if rec := do(e2, owner, http.MethodGet, FakeCheckoutPath+"LVR-1", nil, nil); rec.Code != http.StatusNotFound {
		t.Errorf("simulasi dengan gateway sungguhan: %d", rec.Code)
	}
}
