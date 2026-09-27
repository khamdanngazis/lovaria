package theme

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/khamdanngazis/lovaria/src/modules/theme/view"

	themedb "github.com/khamdanngazis/lovaria/src/modules/theme/db"
)

// ErrUnknownTheme: ID tema tidak terdaftar di registry.
var ErrUnknownTheme = errors.New("tema tidak dikenal")

// WeddingThemes adalah bagian service wedding yang dipakai modul theme
// (kolom weddings.theme_id milik modul wedding).
type WeddingThemes interface {
	SetThemeID(ctx context.Context, weddingID uuid.UUID, themeID string) error
}

type Service struct {
	q        *themedb.Queries
	weddings WeddingThemes
}

func NewService(pool *pgxpool.Pool, weddings WeddingThemes) *Service {
	return &Service{q: themedb.New(pool), weddings: weddings}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Settings mengembalikan pengaturan tampilan wedding (kosong bila belum pernah diatur).
func (s *Service) Settings(ctx context.Context, weddingID uuid.UUID) (view.Settings, error) {
	r, err := s.q.GetSettings(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return view.Settings{}, nil
	}
	if err != nil {
		return view.Settings{}, err
	}
	return view.Settings{
		PrimaryColor: deref(r.PrimaryColor), FontHeading: deref(r.FontHeading), FontBody: deref(r.FontBody),
		Background: deref(r.BackgroundValue), CoverImage: deref(r.CoverImageUrl),
	}, nil
}

// Save menyimpan pilihan tema + pengaturan tampilan setelah divalidasi.
func (s *Service) Save(ctx context.Context, weddingID uuid.UUID, themeID string, st view.Settings) (view.Settings, error) {
	if !Exists(themeID) {
		return view.Settings{}, ErrUnknownTheme
	}
	st, err := ValidateSettings(st)
	if err != nil {
		return view.Settings{}, err
	}
	if err := s.weddings.SetThemeID(ctx, weddingID, themeID); err != nil {
		return view.Settings{}, err
	}
	err = s.q.UpsertSettings(ctx, themedb.UpsertSettingsParams{
		WeddingID: weddingID, PrimaryColor: ptr(st.PrimaryColor), FontHeading: ptr(st.FontHeading),
		FontBody: ptr(st.FontBody), BackgroundValue: ptr(st.Background), CoverImageUrl: ptr(st.CoverImage),
	})
	return st, err
}

// Configured: pasangan sudah pernah menyimpan pilihan tema (checklist onboarding T13).
func (s *Service) Configured(ctx context.Context, weddingID uuid.UUID) (bool, error) {
	_, err := s.q.GetSettings(ctx, weddingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
