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
		"APP_ENV":          "production",
		"PORT":             "3000",
		"BASE_URL":         "https://lovoria.com/",
		"LOG_LEVEL":        "warn",
		"DATABASE_URL":     "postgres://x",
		"SHUTDOWN_TIMEOUT": "5s",
		"DB_MAX_CONNS":     "20",
		"DB_MIN_CONNS":     "2",
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
