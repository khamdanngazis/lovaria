// Package example adalah modul referensi yang mendemonstrasikan pola standar modul Lovoria:
//
//	repository.go  akses data (nanti: query sqlc), SELALU difilter wedding_id
//	service.go     business logic + validasi; satu-satunya pintu masuk bagi modul lain
//	handler.go     HTTP handler (Echo), "API-shaped", mengembalikan HTML fragment
//	routes.go      Register(g *echo.Group, deps) — dipanggil dari cmd/server/main.go
//	views.templ    komponen templ milik modul
//
// Modul ini hanya dipasang di APP_ENV=development (/_example). Lihat CONTRIBUTING.md.
package example

import (
	"context"
	"sync"
	"time"
)

// Note adalah contoh entitas tenant: wajib membawa WeddingID.
type Note struct {
	ID        int64
	WeddingID int64
	Body      string
	CreatedAt time.Time
}

// Repository adalah kontrak akses data. Implementasi nyata di modul lain memakai
// sqlc (T02); setiap method tenant wajib menerima weddingID dan memfilternya.
type Repository interface {
	ListNotes(ctx context.Context, weddingID int64) ([]Note, error)
	CreateNote(ctx context.Context, weddingID int64, body string) (Note, error)
}

// MemoryRepository adalah implementasi in-memory untuk demo & test.
type MemoryRepository struct {
	mu     sync.Mutex
	nextID int64
	notes  []Note
	now    func() time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{now: time.Now}
}

func (r *MemoryRepository) ListNotes(_ context.Context, weddingID int64) ([]Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Note
	for _, n := range r.notes {
		if n.WeddingID == weddingID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *MemoryRepository) CreateNote(_ context.Context, weddingID int64, body string) (Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	n := Note{ID: r.nextID, WeddingID: weddingID, Body: body, CreatedAt: r.now()}
	r.notes = append(r.notes, n)
	return n, nil
}
