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
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // zona waktu wedding (Asia/Jakarta, …) untuk file kalender

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"golang.org/x/term"

	"github.com/khamdanngazis/lovaria/src/dashboard"
	"github.com/khamdanngazis/lovaria/src/modules/admin"
	"github.com/khamdanngazis/lovaria/src/modules/auth"
	"github.com/khamdanngazis/lovaria/src/modules/domain"
	"github.com/khamdanngazis/lovaria/src/modules/example"
	"github.com/khamdanngazis/lovaria/src/modules/gallery"
	"github.com/khamdanngazis/lovaria/src/modules/gift"
	"github.com/khamdanngazis/lovaria/src/modules/guest"
	"github.com/khamdanngazis/lovaria/src/modules/guestbook"
	"github.com/khamdanngazis/lovaria/src/modules/payment"
	"github.com/khamdanngazis/lovaria/src/modules/theme"
	"github.com/khamdanngazis/lovaria/src/modules/wedding"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/event"
	"github.com/khamdanngazis/lovaria/src/modules/wedding/story"
	"github.com/khamdanngazis/lovaria/src/platform/backup"
	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/errtrack"
	"github.com/khamdanngazis/lovaria/src/platform/health"
	"github.com/khamdanngazis/lovaria/src/platform/logger"
	"github.com/khamdanngazis/lovaria/src/platform/mail"
	"github.com/khamdanngazis/lovaria/src/platform/seed"
	"github.com/khamdanngazis/lovaria/src/platform/server"
	"github.com/khamdanngazis/lovaria/src/platform/storage"
	publicsite "github.com/khamdanngazis/lovaria/src/public-site"
	"github.com/khamdanngazis/lovaria/static"
)

// version diisi saat build: -ldflags "-X main.version=<sha>".
var version = "dev"

const usage = `Usage:
  lovoria [serve]            jalankan HTTP server
  lovoria migrate <cmd>      cmd: up | down | status | version | redo
  lovoria seed               isi data contoh (development)
  lovoria media rebase-urls --from <url-lama> [--to <url-baru>] [--apply]
                             ganti basis URL foto tersimpan (default --to = R2_PUBLIC_URL; tanpa --apply hanya simulasi)
  lovoria create-admin --email <email> [--name <nama>] [--password <pw>]
                             tanpa --password: env LOVORIA_ADMIN_PASSWORD, lalu prompt stdin
  lovoria demo seed [--refresh]  buat undangan contoh per tema untuk landing page (/w/contoh-<tema>, idempoten;
                             --refresh mengganti foto demo yang sudah ada dengan versi terbaru)
  lovoria backup run         buat backup database sekarang (BACKUP_BUCKET / BACKUP_DIR)
  lovoria backup list        daftar backup, terbaru dulu
  lovoria backup restore <file> --to <database-url> [--overwrite]
                             pulihkan backup ke database lain (--overwrite bila target = DATABASE_URL)`

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
	store    storage.Storage
	auth     *auth.Service
	weddings *wedding.Service
	domains  *domain.Service
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

	a, err := newApp(ctx, cfg, log, pool)
	if err != nil {
		return err
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
	case "demo":
		if len(args) < 2 || args[1] != "seed" {
			return fmt.Errorf("demo: perintah yang tersedia: seed\n%s", usage)
		}
		// --refresh: ganti foto demo yang sudah ada dengan versi terbaru.
		return a.seedDemo(ctx, os.Stdout, slices.Contains(args[2:], "--refresh"))
	case "backup":
		return a.backupCmd(ctx, args[1:], os.Stdout)
	case "media":
		if len(args) < 2 || args[1] != "rebase-urls" {
			return fmt.Errorf("media: perintah yang tersedia: rebase-urls\n%s", usage)
		}
		return a.rebaseMediaURLs(ctx, args[2:], os.Stdout)
	default:
		return fmt.Errorf("perintah tidak dikenal %q\n%s", cmd, usage)
	}
}

// newApp menyusun dependency bersama (dipakai run & test wiring).
func newApp(ctx context.Context, cfg config.Config, log *slog.Logger, pool *pgxpool.Pool) (*app, error) {
	mailer, err := mail.New(cfg.Mail, log)
	if err != nil {
		return nil, err
	}
	store, err := storage.New(ctx, cfg.Storage)
	if err != nil {
		return nil, err
	}
	// Custom domain (T15): tanpa CLOUDFLARE_* fitur nonaktif, lookup Host tetap jalan.
	var hostnames domain.Hostnames
	if cfg.Domain.Enabled() {
		hostnames = &domain.Cloudflare{Token: cfg.Domain.CloudflareToken, ZoneID: cfg.Domain.CloudflareZoneID}
	}
	reserved := append([]string{hostOnly(cfg.BaseURL)}, cfg.ExtraHosts...)
	return &app{
		cfg:      cfg,
		log:      log,
		pool:     pool,
		store:    store,
		auth:     auth.NewService(auth.NewRepository(pool), mailer, cfg.BaseURL, log),
		weddings: wedding.NewService(wedding.NewRepository(pool)),
		domains:  domain.NewService(pool, hostnames, domain.Config{CNAMETarget: cfg.Domain.CNAMETarget, Reserved: reserved}, log),
	}, nil
}

