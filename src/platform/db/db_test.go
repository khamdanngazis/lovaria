package db_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/db"
	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

func TestNewIDIsV7AndOrdered(t *testing.T) {
	a, b := db.NewID(), db.NewID()
	if a.Version() != 7 {
		t.Fatalf("version = %d, want 7", a.Version())
	}
	if strings.Compare(a.String(), b.String()) >= 0 {
		t.Errorf("UUIDv7 harus terurut: %s >= %s", a, b)
	}
}

func TestOpenInvalidURL(t *testing.T) {
	if _, err := db.Open(context.Background(), config.DB{URL: "://nope", MaxConns: 1}); err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckerReportsDown(t *testing.T) {
	// Port 1 hampir pasti tertutup → ping gagal → /readyz 503.
	pool, err := db.Open(context.Background(), config.DB{
		URL: "postgres://x:y@127.0.0.1:1/x", MaxConns: 1, ConnectTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Checker(pool).Check(context.Background()); err == nil {
		t.Fatal("expected ping error")
	}
}

func TestCheckerHealthy(t *testing.T) {
	pool := dbtest.Pool(t)
	if err := db.Checker(pool).Check(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestMigrationsRoundTrip(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	var out bytes.Buffer

	// Turunkan semua migration, lalu naikkan lagi: memastikan setiap Down bersih.
	for {
		var v int64
		if err := pool.QueryRow(ctx, "SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied").Scan(&v); err != nil {
			t.Fatal(err)
		}
		if v == 0 {
			break
		}
		if err := db.Migrate(ctx, pool, "down", &out); err != nil {
			t.Fatalf("down: %v\n%s", err, out.String())
		}
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_proc WHERE proname = 'set_updated_at'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("set_updated_at masih ada setelah down (n=%d, err=%v)", n, err)
	}

	if err := db.Migrate(ctx, pool, "up", &out); err != nil {
		t.Fatalf("up: %v\n%s", err, out.String())
	}
	for _, cmd := range []string{"status", "version"} {
		if err := db.Migrate(ctx, pool, cmd, &out); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}
	if err := db.Migrate(ctx, pool, "sideways", &out); err == nil {
		t.Error("perintah tidak dikenal harus error")
	}
}

func TestConventionTableWithTrigger(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	// Tabel contoh mengikuti konvensi doc/database.md.
	_, err := pool.Exec(ctx, `
		CREATE TABLE sample (
			id         uuid PRIMARY KEY,
			email      citext NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			updated_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE TRIGGER set_updated_at BEFORE UPDATE ON sample
			FOR EACH ROW EXECUTE FUNCTION set_updated_at();`)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS sample`) })

	id := db.NewID()
	if _, err := pool.Exec(ctx, `INSERT INTO sample (id, email, updated_at) VALUES ($1, 'A@B.com', now() - interval '1 hour')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE sample SET email = 'c@d.com' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}

	var gotID uuid.UUID
	var age time.Duration
	err = pool.QueryRow(ctx, `SELECT id, now() - updated_at FROM sample WHERE email = 'C@D.COM'`).Scan(&gotID, &age)
	if err != nil {
		t.Fatalf("citext lookup: %v", err)
	}
	if gotID != id {
		t.Errorf("id = %s, want %s", gotID, id)
	}
	if age > time.Minute {
		t.Errorf("updated_at tidak di-update trigger (umur %s)", age)
	}
}

func TestWithTx(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS tx_sample (id uuid PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DROP TABLE IF EXISTS tx_sample`) })
	insert := func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO tx_sample (id) VALUES ($1)`, db.NewID())
		return err
	}
	count := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM tx_sample`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := db.WithTx(ctx, pool, insert); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if count() != 1 {
		t.Fatal("commit tidak tersimpan")
	}

	boom := errors.New("boom")
	err := db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := insert(tx); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if count() != 1 {
		t.Fatal("rollback karena error gagal")
	}

	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic harus diteruskan")
			}
		}()
		_ = db.WithTx(ctx, pool, func(tx pgx.Tx) error {
			_ = insert(tx)
			panic("boom")
		})
	}()
	if count() != 1 {
		t.Fatal("rollback karena panic gagal")
	}

	// Nested → savepoint: error di dalam tidak membatalkan transaksi luar.
	err = db.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := insert(tx); err != nil {
			return err
		}
		_ = db.WithTx(ctx, tx, func(inner pgx.Tx) error {
			_ = insert(inner)
			return boom
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count() != 2 {
		t.Fatalf("count = %d, want 2 (savepoint)", count())
	}
}
