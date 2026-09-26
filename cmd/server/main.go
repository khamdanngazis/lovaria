// Command server menjalankan aplikasi Lovoria (modular monolith, satu binary).
// Dependency injection dilakukan manual di sini — tanpa framework DI.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/khamdanngazis/lovaria/src/dashboard"
	"github.com/khamdanngazis/lovaria/src/modules/example"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/health"
	"github.com/khamdanngazis/lovaria/src/platform/logger"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
	"github.com/khamdanngazis/lovaria/static"
)

// version diisi saat build: -ldflags "-X main.version=<sha>".
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.LogLevel, cfg.Env)
	log.Info("starting lovoria", slog.String("version", version))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Infrastruktur ---
	static.Configure(cfg.StaticFromDisk, "static")
	// T02: buka pool Postgres di sini dan tambahkan checker DB ke health.NewHandler.

	// --- HTTP ---
	e := server.New(cfg, log)
	health.NewHandler().Register(e)
	static.Register(e)

	// --- Modul ---
	if cfg.IsDevelopment() {
		example.Register(e.Group("/_example"), example.Deps{
			Service: example.NewService(example.NewMemoryRepository()),
		})
	}
	dashboard.Register(e.Group("/dashboard"), dashboard.Deps{})
	publicsite.Register(e, publicsite.Deps{})

	return server.Run(ctx, e, cfg, log)
}
