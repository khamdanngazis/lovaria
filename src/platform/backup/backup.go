// Package backup: dump database harian ke penyimpanan privat (bucket R2
// terpisah) dengan retensi N hari, plus restore ke database mana pun (T17).
// Memakai pg_dump / pg_restore (format custom, terkompresi) — versi client
// harus ≥ versi server Postgres.
package backup

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const keyTimeLayout = "20060102T150405Z"

type Service struct {
	Store     Store
	DBURL     string
	Retention time.Duration // file lebih tua dari ini dihapus setelah backup baru berhasil
	Prefix    string        // awal nama file, mis. "lovoria-production"
	PgDump    string        // path binary (default "pg_dump")
	PgRestore string        // path binary (default "pg_restore")
	Log       *slog.Logger
	Now       func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func bin(p, def string) string {
	if p != "" {
		return p
	}
	return def
}

// Run membuat satu backup, mengunggahnya, lalu menghapus backup kedaluwarsa.
// Mengembalikan key file yang dibuat.
func (s *Service) Run(ctx context.Context) (string, error) {
	tmp, err := os.CreateTemp("", "lovoria-backup-*.dump")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	_ = tmp.Close()

	start := s.now()
	cmd := exec.CommandContext(ctx, bin(s.PgDump, "pg_dump"), //nolint:gosec // G204: binary dari config operator
		"--format=custom", "--compress=6", "--no-owner", "--no-privileges", "--file="+tmp.Name(), "--dbname="+s.DBURL)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("backup: pg_dump: %w: %s", err, strings.TrimSpace(string(out)))
	}
	f, err := os.Open(tmp.Name())
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	key := s.Prefix + "-" + start.UTC().Format(keyTimeLayout) + ".dump"
	if err := s.Store.Put(ctx, key, f, info.Size()); err != nil {
		return "", fmt.Errorf("backup: upload %s: %w", key, err)
	}
	s.Log.InfoContext(ctx, "backup: selesai", slog.String("key", key), slog.Int64("bytes", info.Size()), slog.Duration("took", s.now().Sub(start)))
	if n, err := s.Prune(ctx); err != nil {
		s.Log.WarnContext(ctx, "backup: hapus backup lama gagal", slog.String("error", err.Error()))
	} else if n > 0 {
		s.Log.InfoContext(ctx, "backup: backup lama dihapus", slog.Int("count", n))
	}
	return key, nil
}

// Prune menghapus backup yang lebih tua dari Retention (selalu menyisakan yang terbaru).
func (s *Service) Prune(ctx context.Context) (int, error) {
	objs, err := s.Store.List(ctx)
	if err != nil {
		return 0, err
	}
	cutoff := s.now().Add(-s.Retention)
	n := 0
	for i, o := range objs {
		if i == 0 || !o.Created.Before(cutoff) {
			continue
		}
		if err := s.Store.Delete(ctx, o.Key); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// Restore memulihkan backup `key` ke database targetURL (--clean: objek yang
// ada ditimpa). Target harus database terpisah kecuali overwrite=true.
func (s *Service) Restore(ctx context.Context, key, targetURL string, overwrite bool) error {
	if targetURL == "" {
		return errors.New("backup: target database wajib diisi")
	}
	if targetURL == s.DBURL && !overwrite {
		return errors.New("backup: target sama dengan DATABASE_URL aplikasi; tambahkan --overwrite bila memang ingin menimpa database produksi")
	}
	rc, err := s.Store.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("backup: ambil %s: %w", key, err)
	}
	defer func() { _ = rc.Close() }()
	tmp, err := os.CreateTemp("", "lovoria-restore-*.dump")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, rc); err != nil {
		_ = tmp.Close()
		return err
	}
	_ = tmp.Close()
	cmd := exec.CommandContext(ctx, bin(s.PgRestore, "pg_restore"), //nolint:gosec // G204: binary dari config operator
		"--clean", "--if-exists", "--no-owner", "--no-privileges", "--exit-on-error", "--dbname="+targetURL, tmp.Name())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("backup: pg_restore: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// lockKey: advisory lock supaya hanya satu instance yang membuat backup.
const lockKey int64 = 0x107E0003

// RunDaily membuat backup setiap hari pada jam `hourUTC` (dan tidak lagi bila
// sudah ada backup < 20 jam, mis. setelah restart).
func (s *Service) RunDaily(ctx context.Context, pool *pgxpool.Pool, hourUTC int) {
	for {
		now := s.now().UTC()
		next := time.Date(now.Year(), now.Month(), now.Day(), hourUTC, 0, 0, 0, time.UTC)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(next.Sub(now)):
		}
		if err := s.runLocked(ctx, pool); err != nil {
			s.Log.ErrorContext(ctx, "backup: gagal", slog.String("error", err.Error()))
		}
	}
}

func (s *Service) runLocked(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockKey).Scan(&locked); err != nil || !locked {
		return err
	}
	defer func() { _, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", lockKey) }()
	if objs, err := s.Store.List(ctx); err == nil && len(objs) > 0 && s.now().Sub(objs[0].Created) < 20*time.Hour {
		return nil // sudah ada backup hari ini (instance lain / restart)
	}
	_, err = s.Run(ctx)
	return err
}
