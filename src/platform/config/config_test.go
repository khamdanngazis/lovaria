package config

import (
	"log/slog"
	"testing"
	"time"
)

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "postgres://x"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Env != EnvDevelopment || cfg.Port != 8080 || cfg.LogLevel != slog.LevelInfo {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
	if !cfg.StaticFromDisk {
		t.Error("StaticFromDisk harus true di development")
	}
	if cfg.DB.MaxConns != 10 || cfg.DB.MinConns != 0 || cfg.DB.MaxConnLifetime != 30*time.Minute ||
		cfg.DB.MaxConnIdleTime != 5*time.Minute || cfg.DB.ConnectTimeout != 5*time.Second {
		t.Errorf("unexpected DB defaults: %+v", cfg.DB)
	}
	if cfg.Addr() != ":8080" {
		t.Errorf("Addr = %q", cfg.Addr())
	}
}

func TestLoadProduction(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{
		"STORAGE_DRIVER":       "r2",
		"R2_ACCOUNT_ID":        "a",
		"R2_ACCESS_KEY_ID":     "k",
		"R2_SECRET_ACCESS_KEY": "s",
		"R2_BUCKET":            "b",
		"R2_PUBLIC_URL":        "https://m.x",
		"APP_ENV":              "production",
		"PORT":                 "3000",
		"BASE_URL":             "https://lovoria.com/",
		"LOG_LEVEL":            "warn",
		"DATABASE_URL":         "postgres://x",
		"SHUTDOWN_TIMEOUT":     "5s",
		"DB_MAX_CONNS":         "20",
		"DB_MIN_CONNS":         "2",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsProduction() || cfg.Port != 3000 || cfg.LogLevel != slog.LevelWarn {
		t.Errorf("unexpected config: %+v", cfg)
	}
	if cfg.BaseURL != "https://lovoria.com" {
		t.Errorf("BaseURL harus tanpa trailing slash, dapat %q", cfg.BaseURL)
	}
	if cfg.StaticFromDisk {
		t.Error("StaticFromDisk harus false di production")
	}
	if cfg.DB.URL != "postgres://x" || cfg.DB.MaxConns != 20 || cfg.DB.MinConns != 2 || cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("unexpected config: %+v", cfg)
	}
}

func TestBaseURLFromRailway(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x", "RAILWAY_PUBLIC_DOMAIN": "app.up.railway.app"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://app.up.railway.app" || !cfg.CookieSecure() {
		t.Errorf("BaseURL = %q, CookieSecure = %v", cfg.BaseURL, cfg.CookieSecure())
	}
	cfg, _ = LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x", "RAILWAY_PUBLIC_DOMAIN": "a", "BASE_URL": "https://lovoria.com"}))
	if cfg.BaseURL != "https://lovoria.com" {
		t.Errorf("BASE_URL harus menang, dapat %q", cfg.BaseURL)
	}
}

func TestMailDefaultsToLog(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mail.Driver != "log" || cfg.Mail.SMTPPort != 587 || cfg.CookieSecure() {
		t.Errorf("unexpected: %+v secure=%v", cfg.Mail, cfg.CookieSecure())
	}
}

func TestStorageConfig(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.Driver != "local" || cfg.Storage.QuotaBytes != 500*1024*1024 {
		t.Errorf("default storage = %+v", cfg.Storage)
	}

	r2 := map[string]string{
		"DATABASE_URL": "x", "APP_ENV": "production", "STORAGE_DRIVER": "r2",
		"R2_ACCOUNT_ID": "acc", "R2_ACCESS_KEY_ID": "k", "R2_SECRET_ACCESS_KEY": "s",
		"R2_BUCKET": "b", "R2_PUBLIC_URL": "https://media.lovoria.com/", "STORAGE_QUOTA_MB": "100",
	}
	cfg, err = LoadFrom(envFrom(r2))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Storage.PublicURL != "https://media.lovoria.com" || cfg.Storage.QuotaBytes != 100*1024*1024 {
		t.Errorf("r2 = %+v", cfg.Storage)
	}

	// Production tanpa R2 (driver local) harus gagal: tidak ada file di disk Railway.
	if _, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x", "APP_ENV": "production"})); err == nil {
		t.Error("production + STORAGE_DRIVER=local harus error")
	}
	delete(r2, "R2_BUCKET")
	if _, err := LoadFrom(envFrom(r2)); err == nil {
		t.Error("r2 tanpa bucket harus error")
	}
}

func TestDatabaseURLOptionalInTest(t *testing.T) {
	if _, err := LoadFrom(envFrom(map[string]string{"APP_ENV": "test"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInvalid(t *testing.T) {
	cases := map[string]map[string]string{
		"noDB":     {"APP_ENV": "production"},
		"maxConns": {"DB_MAX_CONNS": "0"},
		"minConns": {"DB_MIN_CONNS": "20"},
		"lifetime": {"DB_MAX_CONN_LIFETIME": "soon"},
		"env":      {"APP_ENV": "staging"},
		"port":     {"PORT": "abc"},
		"portHigh": {"PORT": "70000"},
		"level":    {"LOG_LEVEL": "loud"},
		"timeout":  {"SHUTDOWN_TIMEOUT": "-1s"},
		"static":   {"STATIC_FROM_DISK": "maybe"},
		"mail":     {"MAIL_DRIVER": "pigeon"},
		"smtp":     {"MAIL_DRIVER": "smtp"},
		"resend":   {"MAIL_DRIVER": "resend"},
		"smtpPort": {"SMTP_PORT": "x"},
		"storDrv":  {"STORAGE_DRIVER": "ftp"},
		"quota":    {"STORAGE_QUOTA_MB": "0"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			full := map[string]string{"DATABASE_URL": "postgres://x"}
			if name == "noDB" {
				full = map[string]string{}
			}
			for k, v := range env {
				full[k] = v
			}
			if _, err := LoadFrom(envFrom(full)); err == nil {
				t.Error("expected error")
			}
		})
	}
}
