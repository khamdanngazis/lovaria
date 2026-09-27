package domain

import (
	"context"
	"log/slog"
	"time"
)

// pollLockKey: advisory lock Postgres supaya hanya satu instance yang memantau.
const pollLockKey int64 = 0x107E0002

// PollPending memeriksa semua domain yang menunggu verifikasi (yang lewat
// PendingTimeout menjadi failed). Mengembalikan jumlah yang berubah status.
func (s *Service) PollPending(ctx context.Context) (int, error) {
	rows, err := s.q.ListPending(ctx)
	if err != nil {
		return 0, err
	}
	changed := 0
	for _, r := range rows {
		d, err := s.check(ctx, r)
		if err != nil {
			continue // sudah dicatat di check; coba lagi putaran berikutnya
		}
		if d.Status != r.Status {
			changed++
		}
	}
	return changed, nil
}

// RunPollingOnce menjalankan PollPending bila memegang advisory lock.
func (s *Service) RunPollingOnce(ctx context.Context) (ran bool, changed int, err error) {
	if !s.Enabled() {
		return false, 0, nil
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return false, 0, err
	}
	defer conn.Release()
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", pollLockKey).Scan(&locked); err != nil {
		return false, 0, err
	}
	if !locked {
		return false, 0, nil
	}
	defer func() {
		_, _ = conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", pollLockKey)
	}()
	changed, err = s.PollPending(ctx)
	return true, changed, err
}

// RunPolling memantau domain pending setiap `every` sampai ctx selesai.
func (s *Service) RunPolling(ctx context.Context, every time.Duration) {
	if !s.Enabled() {
		return
	}
	tick := func() {
		ran, n, err := s.RunPollingOnce(ctx)
		switch {
		case err != nil:
			s.log.ErrorContext(ctx, "domain: polling gagal", slog.String("error", err.Error()))
		case ran && n > 0:
			s.log.InfoContext(ctx, "domain: status diperbarui", slog.Int("changed", n))
		}
	}
	tick()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}
