// Package db menyediakan koneksi Postgres (pgxpool), helper transaksi, ID (UUIDv7),
// dan migration goose yang di-embed. Konvensi schema: doc/database.md.
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/config"
	"github.com/khamdanngazis/lovaria/src/platform/health"
)

// Open membuat pool koneksi. Koneksi dibuka secara lazy, jadi Open tidak gagal
// hanya karena DB sedang mati — kondisi itu dilaporkan oleh /readyz.
func Open(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse DATABASE_URL: %w", err)
	}
	pc.MaxConns = cfg.MaxConns
	pc.MinConns = cfg.MinConns
	pc.MaxConnLifetime = cfg.MaxConnLifetime
	pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	pc.ConnConfig.ConnectTimeout = cfg.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("db: open pool: %w", err)
	}
	return pool, nil
}

// NewID menghasilkan UUIDv7 (terurut waktu) untuk primary key.
// ID selalu dibuat di Go, bukan default di database.
func NewID() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}

// Checker mengembalikan health checker yang melakukan ping ke database.
func Checker(pool *pgxpool.Pool) health.Checker {
	return health.CheckerFunc{N: "database", Fn: pool.Ping}
}

// TxBeginner dipenuhi oleh *pgxpool.Pool, *pgx.Conn, dan pgx.Tx (nested → savepoint).
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithTx menjalankan fn di dalam transaksi. Commit bila fn mengembalikan nil,
// rollback bila fn mengembalikan error atau panic (panic diteruskan kembali).
func WithTx(ctx context.Context, db TxBeginner, fn func(tx pgx.Tx) error) (err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			if rbErr := tx.Rollback(context.WithoutCancel(ctx)); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("db: rollback: %w", rbErr))
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
