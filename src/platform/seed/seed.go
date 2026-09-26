// Package seed menjalankan data contoh untuk development (`lovoria seed`).
//
// Setiap modul boleh menyediakan Seeder (mis. wedding.Seeder(svc)) yang
// didaftarkan di cmd/server/main.go. Seeder memanggil service modulnya sendiri,
// bukan menulis langsung ke tabel modul lain. Seeder harus idempoten.
package seed

import (
	"context"
	"fmt"
	"log/slog"
)

// Seeder adalah satu langkah seed.
type Seeder struct {
	Name string
	Run  func(ctx context.Context) error
}

// Run menjalankan seeder berurutan dan berhenti di error pertama.
func Run(ctx context.Context, log *slog.Logger, seeders []Seeder) error {
	if len(seeders) == 0 {
		log.Info("seed: belum ada seeder terdaftar")
		return nil
	}
	for _, s := range seeders {
		log.Info("seed: running", slog.String("seeder", s.Name))
		if err := s.Run(ctx); err != nil {
			return fmt.Errorf("seed %s: %w", s.Name, err)
		}
	}
	log.Info("seed: selesai", slog.Int("count", len(seeders)))
	return nil
}
