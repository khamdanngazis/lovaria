-- name: CreateWedding :one
INSERT INTO weddings (id, owner_user_id, slug, title, wedding_date, description)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetWedding :one
SELECT * FROM weddings WHERE id = $1;

-- name: GetWeddingForOwner :one
SELECT * FROM weddings WHERE id = $1 AND owner_user_id = $2;

-- name: GetWeddingBySlug :one
SELECT * FROM weddings WHERE slug = $1;

-- name: ListWeddingsByOwner :many
SELECT * FROM weddings WHERE owner_user_id = $1 ORDER BY created_at DESC, id DESC;

-- name: ListSlugsWithPrefix :many
-- Slug yang sudah dipakai: persis `base` atau `base-<angka>` (untuk memilih suffix).
SELECT slug::text FROM weddings WHERE slug = sqlc.arg(base)::citext OR slug ~ ('^' || sqlc.arg(base)::text || '-[0-9]+$');

-- name: UpdateWeddingInfo :one
UPDATE weddings
SET title = $2, wedding_date = $3, description = $4, main_photo_url = $5
WHERE id = $1
RETURNING *;
