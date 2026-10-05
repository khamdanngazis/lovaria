package admin

import "github.com/labstack/echo/v4"

// Register memasang route panel admin. Group wajib sudah dilindungi
// RequireAuth + RequireRole(admin) di cmd/server/main.go.
func Register(g *echo.Group, svc *Service) {
	h := &Handler{svc: svc}
	g.GET("", h.Home)
	g.GET("/users", h.Users)
	g.GET("/users/:userID", h.User)
	g.POST("/users/:userID/disabled", h.SetUserDisabled)
	g.GET("/weddings", h.Weddings)
	g.GET("/weddings/:weddingID", h.Wedding)
	g.POST("/weddings/:weddingID/status", h.SetWeddingStatus)
	g.POST("/weddings/:weddingID/package", h.AssignPackage)
	g.POST("/weddings/:weddingID/paid", h.MarkWeddingPaid)
	g.GET("/payments", h.Payments)
	g.POST("/weddings/:weddingID/view", h.StartView)
	g.POST("/view/stop", h.StopView)
	g.GET("/themes", h.Themes)
	g.POST("/themes/:themeID", h.SetThemeEnabled)
	g.GET("/packages", h.Packages)
	g.POST("/packages", h.CreatePackage)
	g.POST("/packages/:packageID", h.UpdatePackage)
	g.POST("/packages/:packageID/delete", h.DeletePackage)
	g.GET("/storage", h.Storage)
	g.GET("/domains", h.Domains)
	g.GET("/support", h.Support)
	g.POST("/support", h.SaveSupport)
	g.GET("/audit", h.Audit)
}
