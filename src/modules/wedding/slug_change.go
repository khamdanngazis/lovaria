package wedding

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	weddingdb "github.com/khamdanngazis/lovaria/src/modules/wedding/db"
)

// SlugRedirectTTL: lama slug lama dialihkan ke slug baru (T14).
const SlugRedirectTTL = 90 * 24 * time.Hour

// SlugError: slug baru tidak valid / sudah dipakai (pesan untuk pasangan).
type SlugError struct{ Msg string }

func (e *SlugError) Error() string { return e.Msg }

// SlugRedirect adalah slug lama yang masih dialihkan.
type SlugRedirect struct {
	OldSlug   string
	ExpiresAt time.Time
}

// ChangeSlug mengganti slug wedding. Slug lama dialihkan (301) ke slug baru
// selama SlugRedirectTTL supaya link yang sudah dibagikan tidak putus.
func (s *Service) ChangeSlug(ctx context.Context, weddingID uuid.UUID, input string) (Wedding, error) {
	slug := strings.ToLower(strings.TrimSpace(input))
	if msg := ValidateSlug(slug); msg != "" {
		return Wedding{}, &SlugError{Msg: msg}
	}
	now := s.clock()
	var out Wedding
	err := s.repo.inTx(ctx, func(q *weddingdb.Queries) error {
		cur, err := q.GetWeddingForUpdate(ctx, weddingID)
		if err != nil {
			return mapErr(err)
		}
		if strings.EqualFold(cur.Slug, slug) {
			out = toWedding(cur)
			return nil
		}
		taken, err := q.SlugTaken(ctx, weddingdb.SlugTakenParams{Slug: slug, WeddingID: weddingID, Now: now})
		if err != nil {
			return err
		}
		if taken {
			return &SlugError{Msg: "Alamat ini sudah dipakai undangan lain"}
		}
		// Kembali ke slug lama sendiri: redirect-nya tidak diperlukan lagi.
		if err := q.DeleteSlugRedirect(ctx, weddingdb.DeleteSlugRedirectParams{OldSlug: slug, WeddingID: weddingID}); err != nil {
			return err
		}
		if err := q.UpsertSlugRedirect(ctx, weddingdb.UpsertSlugRedirectParams{OldSlug: cur.Slug, WeddingID: weddingID, ExpiresAt: now.Add(SlugRedirectTTL)}); err != nil {
			return err
		}
		row, err := q.SetSlug(ctx, weddingdb.SetSlugParams{ID: weddingID, Slug: slug})
		if err != nil {
			return err
		}
		out = toWedding(row)
		return nil
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // balapan dengan wedding lain
		return Wedding{}, &SlugError{Msg: "Alamat ini sudah dipakai undangan lain"}
	}
	return out, err
}

// SlugRedirectTarget: wedding pemilik slug lama yang masih aktif dialihkan
// (dipakai resolver public site). ok=false bila tidak ada.
func (s *Service) SlugRedirectTarget(ctx context.Context, oldSlug string) (Wedding, bool, error) {
	id, err := s.repo.q.GetSlugRedirect(ctx, weddingdb.GetSlugRedirectParams{OldSlug: strings.ToLower(strings.TrimSpace(oldSlug)), ExpiresAt: s.clock()})
	if errors.Is(err, pgx.ErrNoRows) {
		return Wedding{}, false, nil
	}
	if err != nil {
		return Wedding{}, false, err
	}
	w, err := s.GetWedding(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Wedding{}, false, nil
	}
	return w, err == nil, err
}

// SlugRedirects: slug lama wedding yang masih dialihkan (halaman Bagikan).
func (s *Service) SlugRedirects(ctx context.Context, weddingID uuid.UUID) ([]SlugRedirect, error) {
	rows, err := s.repo.q.ListSlugRedirects(ctx, weddingdb.ListSlugRedirectsParams{WeddingID: weddingID, ExpiresAt: s.clock()})
	if err != nil {
		return nil, err
	}
	out := make([]SlugRedirect, len(rows))
	for i, r := range rows {
		out[i] = SlugRedirect{OldSlug: r.OldSlug, ExpiresAt: r.ExpiresAt}
	}
	return out, nil
}

// CanonicalOrigin: basis URL link undangan tamu (/i/KODE) — https://<custom
// domain> bila aktif, selain itu BASE_URL.
func (s *Service) CanonicalOrigin(ctx context.Context, weddingID uuid.UUID) (string, error) {
	if s.domains != nil {
		host, ok, err := s.domains.ActiveDomain(ctx, weddingID)
		if err != nil {
			return "", err
		}
		if ok {
			return "https://" + host, nil
		}
	}
	return s.baseURL, nil
}
