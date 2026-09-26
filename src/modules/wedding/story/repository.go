package story

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	storydb "github.com/khamdanngazis/lovaria/src/modules/wedding/story/db"
)

// Repository adalah akses data tabel love_stories.
type Repository struct {
	pool *pgxpool.Pool
	q    *storydb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: storydb.New(pool)}
}

func (r *Repository) inTx(ctx context.Context, fn func(q *storydb.Queries) error) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error { return fn(r.q.WithTx(tx)) })
}

// saveOrder menyimpan urutan ids sebagai sort_order 0..n-1.
func saveOrder(ctx context.Context, q *storydb.Queries, weddingID uuid.UUID, ids []uuid.UUID) error {
	for i, id := range ids {
		if err := q.SetStorySortOrder(ctx, storydb.SetStorySortOrderParams{ID: id, WeddingID: weddingID, SortOrder: int32(i)}); err != nil { //nolint:gosec // G115: jumlah cerita kecil
			return err
		}
	}
	return nil
}
