package example

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

const maxNoteLength = 280

var (
	ErrInvalidWedding = errors.New("wedding_id tidak valid")
	ErrEmptyNote      = errors.New("catatan tidak boleh kosong")
	ErrNoteTooLong    = errors.New("catatan maksimal 280 karakter")
)

// Service adalah API publik modul. Modul lain hanya boleh memanggil Service,
// tidak pernah Repository atau tabel milik modul ini.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListNotes(ctx context.Context, weddingID int64) ([]Note, error) {
	if weddingID <= 0 {
		return nil, ErrInvalidWedding
	}
	return s.repo.ListNotes(ctx, weddingID)
}

func (s *Service) AddNote(ctx context.Context, weddingID int64, body string) (Note, error) {
	if weddingID <= 0 {
		return Note{}, ErrInvalidWedding
	}
	body = strings.TrimSpace(body)
	switch {
	case body == "":
		return Note{}, ErrEmptyNote
	case utf8.RuneCountInString(body) > maxNoteLength:
		return Note{}, ErrNoteTooLong
	}
	return s.repo.CreateNote(ctx, weddingID, body)
}
