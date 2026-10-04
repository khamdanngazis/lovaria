package payment

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/khamdanngazis/lovaria/src/platform/server"
)

type Deps struct {
	Service  *Service
	Weddings ChecklistSource
	Log      *slog.Logger
}

// Register memasang halaman publikasi & pembayaran di group
// /dashboard/weddings/:weddingID (sudah RequireAuth + RequireWeddingOwner).
func Register(owned *echo.Group, deps Deps) *Handler {
	h := &Handler{svc: deps.Service, weddings: deps.Weddings, log: deps.Log}
	owned.GET("/publish", h.PublishPage)
	owned.POST("/payment", h.Pay)
	owned.GET("/payment/return", h.Return)
	return h
}

// RegisterPublic memasang route di luar dashboard: webhook gateway (tanpa sesi,
// dikecualikan dari CSRF — keasliannya dijamin tanda tangan) dan, bila gateway
// simulasi aktif, halaman bayar simulasi (wajib login: auth = RequireAuth).
func (h *Handler) RegisterPublic(e *echo.Echo, auth echo.MiddlewareFunc) {
	e.POST(server.WebhookPrefix+":gateway", h.Webhook, middleware.BodyLimit("64K"))
	if _, ok := h.svc.Fake(); ok {
		e.GET(FakeCheckoutPath+":order", h.FakeCheckout, auth)
		e.POST(FakeCheckoutPath+":order", h.FakePay, auth)
	}
}
