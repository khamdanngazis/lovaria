-- name: CreateEntry :one
INSERT INTO guestbook_entries (id, wedding_id, guest_id, guest_name, message, is_hidden)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListVisible :many
-- Pesan tampil, terbaru dulu. before_id (opsional) = entri terakhir halaman sebelumnya.
SELECT * FROM guestbook_entries e
WHERE e.wedding_id = sqlc.arg(wedding_id)
  AND NOT e.is_hidden
  AND (sqlc.narg(before_id)::uuid IS NULL OR (e.created_at, e.id) < (
      SELECT b.created_at, b.id FROM guestbook_entries b
      WHERE b.id = sqlc.narg(before_id) AND b.wedding_id = sqlc.arg(wedding_id)))
ORDER BY e.created_at DESC, e.id DESC
LIMIT sqlc.arg(lim);

-- name: ListEntries :many
SELECT * FROM guestbook_entries
WHERE wedding_id = sqlc.arg(wedding_id)
  AND (sqlc.narg(hidden)::boolean IS NULL OR is_hidden = sqlc.narg(hidden))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: EntryStats :one
SELECT count(*) AS total, count(*) FILTER (WHERE is_hidden) AS hidden
FROM guestbook_entries WHERE wedding_id = $1;

-- name: SetHidden :one
UPDATE guestbook_entries SET is_hidden = $3
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: DeleteEntry :execrows
DELETE FROM guestbook_entries WHERE id = $1 AND wedding_id = $2;
