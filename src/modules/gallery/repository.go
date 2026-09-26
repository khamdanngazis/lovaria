package gallery

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	gallerydb "github.com/khamdanngazis/lovaria/src/modules/gallery/db"
)

// Repository adalah akses data tabel gallery_items.
type Repository struct {
	pool *pgxpool.Pool
	q    *gallerydb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: gallerydb.New(pool)}
}

func (r *Repository) inTx(ctx context.Context, fn func(q *gallerydb.Queries) error) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error { return fn(r.q.WithTx(tx)) })
}
