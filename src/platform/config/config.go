// Package config memuat seluruh konfigurasi aplikasi dari environment variable.
// Semua akses ke env var harus lewat Load(); jangan panggil os.Getenv di modul lain.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvDevelopment = "development"
	EnvProduction  = "production"
	EnvTest        = "test"
)

type Config struct {
	// Env: development | production | test (APP_ENV).
	Env string
	// Port tempat HTTP server listen (PORT; Railway mengisi ini otomatis).
	Port int
	// BaseURL adalah URL publik utama aplikasi, mis. https://lovoria.com (BASE_URL).
	// Bila kosong dan berjalan di Railway, diambil dari RAILWAY_PUBLIC_DOMAIN.
	BaseURL string
	// LogLevel: debug | info | warn | error (LOG_LEVEL).
	LogLevel slog.Level
	// DB berisi koneksi & setelan pool Postgres.
	DB DB
	// Mail berisi setelan pengiriman email.
	Mail Mail
	// ShutdownTimeout batas waktu graceful shutdown (SHUTDOWN_TIMEOUT, format Go duration).
	ShutdownTimeout time.Duration
	// StaticFromDisk: true → /static dibaca dari folder ./static (hot reload saat dev);
	// false → dari file yang di-embed ke binary (STATIC_FROM_DISK).
	StaticFromDisk bool
}

// DB adalah setelan koneksi Postgres (pgxpool).
type DB struct {
	// URL connection string Postgres (DATABASE_URL). Wajib kecuali APP_ENV=test.
	URL string
	// MaxConns jumlah koneksi maksimum di pool (DB_MAX_CONNS).
	MaxConns int32
	// MinConns jumlah koneksi yang dijaga tetap terbuka (DB_MIN_CONNS).
	MinConns int32
	// MaxConnLifetime umur maksimum satu koneksi (DB_MAX_CONN_LIFETIME).
	MaxConnLifetime time.Duration
	// MaxConnIdleTime lama koneksi idle sebelum ditutup (DB_MAX_CONN_IDLE_TIME).
	MaxConnIdleTime time.Duration
	// ConnectTimeout batas waktu membuka koneksi baru (DB_CONNECT_TIMEOUT).
	ConnectTimeout time.Duration
}

// Mail adalah setelan pengirim email (lihat src/platform/mail).
type Mail struct {
	// Driver: log | smtp | resend (MAIL_DRIVER). "log" hanya menulis email ke log.
	Driver string
	// From alamat pengirim, mis. "Lovoria <no-reply@lovoria.com>" (MAIL_FROM).
	From string
	// SMTP (SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD). Port 587 + STARTTLS.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	// ResendAPIKey untuk driver resend (RESEND_API_KEY).
	ResendAPIKey string
}

func (c Config) IsDevelopment() bool { return c.Env == EnvDevelopment }
func (c Config) IsProduction() bool  { return c.Env == EnvProduction }

// CookieSecure true bila cookie harus diberi atribut Secure (HTTPS).
func (c Config) CookieSecure() bool {
	return c.IsProduction() || strings.HasPrefix(c.BaseURL, "https://")
}

// Addr mengembalikan alamat listen HTTP server.
func (c Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }

// Load membaca konfigurasi dari environment proses.
func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

