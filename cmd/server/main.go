// Command lovoria menjalankan aplikasi Lovoria (modular monolith, satu binary).
// Dependency injection dilakukan manual di sini — tanpa framework DI.
//
//	lovoria                    jalankan HTTP server (sama dengan `lovoria serve`)
//	lovoria migrate <cmd>      up | down | status | version | redo
//	lovoria seed               isi data contoh (ditolak di production)
//	lovoria create-admin       buat akun admin
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/term"

	"github.com/khamdanngazis/lovaria/src/dashboard"
	"github.com/khamdanngazis/lovaria/src/modules/admin"
	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/example"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/health"
	"github.com/khamdanngazis/lovaria/src/platform/logger"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/seed"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
	"github.com/khamdanngazis/lovaria/static"
)

// version diisi saat build: -ldflags "-X main.version=<sha>".
var version = "dev"

const usage = `Usage:
  lovoria [serve]            jalankan HTTP server
  lovoria migrate <cmd>      cmd: up | down | status | version | redo
  lovoria seed               isi data contoh (development)
  lovoria create-admin --email <email> [--name <nama>] [--password <pw>]
                             tanpa --password: env LOVORIA_ADMIN_PASSWORD, lalu prompt stdin`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// app berisi dependency yang dipakai bersama oleh semua subcommand.
type app struct {
	cfg      config.Config
	log      *slog.Logger
	pool     *pgxpool.Pool
	auth     *auth.Service
	weddings *wedding.Service
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

	mailer, err := mail.New(cfg.Mail, log)
	if err != nil {
		return err
	}
	a := &app{
		cfg:      cfg,
		log:      log,
		pool:     pool,
		auth:     auth.NewService(auth.NewRepository(pool), mailer, cfg.BaseURL, log),
		weddings: wedding.NewService(wedding.NewRepository(pool)),
	}

	switch cmd {
	case "serve":
		return a.serve(ctx)
	case "migrate":
		if len(args) < 2 {
			return fmt.Errorf("migrate: butuh perintah (%s)", strings.Join(db.MigrateCommands, " | "))
		}
		return db.Migrate(ctx, pool, args[1], os.Stdout)
	case "seed":
		if cfg.IsProduction() {
			return errors.New("seed: ditolak di APP_ENV=production")
		}
		return seed.Run(ctx, log, a.seeders())
	case "create-admin":
		return a.createAdmin(ctx, args[1:], os.Stdin)
	default:
		return fmt.Errorf("perintah tidak dikenal %q\n%s", cmd, usage)
	}
}

// seeders mendaftarkan seeder modul untuk `lovoria seed`.
func (a *app) seeders() []seed.Seeder {
	return []seed.Seeder{
		auth.Seeder(a.auth),
	}
}

func (a *app) createAdmin(ctx context.Context, args []string, stdin io.Reader) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	email := fs.String("email", "", "email admin (wajib)")
	name := fs.String("name", "Admin", "nama admin")
	password := fs.String("password", "", "password (kosong → env LOVORIA_ADMIN_PASSWORD, lalu stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *email == "" {
		return errors.New("create-admin: --email wajib diisi")
	}
	if *password == "" {
		*password = os.Getenv("LOVORIA_ADMIN_PASSWORD")
	}
	if *password == "" {
		pw, err := promptPassword(stdin, os.Stderr)
		if err != nil {
			return fmt.Errorf("create-admin: baca password: %w", err)
		}
		*password = pw
	}

	u, err := a.auth.CreateAdmin(ctx, auth.RegisterInput{Name: *name, Email: *email, Password: *password})
	if err != nil {
		return fmt.Errorf("create-admin: %w", err)
	}
	fmt.Printf("admin dibuat: %s (%s)\n", u.Email, u.ID)
	return nil
}

// promptPassword membaca password dari stdin. Di terminal sungguhan input tidak
// ditampilkan (x/term). Selain itu dibaca sampai \r, \n, atau EOF — `railway ssh`
// dengan perintah langsung mengirim Enter sebagai \r, bukan \n.
func promptPassword(stdin io.Reader, prompt io.Writer) (string, error) {
	fmt.Fprint(prompt, "Password: ")
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) { //nolint:gosec // G115: fd selalu kecil
		b, err := term.ReadPassword(int(f.Fd())) //nolint:gosec // G115: fd selalu kecil
		fmt.Fprintln(prompt)
		return string(b), err
	}
	return readLine(stdin)
}

// readLine membaca satu baris yang diakhiri \r, \n, atau EOF.
func readLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	var sb strings.Builder
	for {
		c, err := br.ReadByte()
		if errors.Is(err, io.EOF) {
			return sb.String(), nil
		}
		if err != nil {
			return "", err
		}
		if c == '\r' || c == '\n' {
			return sb.String(), nil
		}
		sb.WriteByte(c)
	}
}

func (a *app) serve(ctx context.Context) error {
	cfg, log := a.cfg, a.log
	log.Info("starting lovoria", slog.String("version", version), slog.String("base_url", cfg.BaseURL))

	// --- Infrastruktur ---
	static.Configure(cfg.StaticFromDisk, "static")
	go a.auth.RunCleanup(ctx, time.Hour)

	// --- HTTP ---
	e := server.New(cfg, log)
	health.NewHandler(db.Checker(a.pool)).Register(e)
	static.Register(e)

	authMW := auth.NewMiddleware(a.auth, cfg.CookieSecure(), log)
	e.Use(authMW.LoadSession)

	// --- Modul ---
	auth.Register(e, auth.Deps{Service: a.auth, Middleware: authMW})
	if cfg.IsDevelopment() {
		example.Register(e.Group("/_example"), example.Deps{
			Service: example.NewService(example.NewMemoryRepository()),
		})
	}
	dash := e.Group("/dashboard", authMW.RequireAuth)
	dashboard.Register(dash, dashboard.Deps{Weddings: a.weddings})
	wedding.Register(dash.Group("/weddings"), wedding.Deps{Service: a.weddings})
	admin.Register(e.Group("/admin", authMW.RequireAuth, authMW.RequireRole(auth.RoleAdmin)), admin.Deps{})
	publicsite.Register(e, publicsite.Deps{})

	return server.Run(ctx, e, cfg, log)
}
