package payment

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/web"
)

// ChecklistSource adalah bagian service wedding untuk halaman publikasi.
type ChecklistSource interface {
	Checklist(ctx context.Context, weddingID uuid.UUID) ([]wedding.ChecklistItem, error)
	GetWedding(ctx context.Context, weddingID uuid.UUID) (wedding.Wedding, error)
}

type Handler struct {
	svc      *Service
	weddings ChecklistSource
	log      *slog.Logger
}

func ctxWedding(c echo.Context) wedding.Wedding {
	w, ok := wedding.FromContext(c.Request().Context())
	if !ok {
		panic("payment: route dipasang tanpa RequireWeddingOwner")
	}
	return w
}

func (h *Handler) state(c echo.Context, w wedding.Wedding) (publishState, error) {
	ctx := c.Request().Context()
	sum, err := h.svc.Summary(ctx, w.ID)
	if err != nil {
		return publishState{}, err
	}
	items, err := h.weddings.Checklist(ctx, w.ID)
	if err != nil {
		return publishState{}, err
	}
	orders, err := h.svc.ListOrders(ctx, w.ID)
	if err != nil {
		return publishState{}, err
	}
	return publishState{W: w, Price: h.svc.Price(), Enabled: h.svc.Enabled(), Summary: sum, Checklist: items, Orders: orders}, nil
}

// GET …/publish — konfirmasi publikasi: harga, yang didapat, status pembayaran.
func (h *Handler) PublishPage(c echo.Context) error {
	s, err := h.state(c, ctxWedding(c))
	if err != nil {
		return err
	}
	return web.Render(c, http.StatusOK, publishPage(s))
}

// POST …/payment — buat (atau pakai ulang) order lalu arahkan ke halaman bayar gateway.
func (h *Handler) Pay(c echo.Context) error {
	w := ctxWedding(c)
	u, ok := web.CurrentUser(c.Request().Context())
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized)
	}
	o, err := h.svc.CreateOrder(c.Request().Context(), w, u.ID, Customer{Name: u.Name, Email: u.Email})
	switch {
	case errors.Is(err, ErrAlreadyPaid):
		return c.Redirect(http.StatusSeeOther, w.DashboardURL("/publish"))
	case err != nil:
		if !errors.Is(err, ErrUnavailable) {
			h.log.ErrorContext(c.Request().Context(), "payment: buat order", slog.String("wedding_id", w.ID.String()), slog.String("error", err.Error()))
		}
		s, serr := h.state(c, w)
		if serr != nil {
			return serr
		}
		s.Error = "Pembayaran belum bisa dimulai. Coba lagi beberapa saat lagi."
		if errors.Is(err, ErrUnavailable) {
			s.Error = "Pembayaran belum tersedia. Silakan hubungi kami."
		}
		return web.Render(c, http.StatusServiceUnavailable, publishPage(s))
	}
	return c.Redirect(http.StatusSeeOther, o.CheckoutURL)
}

// GET …/payment/return — kembali dari gateway. Redirect browser TIDAK menandai
// lunas: status hanya berubah lewat webhook / cek status ke gateway.
func (h *Handler) Return(c echo.Context) error {
	w := ctxWedding(c)
	if err := h.svc.Refresh(c.Request().Context(), w.ID); err != nil {
		h.log.WarnContext(c.Request().Context(), "payment: cek status", slog.String("error", err.Error()))
	}
	// Status lunas ada di wedding: baca ulang setelah Refresh.
	w, err := h.weddings.GetWedding(c.Request().Context(), w.ID)
	if err != nil {
		return err
	}
	s, err := h.state(c, w)
	if err != nil {
		return err
	}
	if web.IsHTMX(c) {
		return web.Render(c, http.StatusOK, paymentStatus(s))
	}
	return web.Render(c, http.StatusOK, returnPage(s))
}

// Webhook: POST /webhooks/:gateway — notifikasi dari gateway (tanpa sesi).
func (h *Handler) Webhook(c echo.Context) error {
	if !h.svc.Enabled() || c.Param("gateway") != h.svc.GatewayName() {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 64<<10))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	outcome, err := h.svc.HandleNotification(c.Request().Context(), body)
	switch {
	case errors.Is(err, ErrBadSignature):
		return echo.NewHTTPError(http.StatusForbidden, "tanda tangan tidak valid")
	case errors.Is(err, ErrBadNotification), errors.Is(err, ErrAmountMismatch), errors.Is(err, ErrUnconfirmed):
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "notifikasi ditolak")
	case errors.Is(err, ErrOrderNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "order tidak ditemukan")
	case err != nil:
		return err // 5xx → gateway mengirim ulang
	}
	return c.JSON(http.StatusOK, map[string]string{"status": outcome})
}

// ---------- Gateway simulasi (dev/test saja) ----------

// FakeCheckout: GET /payment/simulasi/:order — halaman bayar simulasi.
func (h *Handler) FakeCheckout(c echo.Context) error {
	if _, ok := h.svc.Fake(); !ok {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	o, err := h.svc.OrderByNumber(c.Request().Context(), c.Param("order"))
	if u, ok := web.CurrentUser(c.Request().Context()); err != nil || !ok || o.UserID != u.ID {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	return web.Render(c, http.StatusOK, fakeCheckoutPage(o))
}

// FakePay: POST /payment/simulasi/:order (result=paid|failed|expired) — mengirim
// "webhook" bertanda tangan lewat jalur yang sama dengan gateway sungguhan.
func (h *Handler) FakePay(c echo.Context) error {
	f, ok := h.svc.Fake()
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	ctx := c.Request().Context()
	o, err := h.svc.OrderByNumber(ctx, c.Param("order"))
	if u, ok := web.CurrentUser(ctx); err != nil || !ok || o.UserID != u.ID {
		return echo.NewHTTPError(http.StatusNotFound)
	}
	result := c.FormValue("result")
	switch result {
	case StatusPaid, StatusFailed, StatusExpired, StatusCancelled:
	default:
		return echo.NewHTTPError(http.StatusBadRequest)
	}
	if _, err := h.svc.HandleNotification(ctx, f.Notify(o.Number, result, o.Amount, h.svc.now())); err != nil {
		return err
	}
	return c.Redirect(http.StatusSeeOther, "/dashboard/weddings/"+o.WeddingID.String()+"/payment/return")
}