// LoadFrom membaca konfigurasi dari fungsi getenv yang diberikan (memudahkan test).
func LoadFrom(getenv func(string) string) (Config, error) {
	get := func(key, def string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return def
	}

	defaultBaseURL := "http://localhost:8080"
	if d := strings.TrimSpace(getenv("RAILWAY_PUBLIC_DOMAIN")); d != "" {
		defaultBaseURL = "https://" + d
	}

	var errs []error
	cfg := Config{
		Env:     get("APP_ENV", EnvDevelopment),
		BaseURL: strings.TrimRight(get("BASE_URL", defaultBaseURL), "/"),
		DB:      DB{URL: get("DATABASE_URL", "")},
		Mail: Mail{
			Driver:       get("MAIL_DRIVER", "log"),
			From:         get("MAIL_FROM", "Lovoria <no-reply@lovoria.local>"),
			SMTPHost:     get("SMTP_HOST", ""),
			SMTPUsername: get("SMTP_USERNAME", ""),
			SMTPPassword: getenv("SMTP_PASSWORD"),
			ResendAPIKey: get("RESEND_API_KEY", ""),
		},
	}

	switch cfg.Env {
	case EnvDevelopment, EnvProduction, EnvTest:
	default:
		errs = append(errs, fmt.Errorf("APP_ENV: nilai tidak valid %q", cfg.Env))
	}

	port, err := strconv.Atoi(get("PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("PORT: nilai tidak valid %q", getenv("PORT")))
	}
	cfg.Port = port

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
	}

	cfg.ShutdownTimeout, err = time.ParseDuration(get("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil || cfg.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: nilai tidak valid %q", getenv("SHUTDOWN_TIMEOUT")))
	}

	cfg.StaticFromDisk, err = strconv.ParseBool(get("STATIC_FROM_DISK", strconv.FormatBool(cfg.IsDevelopment())))
	if err != nil {
		errs = append(errs, fmt.Errorf("STATIC_FROM_DISK: %w", err))
	}

	cfg.Mail.SMTPPort, err = strconv.Atoi(get("SMTP_PORT", "587"))
	if err != nil {
		errs = append(errs, fmt.Errorf("SMTP_PORT: nilai tidak valid %q", getenv("SMTP_PORT")))
	}
	switch cfg.Mail.Driver {
	case "log":
	case "smtp":
		if cfg.Mail.SMTPHost == "" {
			errs = append(errs, errors.New("SMTP_HOST: wajib diisi untuk MAIL_DRIVER=smtp"))
		}
	case "resend":
		if cfg.Mail.ResendAPIKey == "" {
			errs = append(errs, errors.New("RESEND_API_KEY: wajib diisi untuk MAIL_DRIVER=resend"))
		}
	default:
		errs = append(errs, fmt.Errorf("MAIL_DRIVER: nilai tidak valid %q (log | smtp | resend)", cfg.Mail.Driver))
	}

	if cfg.DB.URL == "" && cfg.Env != EnvTest {
		errs = append(errs, errors.New("DATABASE_URL: wajib diisi"))
	}
	durations := []struct {
		key string
		def string
		dst *time.Duration
	}{
		{"DB_MAX_CONN_LIFETIME", "30m", &cfg.DB.MaxConnLifetime},
		{"DB_MAX_CONN_IDLE_TIME", "5m", &cfg.DB.MaxConnIdleTime},
		{"DB_CONNECT_TIMEOUT", "5s", &cfg.DB.ConnectTimeout},
	}
	for _, d := range durations {
		v, err := time.ParseDuration(get(d.key, d.def))
		if err != nil || v <= 0 {
			errs = append(errs, fmt.Errorf("%s: nilai tidak valid %q", d.key, getenv(d.key)))
		}
		*d.dst = v
	}
	conns := []struct {
		key string
		def string
		dst *int32
	}{
		{"DB_MAX_CONNS", "10", &cfg.DB.MaxConns},
		{"DB_MIN_CONNS", "0", &cfg.DB.MinConns},
	}
	for _, n := range conns {
		v, err := strconv.ParseInt(get(n.key, n.def), 10, 32)
		if err != nil || v < 0 {
			errs = append(errs, fmt.Errorf("%s: nilai tidak valid %q", n.key, getenv(n.key)))
		}
		*n.dst = int32(v)
	}
	if cfg.DB.MaxConns < 1 || cfg.DB.MinConns > cfg.DB.MaxConns {
		errs = append(errs, errors.New("DB_MAX_CONNS harus ≥ 1 dan ≥ DB_MIN_CONNS"))
	}

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
