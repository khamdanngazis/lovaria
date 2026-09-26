package event

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	eventdb "github.com/khamdanngazis/lovaria/src/modules/wedding/event/db"
)

// Repository adalah akses data tabel events.
type Repository struct {
	pool *pgxpool.Pool
	q    *eventdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: eventdb.New(pool)}
}

func (r *Repository) inTx(ctx context.Context, fn func(q *eventdb.Queries) error) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error { return fn(r.q.WithTx(tx)) })
}

// saveOrder menyimpan urutan ids sebagai sort_order 0..n-1.
func saveOrder(ctx context.Context, q *eventdb.Queries, weddingID uuid.UUID, ids []uuid.UUID) error {
	for i, id := range ids {
		if err := q.SetEventSortOrder(ctx, eventdb.SetEventSortOrderParams{ID: id, WeddingID: weddingID, SortOrder: int32(i)}); err != nil { //nolint:gosec // G115: jumlah acara kecil
			return err
		}
	}
	return nil
}