// hostOnly: host dari URL (https://lovoria.com → lovoria.com).
func hostOnly(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
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

// rebaseMediaURLs mengganti basis URL foto yang tersimpan di DB (mis. setelah
// R2_PUBLIC_URL diperbaiki atau pindah ke custom domain). Objek di storage tidak
// berubah. Tiap modul mengubah tabelnya sendiri lewat service-nya.
func (a *app) rebaseMediaURLs(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("media rebase-urls", flag.ContinueOnError)
	from := fs.String("from", "", "basis URL lama, mis. https://<akun>.r2.cloudflarestorage.com (wajib)")
	to := fs.String("to", a.cfg.Storage.PublicURL, "basis URL baru (default: R2_PUBLIC_URL)")
	apply := fs.Bool("apply", false, "terapkan perubahan (tanpa ini hanya simulasi)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	oldPrefix, err := mediaPrefix(*from)
	if err != nil {
		return fmt.Errorf("--from: %w", err)
	}
	newPrefix, err := mediaPrefix(*to)
	if err != nil {
		return fmt.Errorf("--to: %w", err)
	}
	if oldPrefix == newPrefix {
		return errors.New("--from dan --to sama")
	}

	steps := []struct {
		name string
		fn   func(context.Context, string, string, bool) (int64, error)
	}{
		{"gallery_items", gallery.NewService(gallery.NewRepository(a.pool), a.store, a.weddings, a.cfg.Storage.QuotaBytes, a.log).RebaseMediaURLs},
		{"weddings + couples", a.weddings.RebaseMediaURLs},
		{"love_stories", story.NewService(story.NewRepository(a.pool)).RebaseMediaURLs},
	}
	mode := "SIMULASI (tidak ada yang diubah)"
	if *apply {
		mode = "DITERAPKAN"
	}
	fmt.Fprintf(out, "media rebase-urls — %s\n  dari: %s\n  ke:   %s\n", mode, oldPrefix, newPrefix)
	var total int64
	for _, st := range steps {
		n, err := st.fn(ctx, oldPrefix, newPrefix, *apply)
		if err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
		total += n
		fmt.Fprintf(out, "  %-20s %d baris\n", st.name+":", n)
	}
	if !*apply && total > 0 {
		fmt.Fprintln(out, "Jalankan ulang dengan --apply untuk menerapkan.")
	}
	return nil
}

// mediaPrefix menormalkan basis URL menjadi "https://host/path/" (selalu diakhiri
// "/", supaya https://a.com tidak ikut mencocokkan https://a.com.lain).
func mediaPrefix(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if raw == "" || err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", fmt.Errorf("URL tidak valid %q", raw)
	}
	return strings.TrimRight(u.String(), "/") + "/", nil
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
	a.log.Info("starting lovoria", slog.String("version", version), slog.String("base_url", a.cfg.BaseURL))
	static.Configure(a.cfg.StaticFromDisk, "static")
	e := a.routes() // merakit service lebih dulu (SetArchiveDaysSource dll.) sebelum scheduler jalan
	report, flush, err := errtrack.Init(a.cfg.SentryDSN, a.cfg.Env, version)
	if err != nil {
		return err
	}
	defer flush()
	server.ReportErrors(e, report)
	if a.cfg.SentryDSN != "" {
		a.log.Info("error tracking: Sentry aktif")
	}
	go a.auth.RunCleanup(ctx, time.Hour)
	go a.weddings.RunLifecycle(ctx, 10*time.Minute, a.cfg.ArchiveAfterDays, a.log)
	go a.domains.RunPolling(ctx, 5*time.Minute) // verifikasi custom domain (T15)
	if bk := a.backups(); bk != nil {
		go bk.RunDaily(ctx, a.pool, a.cfg.Backup.HourUTC)
		a.log.Info("backup harian aktif", slog.Int("hour_utc", a.cfg.Backup.HourUTC), slog.Int("retention_days", a.cfg.Backup.RetentionDays))
	}
	return server.Run(ctx, e, a.cfg, a.log)
}

// routes merakit seluruh HTTP handler aplikasi (dipakai serve & test wiring).
func (a *app) routes() *echo.Echo {
	cfg, log := a.cfg, a.log

	// --- Infrastruktur ---
	e := server.New(cfg, log)
	health.NewHandler(db.Checker(a.pool)).Register(e)
	static.Register(e)
	if local, ok := a.store.(*storage.Local); ok {
		// Hanya dev: config menolak STORAGE_DRIVER=local di production.
		log.Warn("storage: memakai disk lokal (hanya development)", slog.String("dir", cfg.Storage.LocalDir))
		local.Register(e)
	}

	authMW := auth.NewMiddleware(a.auth, cfg.CookieSecure(), log)
	e.Use(authMW.LoadSession)

	// --- Modul ---
	auth.Register(e, auth.Deps{Service: a.auth, Middleware: authMW})
	if cfg.IsDevelopment() {
		example.Register(e.Group("/_example"), example.Deps{
			Service: example.NewService(example.NewMemoryRepository()),
		})
	}
	secret := appSecret(cfg, log)

	// Service modul (dibuat dulu: beranda dashboard mengagregasi semuanya).
	events := event.NewService(event.NewRepository(a.pool))
	a.weddings.SetEventCounter(events) // checklist publikasi
	stories := story.NewService(story.NewRepository(a.pool))
	photos := gallery.NewService(gallery.NewRepository(a.pool), a.store, a.weddings, cfg.Storage.QuotaBytes, log)
	themes := theme.NewService(a.pool, a.weddings)
	guests := guest.NewService(guest.NewRepository(a.pool), cfg.BaseURL)
	guestbooks := guestbook.NewService(a.pool, guestbook.NewWordFilter(slices.Concat(guestbook.DefaultBlockedWords, cfg.GuestbookBlockedWords)))
	gifts := gift.NewService(a.pool)
	a.weddings.SetDomains(a.domains, cfg.BaseURL) // URL kanonik (custom domain aktif)
	views := &publicsite.ViewBuilder{Weddings: a.weddings, Events: events, Stories: stories, Gallery: photos, Themes: themes, Guestbook: guestbooks, Gifts: gifts}
	guestbooks.OnChange(views.Invalidate) // favorit/sembunyikan ucapan langsung terlihat di halaman publik (T19)
	themes.SetMusicStore(photos)          // musik latar: pustaka R2 music/ + unggahan (kuota sama dengan foto, T20)
	themes.OnChange(views.Invalidate)     // pengaturan tampilan langsung terlihat di halaman publik
	home := &dashboard.Home{
		Weddings: a.weddings, Events: events, Stories: stories, Gallery: photos, Themes: themes,
		Guests: guests, Guestbook: guestbooks,
	}

	dash := e.Group("/dashboard", server.NoStore, authMW.RequireAuth)
	dashboard.Register(dash, dashboard.Deps{Weddings: a.weddings})
	owned := wedding.Register(dash.Group("/weddings"), wedding.Deps{Service: a.weddings, ArchiveDays: cfg.ArchiveAfterDays, Home: home.Widgets})
	event.Register(owned, event.Deps{Service: events})
	story.Register(owned, story.Deps{Service: stories})
	gallery.Register(owned, gallery.Deps{Service: photos, Weddings: a.weddings})
	theme.Register(owned, theme.Deps{Service: themes, Previewer: views})
	guests.SetOrigins(a.weddings.CanonicalOrigin) // link tamu memakai custom domain bila aktif
	guest.Register(owned, guest.Deps{Service: guests, Weddings: a.weddings})
	guestbook.Register(owned, guestbook.Deps{Service: guestbooks})
	gift.Register(owned, gift.Deps{Service: gifts})
	keepsake := &dashboard.Keepsake{Weddings: a.weddings, Stories: stories, Guests: guests, Guestbook: guestbooks, Photo: dashboard.HTTPPhoto(cfg.BaseURL)}
	keepsake.Register(owned) // PDF kenang-kenangan (T19)
	domain.Register(owned, domain.Deps{Service: a.domains})
	// Pembayaran publikasi (T23): halaman harga & bayar di dashboard, webhook
	// gateway di luar dashboard (tanpa sesi; keasliannya dijamin tanda tangan).
	payments := payment.NewService(a.pool, a.weddings, payment.Config{
		Gateway: paymentGateway(cfg, secret), Price: cfg.Payment.PriceIDR,
		Expiry: time.Duration(cfg.Payment.ExpiryHours) * time.Hour, BaseURL: cfg.BaseURL,
	}, log)
	payment.Register(owned, payment.Deps{Service: payments, Weddings: a.weddings, Log: log}).RegisterPublic(e, authMW.RequireAuth)
	// Panel admin (T16): data modul lain lewat service-nya; paket mengatur kuota
	// storage & lama arsip; admin bisa melihat dashboard pasangan (lihat saja).
	admins := admin.NewService(admin.Deps{
		Pool: a.pool, Users: a.auth, Weddings: a.weddings, Guests: guests, Gallery: photos, Domains: a.domains, Themes: themes,
		Payments: payments, Secret: secret, CookieSecure: cfg.CookieSecure(), Log: log,
	})
	a.weddings.SetAdminAccess(admins)
	a.weddings.SetArchiveDaysSource(admins.ArchiveDays)
	photos.SetQuotaSource(admins.QuotaBytes)
	admin.Register(e.Group("/admin", server.NoStore, authMW.RequireAuth, authMW.RequireRole(auth.RoleAdmin)), admins)
	publicsite.Register(e, publicsite.Deps{
		Resolver: &publicsite.Resolver{Weddings: a.weddings, Guests: guests, Domains: a.domains, BaseURL: cfg.BaseURL, ExtraHosts: cfg.ExtraHosts, HostHeader: cfg.Domain.HostHeader, Log: log},
		Handler:  &publicsite.Handler{BaseURL: cfg.BaseURL, PublishPrice: cfg.Payment.PriceIDR, Views: views, Guests: guests, Guestbook: guestbooks, Events: events, Log: log, Secret: secret},
	})
	return e
}

// paymentGateway memilih gateway dari config; nil = pembayaran belum tersedia
// (wedding baru belum bisa terbit sampai PAYMENT_GATEWAY diisi).
func paymentGateway(cfg config.Config, secret []byte) payment.Gateway {
	switch cfg.Payment.Gateway {
	case config.GatewayMidtrans:
		return payment.NewMidtrans(cfg.Payment.MidtransServerKey, cfg.Payment.MidtransProduction)
	case config.GatewayFake:
		return payment.NewFake(secret, cfg.BaseURL)
	}
	return nil
}

// appSecret: APP_SECRET, atau kunci acak per proses bila kosong (token form
// RSVP yang sudah dirender jadi tidak berlaku setelah restart).
func appSecret(cfg config.Config, log *slog.Logger) []byte {
	if cfg.Secret != "" {
		return []byte(cfg.Secret)
	}
	if cfg.IsProduction() {
		log.Warn("APP_SECRET kosong: memakai kunci acak (isi APP_SECRET supaya token form tetap berlaku setelah restart)")
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

// backups menyusun service backup dari config (nil bila tidak dikonfigurasi).
func (a *app) backups() *backup.Service {
	c := a.cfg.Backup
	if !c.Enabled() {
		return nil
	}
	var store backup.Store = backup.DirStore{Dir: c.Dir}
	if c.Bucket != "" {
		endpoint := a.cfg.Storage.R2Endpoint
		if endpoint == "" {
			endpoint = "https://" + a.cfg.Storage.R2AccountID + ".r2.cloudflarestorage.com"
		}
		store = backup.NewR2Store(endpoint, c.Bucket, a.cfg.Storage.R2AccessKeyID, a.cfg.Storage.R2SecretAccessKey)
	}
	return &backup.Service{
		Store: store, DBURL: a.cfg.DB.URL, Retention: time.Duration(c.RetentionDays) * 24 * time.Hour,
		Prefix: "lovoria-" + a.cfg.Env, PgDump: c.PgDump, PgRestore: c.PgRestore, Log: a.log,
	}
}

// backupCmd: lovoria backup run | list | restore <file> --to <url> [--overwrite].
func (a *app) backupCmd(ctx context.Context, args []string, out io.Writer) error {
	bk := a.backups()
	if bk == nil {
		return errors.New("backup: isi BACKUP_BUCKET (R2 privat) atau BACKUP_DIR")
	}
	if len(args) == 0 {
		return fmt.Errorf("backup: butuh perintah run | list | restore\n%s", usage)
	}
	switch args[0] {
	case "run":
		key, err := bk.Run(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "backup dibuat:", key)
		return nil
	case "list":
		objs, err := bk.Store.List(ctx)
		if err != nil {
			return err
		}
		for _, o := range objs {
			fmt.Fprintf(out, "%s\t%d KB\t%s\n", o.Key, (o.Size+1023)/1024, o.Created.UTC().Format(time.RFC3339))
		}
		if len(objs) == 0 {
			fmt.Fprintln(out, "(belum ada backup)")
		}
		return nil
	case "restore":
		fs := flag.NewFlagSet("backup restore", flag.ContinueOnError)
		to := fs.String("to", "", "URL database tujuan")
		overwrite := fs.Bool("overwrite", false, "izinkan menimpa DATABASE_URL aplikasi")
		if len(args) < 2 {
			return errors.New("backup restore: butuh nama file (lihat: lovoria backup list)")
		}
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if err := bk.Restore(ctx, args[1], *to, *overwrite); err != nil {
			return err
		}
		fmt.Fprintln(out, "restore selesai:", args[1])
		return nil
	default:
		return fmt.Errorf("backup: perintah tidak dikenal %q (run | list | restore)", args[0])
	}
}
