package web

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

type ctxKey int

const (
	userKey ctxKey = iota
	csrfKey
	weddingKey
	readOnlyKey
)

// User adalah identitas user yang sedang login, disimpan di context request oleh
// middleware auth. Dipakai template & modul lain tanpa mengimpor modul auth.
type User struct {
	ID    uuid.UUID
	Email string
	Name  string
	Role  string
}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// CurrentUser mengembalikan user yang login, bila ada.
func CurrentUser(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey).(User)
	return u, ok
}

// WithWeddingID menyimpan wedding_id yang sudah diotorisasi (diisi middleware
// wedding.RequireWeddingOwner). Modul dashboard lain membaca dari sini.
func WithWeddingID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, weddingKey, id)
}

// WeddingID mengembalikan wedding_id dari context, bila ada.
func WeddingID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(weddingKey).(uuid.UUID)
	return id, ok
}

// WithReadOnly menandai request dashboard sebagai mode lihat-saja (admin
// melihat dashboard pasangan, T16). Layout menampilkan banner.
func WithReadOnly(ctx context.Context) context.Context {
	return context.WithValue(ctx, readOnlyKey, true)
}

// ReadOnly: request sedang dalam mode lihat-saja.
func ReadOnly(ctx context.Context) bool {
	v, _ := ctx.Value(readOnlyKey).(bool)
	return v
}

func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey, token)
}

// CSRFToken mengembalikan token CSRF untuk dirender di form (field _csrf).
func CSRFToken(ctx context.Context) string {
	t, _ := ctx.Value(csrfKey).(string)
	return t
}

// HXHeaders mengembalikan nilai atribut hx-headers yang menyertakan token CSRF
// di setiap request htmx.
func HXHeaders(ctx context.Context) string {
	b, _ := json.Marshal(map[string]string{"X-CSRF-Token": CSRFToken(ctx)})
	return string(b)
}
