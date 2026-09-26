package guest

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/platform/db"

	guestdb "github.com/khamdanngazis/lovaria/src/modules/guest/db"
)

// Repository adalah akses data tabel guests.
type Repository struct {
	pool *pgxpool.Pool
	q    *guestdb.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool, q: guestdb.New(pool)}
}

func (r *Repository) inTx(ctx context.Context, fn func(q *guestdb.Queries) error) error {
	return db.WithTx(ctx, r.pool, func(tx pgx.Tx) error { return fn(r.q.WithTx(tx)) })
}

func toGuest(r guestdb.Guest) Guest {
	g := Guest{
		ID: r.ID, WeddingID: r.WeddingID, Name: r.Name, Phone: r.Phone, GroupName: r.GroupName,
		MaxPax: int(r.MaxPax), InvitationCode: r.InvitationCode, RSVPStatus: r.RsvpStatus,
		RSVPPax: int(r.RsvpPax), RSVPMessage: r.RsvpMessage, RSVPAt: r.RsvpAt, Notes: r.Notes,
		LastOpenedAt: r.LastOpenedAt, CreatedAt: r.CreatedAt,
	}
	if r.Email != nil {
		g.Email = *r.Email
	}
	return g
}
