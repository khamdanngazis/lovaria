// Package config memuat seluruh konfigurasi aplikasi dari environment variable.
// Semua akses ke env var harus lewat Load(); jangan panggil os.Getenv di modul lain.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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
	// Storage berisi setelan penyimpanan foto (R2 / disk lokal dev).
	Storage Storage
	// ArchiveAfterDays: wedding Kenangan diarsipkan otomatis setelah N hari
	// sejak hari H+1 (LIFECYCLE_ARCHIVE_DAYS, default 365).
	ArchiveAfterDays int
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

// Storage adalah setelan penyimpanan objek (lihat src/platform/storage).
type Storage struct {
	// Driver: local | r2 (STORAGE_DRIVER). local hanya boleh di luar production.
	Driver string
	// LocalDir folder penyimpanan driver local (STORAGE_LOCAL_DIR).
	LocalDir string
	// R2 / S3-compatible (R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET).
	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	// R2Endpoint override endpoint S3 (R2_ENDPOINT); kosong → https://<account>.r2.cloudflarestorage.com.
	R2Endpoint string
	// PublicURL basis URL publik objek, mis. https://media.lovoria.com (R2_PUBLIC_URL).
	PublicURL string
	// QuotaBytes kuota penyimpanan per wedding (STORAGE_QUOTA_MB, default 500).
	QuotaBytes int64
}

// validatePublicURL memastikan R2_PUBLIC_URL adalah alamat yang bisa dibuka
// browser (r2.dev / custom domain), bukan endpoint S3 API yang butuh tanda tangan.
func validatePublicURL(raw string) error {
	if raw == "" {
		return nil // sudah dilaporkan sebagai "wajib diisi"
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("R2_PUBLIC_URL: URL tidak valid %q", raw)
	}
	if strings.HasSuffix(strings.ToLower(u.Hostname()), ".r2.cloudflarestorage.com") {
		return errors.New("R2_PUBLIC_URL: ini endpoint S3 API (butuh tanda tangan, foto tidak bisa dibuka browser); " +
			"pakai URL Public access bucket (https://pub-xxxx.r2.dev) atau custom domain (mis. https://media.lovoria.com)")
	}
	return nil
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

	cfg.Storage = Storage{
		Driver:            get("STORAGE_DRIVER", "local"),
		LocalDir:          get("STORAGE_LOCAL_DIR", os.TempDir()+"/lovoria-media"),
		R2AccountID:       get("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     get("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getenv("R2_SECRET_ACCESS_KEY"),
		R2Bucket:          get("R2_BUCKET", ""),
		R2Endpoint:        strings.TrimRight(get("R2_ENDPOINT", ""), "/"),
		PublicURL:         strings.TrimRight(get("R2_PUBLIC_URL", ""), "/"),
	}
	quotaMB, err := strconv.ParseInt(get("STORAGE_QUOTA_MB", "500"), 10, 64)
	if err != nil || quotaMB < 1 {
		errs = append(errs, fmt.Errorf("STORAGE_QUOTA_MB: nilai tidak valid %q", getenv("STORAGE_QUOTA_MB")))
	}
	cfg.Storage.QuotaBytes = quotaMB * 1024 * 1024
	switch cfg.Storage.Driver {
	case "local":
		if cfg.IsProduction() {
			errs = append(errs, errors.New("STORAGE_DRIVER=local tidak boleh di production (foto wajib ke R2)"))
		}
	case "r2":
		for k, v := range map[string]string{
			"R2_ACCESS_KEY_ID": cfg.Storage.R2AccessKeyID, "R2_SECRET_ACCESS_KEY": cfg.Storage.R2SecretAccessKey,
			"R2_BUCKET": cfg.Storage.R2Bucket, "R2_PUBLIC_URL": cfg.Storage.PublicURL,
		} {
			if v == "" {
				errs = append(errs, fmt.Errorf("%s: wajib diisi untuk STORAGE_DRIVER=r2", k))
			}
		}
		if cfg.Storage.R2AccountID == "" && cfg.Storage.R2Endpoint == "" {
			errs = append(errs, errors.New("R2_ACCOUNT_ID atau R2_ENDPOINT: wajib diisi untuk STORAGE_DRIVER=r2"))
		}
		if err := validatePublicURL(cfg.Storage.PublicURL); err != nil {
			errs = append(errs, err)
		}
	default:
		errs = append(errs, fmt.Errorf("STORAGE_DRIVER: nilai tidak valid %q (local | r2)", cfg.Storage.Driver))
	}

	cfg.ArchiveAfterDays, err = strconv.Atoi(get("LIFECYCLE_ARCHIVE_DAYS", "365"))
	if err != nil || cfg.ArchiveAfterDays < 1 {
		errs = append(errs, fmt.Errorf("LIFECYCLE_ARCHIVE_DAYS: nilai tidak valid %q", getenv("LIFECYCLE_ARCHIVE_DAYS")))
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
