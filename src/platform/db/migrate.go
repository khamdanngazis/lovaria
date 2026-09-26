package db

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/khamdanngazis/lovaria/migrations"
)

// MigrateCommands adalah subcommand yang didukung `lovoria migrate <cmd>`.
var MigrateCommands = []string{"up", "down", "status", "version", "redo"}

// Migrate menjalankan perintah migration goose memakai file SQL yang di-embed.
// Hasil dicetak ke out.
func Migrate(ctx context.Context, pool *pgxpool.Pool, command string, out io.Writer) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()

	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	switch command {
	case "up":
		res, err := p.Up(ctx)
		printResults(out, res)
		if err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
		if len(res) == 0 {
			fmt.Fprintln(out, "migrate: tidak ada migration baru")
		}
	case "down":
		res, err := p.Down(ctx)
		if res != nil {
			printResults(out, []*goose.MigrationResult{res})
		}
		if err != nil {
			if errors.Is(err, goose.ErrNoNextVersion) {
				fmt.Fprintln(out, "migrate: tidak ada migration untuk di-rollback")
				return nil
			}
			return fmt.Errorf("migrate down: %w", err)
		}
	case "redo":
		down, err := p.Down(ctx)
		if err != nil {
			return fmt.Errorf("migrate redo (down): %w", err)
		}
		up, err := p.UpByOne(ctx)
		printResults(out, []*goose.MigrationResult{down, up})
		if err != nil {
			return fmt.Errorf("migrate redo (up): %w", err)
		}
	case "status":
		st, err := p.Status(ctx)
		if err != nil {
			return fmt.Errorf("migrate status: %w", err)
		}
		tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "VERSION\tSTATE\tAPPLIED AT\tFILE")
		for _, s := range st {
			applied := "-"
			if !s.AppliedAt.IsZero() {
				applied = s.AppliedAt.Format(time.RFC3339)
			}
			fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", s.Source.Version, s.State, applied, s.Source.Path)
		}
		return tw.Flush()
	case "version":
		v, err := p.GetDBVersion(ctx)
		if err != nil {
			return fmt.Errorf("migrate version: %w", err)
		}
		fmt.Fprintf(out, "migrate: versi database %d\n", v)
	default:
		return fmt.Errorf("migrate: perintah tidak dikenal %q (pilihan: %v)", command, MigrateCommands)
	}
	return nil
}

func printResults(out io.Writer, results []*goose.MigrationResult) {
	for _, r := range results {
		if r == nil {
			continue
		}
		fmt.Fprintf(out, "migrate: %-4s %s (%s)\n", r.Direction, r.Source.Path, r.Duration.Round(time.Millisecond))
	}
}
