// Package dbtest adalah harness integration test Postgres.
//
// Setiap test package mendapat database baru (lovoria_test_<acak>) yang sudah
// di-migrate, lalu di-drop setelah package selesai. Pakai:
//
//	func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }
//
//	func TestSesuatu(t *testing.T) {
//		pool := dbtest.Pool(t) // skip bila DATABASE_URL_TEST kosong
//		dbtest.Reset(t, pool)  // kosongkan semua tabel (opsional)
//	}
//
// DATABASE_URL_TEST harus berupa URL postgres:// milik user yang boleh CREATE DATABASE.
// Set LOVORIA_REQUIRE_DB_TESTS=true (di CI) supaya test gagal alih-alih di-skip.
package dbtest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"
)

const (
	envURL      = "DATABASE_URL_TEST"
	envRequired = "LOVORIA_REQUIRE_DB_TESTS"
)

var (
	pool    *pgxpool.Pool
	dbURL   string
	skipMsg = "DATABASE_URL_TEST tidak di-set; integration test di-skip"
)

// Main membuat database test untuk package ini, menjalankan test, lalu men-drop-nya.
// Kembalikan hasilnya ke os.Exit dari TestMain.
func Main(m *testing.M) int {
	adminURL := os.Getenv(envURL)
	if adminURL == "" {
		return m.Run()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	name, err := createDatabase(ctx, adminURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbtest:", err)
		return 1
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := dropDatabase(ctx, adminURL, name); err != nil {
			fmt.Fprintln(os.Stderr, "dbtest:", err)
		}
	}()

	dbURL, err = withDatabase(adminURL, name)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbtest:", err)
		return 1
	}
	pool, err = pgxpool.New(ctx, dbURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbtest: open pool:", err)
		return 1
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, "up", io.Discard); err != nil {
		fmt.Fprintln(os.Stderr, "dbtest:", err)
		return 1
	}
	return m.Run()
}

// Pool mengembalikan pool ke database test package ini. Test di-skip bila
// DATABASE_URL_TEST tidak di-set (atau gagal bila LOVORIA_REQUIRE_DB_TESTS=true).
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if pool == nil {
		if os.Getenv(envURL) == "" && os.Getenv(envRequired) != "true" {
			t.Skip(skipMsg)
		}
		t.Fatal("dbtest: pool belum siap — apakah TestMain memanggil dbtest.Main?")
	}
	return pool
}

// URL mengembalikan connection string database test package ini.
func URL(t testing.TB) string {
	t.Helper()
	Pool(t)
	return dbURL
}

// Reset mengosongkan semua tabel di schema public (kecuali tabel versi goose).
func Reset(t testing.TB, p *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	rows, err := p.Query(ctx, `
		SELECT quote_ident(tablename) FROM pg_tables
		WHERE schemaname = 'public' AND tablename <> 'goose_db_version'`)
	if err != nil {
		t.Fatalf("dbtest: list tables: %v", err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("dbtest: list tables: %v", err)
	}
	if len(tables) == 0 {
		return
	}
	if _, err := p.Exec(ctx, "TRUNCATE "+strings.Join(tables, ", ")+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("dbtest: truncate: %v", err)
	}
}

func createDatabase(ctx context.Context, adminURL string) (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	name := "lovoria_test_" + hex.EncodeToString(b)

	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", fmt.Errorf("connect %s: %w", envURL, err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		return "", fmt.Errorf("create database: %w", err)
	}
	return name, nil
}

func dropDatabase(ctx context.Context, adminURL, name string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
	return err
}

func withDatabase(rawURL, name string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return "", fmt.Errorf("%s harus berupa URL postgres://", envURL)
	}
	u.Path = "/" + name
	return u.String(), nil
}
