package wedding

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	weddingdb "github.com/khamdanngazis/lovaria/src/modules/wedding/db"
)

var errSlugTaken = errors.New("wedding: slug sudah dipakai")

// Repository adalah akses data modul wedding (tabel weddings, couples).
type Repository struct {
	pool *pgxpool.Pool
	q    *weddingdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: weddingdb.New(pool)}
}

func (r *Repository) inTx(ctx context.Context, fn func(q *weddingdb.Queries) error) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error { return fn(r.q.WithTx(tx)) })
}

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "weddings_slug_key" {
		return errSlugTaken
	}
	return err
}

func toWedding(w weddingdb.Wedding) Wedding {
	return Wedding{
		ID:           w.ID,
		OwnerUserID:  w.OwnerUserID,
		Slug:         w.Slug,
		Title:        w.Title,
		WeddingDate:  w.WeddingDate,
		Description:  w.Description,
		MainPhotoURL: w.MainPhotoUrl,
		Status:       w.Status,
		ThemeID:      w.ThemeID,
		CreatedAt:    w.CreatedAt,
		UpdatedAt:    w.UpdatedAt,
	}
}

func toCouple(c weddingdb.Couple) Couple {
	return Couple{
		ID:               c.ID,
		WeddingID:        c.WeddingID,
		GroomName:        c.GroomName,
		BrideName:        c.BrideName,
		GroomPhotoURL:    c.GroomPhotoUrl,
		BridePhotoURL:    c.BridePhotoUrl,
		GroomDescription: c.GroomDescription,
		BrideDescription: c.BrideDescription,
	}
}
