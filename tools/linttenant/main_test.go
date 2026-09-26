package main

import (
	"os"
	"strings"
	"testing"
	"testing/fstest"
)

const migration = `-- +goose Up
CREATE TABLE weddings (id uuid PRIMARY KEY, slug citext NOT NULL);

CREATE TABLE guests (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    code       text NOT NULL,
    UNIQUE (wedding_id, code)
);

CREATE TABLE events (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id)
);
CREATE INDEX events_wedding_id_idx ON events (wedding_id);

CREATE TABLE photos (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL
);

-- +goose Down
CREATE TABLE ghosts (wedding_id uuid);
DROP TABLE photos, events, guests, weddings;
`

const queries = `-- name: ListGuests :many
SELECT * FROM guests WHERE wedding_id = $1;

-- name: GetGuestLeaky :one
SELECT * FROM guests WHERE id = $1;

-- name: GetGuestByCode :one
-- tenant:ignore lookup publik by kode undangan, kode unik global
SELECT * FROM guests WHERE code = $1;

-- name: CreateGuest :one
INSERT INTO guests (id, wedding_id, code) VALUES ($1, $2, $3) RETURNING *;

-- name: CreateEventLeaky :exec
INSERT INTO events (id) VALUES ($1);

-- name: DeleteEvent :exec
DELETE FROM events WHERE id = $1 AND wedding_id = $2;

-- name: UpdateEventLeaky :exec
UPDATE events SET id = $1 -- wedding_id disebut di komentar tidak dihitung
WHERE id = $2;

-- name: GetWedding :one
SELECT * FROM weddings WHERE id = $1;

-- name: JoinLeaky :many
SELECT g.* FROM weddings w JOIN guests g ON g.wedding_id = w.id WHERE w.slug = $1;
`

func TestLint(t *testing.T) {
	fsys := fstest.MapFS{
		"migrations/00002_tables.sql":            {Data: []byte(migration)},
		"src/modules/guest/db/queries/guest.sql": {Data: []byte(queries)},
	}
	problems, err := lint(fsys, "migrations", "src/modules/*/db/queries/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(problems, "\n")

	wantFlagged := []string{"GetGuestLeaky", "CreateEventLeaky", "UpdateEventLeaky", "JoinLeaky", `tabel "photos"`}
	for _, w := range wantFlagged {
		if !strings.Contains(got, w) {
			t.Errorf("harus melaporkan %s\n%s", w, got)
		}
	}
	notFlagged := []string{"ListGuests", "GetGuestByCode", "CreateGuest ", "DeleteEvent", "GetWedding", `"guests" punya`, `"events" punya`, "ghosts"}
	for _, w := range notFlagged {
		if strings.Contains(got, w) {
			t.Errorf("tidak boleh melaporkan %s\n%s", w, got)
		}
	}
	if len(problems) != len(wantFlagged) {
		t.Errorf("jumlah masalah = %d, want %d\n%s", len(problems), len(wantFlagged), got)
	}
	if !strings.Contains(got, "guest.sql:4:") {
		t.Errorf("nomor baris salah:\n%s", got)
	}
}

func TestLintRepoIsClean(t *testing.T) {
	problems, err := lint(os.DirFS("../.."), "migrations", "src/modules/*/db/queries/*.sql")
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v", problems, err)
	}
}
