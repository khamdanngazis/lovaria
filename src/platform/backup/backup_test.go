package backup

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db/dbtest"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var ctx = context.Background()

func TestPruneKeepsRetentionAndNewest(t *testing.T) {
	dir := t.TempDir()
	st := DirStore{Dir: dir}
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, age := range []int{1, 13, 15, 30} {
		key := "lovoria-test-" + strconv.Itoa(age) + ".dump"
		if err := st.Put(ctx, key, strings.NewReader("x"), 1); err != nil {
			t.Fatal(err)
		}
		mt := now.Add(-time.Duration(age) * 24 * time.Hour)
		if err := os.Chtimes(filepath.Join(dir, key), mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{Store: st, Retention: 14 * 24 * time.Hour, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return now }}
	if n, err := s.Prune(ctx); err != nil || n != 2 {
		t.Fatalf("prune: %d %v", n, err)
	}
	objs, _ := st.List(ctx)
	if len(objs) != 2 || objs[0].Key != "lovoria-test-1.dump" || objs[1].Key != "lovoria-test-13.dump" {
		t.Errorf("tersisa: %+v", objs)
	}
	// Satu-satunya backup tidak pernah dihapus walau sudah tua.
	only := DirStore{Dir: t.TempDir()}
	only.Put(ctx, "old.dump", strings.NewReader("x"), 1) //nolint:errcheck
	old := now.Add(-100 * 24 * time.Hour)
	os.Chtimes(filepath.Join(only.Dir, "old.dump"), old, old) //nolint:errcheck
	s.Store = only
	if n, _ := s.Prune(ctx); n != 0 {
		t.Error("backup terakhir tidak boleh dihapus")
	}
	for _, bad := range []string{"../x", "a/b", ".hidden", ""} {
		if err := st.Put(ctx, bad, strings.NewReader("x"), 1); err == nil {
			t.Errorf("key %q harus ditolak", bad)
		}
	}
}

// pgMajor: versi mayor pg_dump & server; test restore dilewati bila client
// lebih lama dari server (pg_dump menolak dump server yang lebih baru).
func pgMajor(t *testing.T, pool *pgxpool.Pool) (client, server int) {
	out, err := exec.Command("pg_dump", "--version").Output()
	if err != nil {
		if os.Getenv("LOVORIA_REQUIRE_PG_DUMP") == "true" {
			t.Fatal("pg_dump tidak tersedia")
		}
		t.Skip("pg_dump tidak tersedia")
	}
	m := regexp.MustCompile(`(\d+)\.`).FindStringSubmatch(string(out))
	client, _ = strconv.Atoi(m[1])
	var v string
	pool.QueryRow(ctx, "SHOW server_version_num").Scan(&v) //nolint:errcheck
	n, _ := strconv.Atoi(v)
	return client, n / 10000
}

func TestBackupAndRestoreToNewDatabase(t *testing.T) {
	pool := dbtest.Pool(t)
	if c, s := pgMajor(t, pool); c < s {
		// CI memasang postgresql-client-16 dan mewajibkan test ini jalan.
		if os.Getenv("LOVORIA_REQUIRE_PG_DUMP") == "true" {
			t.Fatalf("pg_dump %d lebih lama dari server %d", c, s)
		}
		t.Skipf("pg_dump %d lebih lama dari server %d", c, s)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS backup_probe (id int PRIMARY KEY, note text);
		TRUNCATE backup_probe; INSERT INTO backup_probe SELECT i, 'baris ' || i FROM generate_series(1, 250) i`); err != nil {
		t.Fatal(err)
	}
	s := &Service{Store: DirStore{Dir: t.TempDir()}, DBURL: dbtest.URL(t), Retention: 14 * 24 * time.Hour, Prefix: "lovoria-test",
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	key, err := s.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	target := dbtest.EmptyDatabase(t)
	if err := s.Restore(ctx, key, s.DBURL, false); err == nil {
		t.Error("restore ke DATABASE_URL aplikasi tanpa --overwrite harus ditolak")
	}
	if err := s.Restore(ctx, key, target, false); err != nil {
		t.Fatal(err)
	}
	tp, err := pgxpool.New(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	defer tp.Close()
	var n int
	var note string
	if err := tp.QueryRow(ctx, "SELECT count(*), max(note) FROM backup_probe").Scan(&n, &note); err != nil || n != 250 || note != "baris 99" {
		t.Errorf("hasil restore: %d %q %v", n, note, err)
	}
	var migrations int
	if err := tp.QueryRow(ctx, "SELECT count(*) FROM goose_db_version").Scan(&migrations); err != nil || migrations == 0 {
		t.Errorf("tabel migration ikut ter-restore: %d %v", migrations, err)
	}
}
