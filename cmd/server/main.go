// Command lovoria menjalankan aplikasi Lovoria (modular monolith, satu binary).
// Dependency injection dilakukan manual di sini — tanpa framework DI.
//
//	lovoria                    jalankan HTTP server (sama dengan `lovoria serve`)
//	lovoria migrate <cmd>      up | down | status | version | redo
//	lovoria seed               isi data contoh (ditolak di production)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/dashboard"
	"github.com/khamdanngazis/lovaria/src/modules/example"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/health"
	"github.com/khamdanngazis/lovaria/src/platform/logger"
	"github.com/khamdanngazis/lovaria/src/platform/seed"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
	"github.com/khamdanngazis/lovaria/static"
)

// version diisi saat build: -ldflags "-X main.version=<sha>".
var version = "dev"

const usage = `Usage:
  lovoria [serve]            jalankan HTTP server
  lovoria migrate <cmd>      cmd: ` + "up | down | status | version | redo" + `
  lovoria seed               isi data contoh (development)`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	if cmd == "help" || cmd == "-h" || cmd == "--help" {
		fmt.Println(usage)
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.LogLevel, cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "serve":
		return serve(ctx, cfg, log, pool)
	case "migrate":
		if len(args) < 2 {
			return fmt.Errorf("migrate: butuh perintah (%s)", strings.Join(db.MigrateCommands, " | "))
		}
		return db.Migrate(ctx, pool, args[1], os.Stdout)
	case "seed":
		if cfg.IsProduction() {
			return errors.New("seed: ditolak di APP_ENV=production")
		}
		return seed.Run(ctx, log, seeders(pool))
	default:
		return fmt.Errorf("perintah tidak dikenal %q\n%s", cmd, usage)
	}
}

// seeders mendaftarkan seeder modul (diisi oleh task fitur).
func seeders(_ *pgxpool.Pool) []seed.Seeder {
	return nil
}

func serve(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) error {
	log.Info("starting lovoria", slog.String("version", version))

	// --- Infrastruktur ---
	static.Configure(cfg.StaticFromDisk, "static")

	// --- HTTP ---
	e := server.New(cfg, log)
	health.NewHandler(db.Checker(pool)).Register(e)
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
