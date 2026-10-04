package config

import (
	"log/slog"
	"strings"
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
	if cfg.DB.MaxConns != 25 || cfg.DB.MinConns != 0 || cfg.DB.MaxConnLifetime != 30*time.Minute ||
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

func TestExtraHosts(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{
		"DATABASE_URL": "x", "RAILWAY_PUBLIC_DOMAIN": "lovoria.my.id",
		"EXTRA_HOSTS": " Lovaria-Production.up.railway.app. , ,old.example.com",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"lovoria.my.id", "lovaria-production.up.railway.app", "old.example.com"}
	if strings.Join(cfg.ExtraHosts, ",") != strings.Join(want, ",") {
		t.Errorf("ExtraHosts = %v", cfg.ExtraHosts)
	}
	if _, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x", "EXTRA_HOSTS": "https://x.up.railway.app"})); err == nil {
		t.Error("EXTRA_HOSTS dengan skema harus ditolak")
	}
}

func TestMailDefaultsToLog(t *testing.T) {
	cfg, err := LoadFrom(envFrom(map[string]string{"DATABASE_URL": "x"}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ArchiveAfterDays != 365 {
		t.Errorf("ArchiveAfterDays = %d", cfg.ArchiveAfterDays)
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
	// Endpoint S3 API bukan URL publik.
	r2["R2_PUBLIC_URL"] = "https://f0eda752dd7b08e24bb1ba322376c95e.r2.cloudflarestorage.com"
	if _, err := LoadFrom(envFrom(r2)); err == nil || !strings.Contains(err.Error(), "pub-xxxx.r2.dev") {
		t.Errorf("endpoint API sebagai R2_PUBLIC_URL harus ditolak: %v", err)
	}
	r2["R2_PUBLIC_URL"] = "https://pub-0123456789abcdef.r2.dev"
	if _, err := LoadFrom(envFrom(r2)); err != nil {
		t.Errorf("r2.dev harus diterima: %v", err)
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
		"noDB":      {"APP_ENV": "production"},
		"maxConns":  {"DB_MAX_CONNS": "0"},
		"minConns":  {"DB_MIN_CONNS": "30"},
		"lifetime":  {"DB_MAX_CONN_LIFETIME": "soon"},
		"env":       {"APP_ENV": "staging"},
		"port":      {"PORT": "abc"},
		"portHigh":  {"PORT": "70000"},
		"level":     {"LOG_LEVEL": "loud"},
		"timeout":   {"SHUTDOWN_TIMEOUT": "-1s"},
		"static":    {"STATIC_FROM_DISK": "maybe"},
		"mail":      {"MAIL_DRIVER": "pigeon"},
		"smtp":      {"MAIL_DRIVER": "smtp"},
		"resend":    {"MAIL_DRIVER": "resend"},
		"smtpPort":  {"SMTP_PORT": "x"},
		"storDrv":   {"STORAGE_DRIVER": "ftp"},
		"archive":   {"LIFECYCLE_ARCHIVE_DAYS": "0"},
		"secret":    {"APP_SECRET": "pendek"},
		"cfPartial": {"CLOUDFLARE_API_TOKEN": "x"},
		"quota":     {"STORAGE_QUOTA_MB": "0"},
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

func TestLoadPayment(t *testing.T) {
	// Bawaan: belum ada gateway, harga Rp149.000, masa berlaku 24 jam.
	cfg, err := LoadFrom(envFrom(map[string]string{"APP_ENV": EnvTest}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Payment.Enabled() || cfg.Payment.PriceIDR != 149000 || cfg.Payment.ExpiryHours != 24 {
		t.Errorf("bawaan = %+v", cfg.Payment)
	}
	cfg, err = LoadFrom(envFrom(map[string]string{
		"APP_ENV": EnvTest, "PAYMENT_GATEWAY": "Midtrans", "MIDTRANS_SERVER_KEY": "SB-Mid-server-x",
		"MIDTRANS_ENV": "production", "PUBLISH_PRICE_IDR": "199000", "PAYMENT_EXPIRY_HOURS": "48",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if p := cfg.Payment; p.Gateway != GatewayMidtrans || !p.MidtransProduction || p.PriceIDR != 199000 || p.ExpiryHours != 48 || !p.Enabled() {
		t.Errorf("midtrans = %+v", p)
	}
	// MIDTRANS_ENV selain "production" → sandbox.
	if cfg, _ := LoadFrom(envFrom(map[string]string{"APP_ENV": EnvTest, "PAYMENT_GATEWAY": "midtrans", "MIDTRANS_SERVER_KEY": "k", "MIDTRANS_ENV": "prod"})); cfg.Payment.MidtransProduction {
		t.Error("hanya MIDTRANS_ENV=production yang memakai endpoint produksi")
	}
	// Gateway simulasi boleh di development/test.
	if _, err := LoadFrom(envFrom(map[string]string{"APP_ENV": EnvDevelopment, "DATABASE_URL": "postgres://x", "PAYMENT_GATEWAY": "fake"})); err != nil {
		t.Errorf("fake di development: %v", err)
	}
	// …tetapi tidak pernah di production (menandai lunas tanpa uang sungguhan).
	prod := map[string]string{
		"APP_ENV": EnvProduction, "DATABASE_URL": "postgres://x", "BASE_URL": "https://lovoria.com",
		"STORAGE_DRIVER": "r2", "R2_ACCOUNT_ID": "a", "R2_ACCESS_KEY_ID": "k", "R2_SECRET_ACCESS_KEY": "s", "R2_BUCKET": "b", "R2_PUBLIC_URL": "https://m.x",
		"APP_SECRET": "rahasia-rahasia-rahasia-rahasia-12",
	}
	if _, err := LoadFrom(envFrom(prod)); err != nil {
		t.Fatalf("config production dasar harus valid: %v", err)
	}
	prod["PAYMENT_GATEWAY"] = "fake"
	if _, err := LoadFrom(envFrom(prod)); err == nil || !strings.Contains(err.Error(), "fake tidak boleh dipakai di production") {
		t.Errorf("fake di production harus ditolak: %v", err)
	}

	for name, env := range map[string]map[string]string{
		"midtrans tanpa server key": {"APP_ENV": EnvTest, "PAYMENT_GATEWAY": "midtrans"},
		"gateway tak dikenal":       {"APP_ENV": EnvTest, "PAYMENT_GATEWAY": "paypal"},
		"harga nol":                 {"APP_ENV": EnvTest, "PUBLISH_PRICE_IDR": "0"},
		"harga bukan angka":         {"APP_ENV": EnvTest, "PUBLISH_PRICE_IDR": "149rb"},
		"masa berlaku nol":          {"APP_ENV": EnvTest, "PAYMENT_EXPIRY_HOURS": "0"},
	} {
		if _, err := LoadFrom(envFrom(env)); err == nil {
			t.Errorf("%s: harus ditolak", name)
		}
	}
}
