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
	BaseURL string
	// LogLevel: debug | info | warn | error (LOG_LEVEL).
	LogLevel slog.Level
	// DatabaseURL connection string Postgres (DATABASE_URL). Wajib mulai T02.
	DatabaseURL string
	// ShutdownTimeout batas waktu graceful shutdown (SHUTDOWN_TIMEOUT, format Go duration).
	ShutdownTimeout time.Duration
	// StaticFromDisk: true → /static dibaca dari folder ./static (hot reload saat dev);
	// false → dari file yang di-embed ke binary (STATIC_FROM_DISK).
	StaticFromDisk bool
}

func (c Config) IsDevelopment() bool { return c.Env == EnvDevelopment }
func (c Config) IsProduction() bool  { return c.Env == EnvProduction }

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

	var errs []error
	cfg := Config{
		Env:         get("APP_ENV", EnvDevelopment),
		BaseURL:     strings.TrimRight(get("BASE_URL", "http://localhost:8080"), "/"),
		DatabaseURL: get("DATABASE_URL", ""),
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

	if err := errors.Join(errs...); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}
