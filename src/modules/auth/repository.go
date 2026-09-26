package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	authdb "github.com/khamdanngazis/lovaria/src/modules/auth/db"
)

var (
	errNotFound   = errors.New("auth: tidak ditemukan")
	errEmailTaken = errors.New("auth: email sudah terdaftar")
)

// Repository adalah akses data modul auth (tabel users, sessions,
// password_reset_tokens). Hanya dipakai oleh Service di modul ini.
type Repository struct {
	pool *pgxpool.Pool
	q    *authdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: authdb.New(pool)}
}

// inTx menjalankan fn dalam satu transaksi.
func (r *Repository) inTx(ctx context.Context, fn func(q *authdb.Queries) error) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error { return fn(r.q.WithTx(tx)) })
}

// mapErr menerjemahkan error pgx ke error domain.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return errNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_email_key" {
		return errEmailTaken
	}
	return err
}

func toUser(u authdb.User) User {
	return User{
		ID:              u.ID,
		Email:           u.Email,
		Name:            u.Name,
		Role:            u.Role,
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
		passwordHash:    u.PasswordHash,
	}
}
