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
SET title = $2, wedding_date = $3, description = $4, main_photo_url = $5, timezone = $6
WHERE id = $1
RETURNING *;

-- name: ReserveStorage :one
-- Atomik: tambah pemakaian hanya bila tidak melewati batas.
UPDATE weddings
SET storage_used_bytes = storage_used_bytes + sqlc.arg(bytes)::bigint
WHERE id = sqlc.arg(id) AND storage_used_bytes + sqlc.arg(bytes)::bigint <= sqlc.arg(quota)::bigint
RETURNING storage_used_bytes;

-- name: ReleaseStorage :exec
UPDATE weddings
SET storage_used_bytes = GREATEST(0, storage_used_bytes - sqlc.arg(bytes)::bigint)
WHERE id = sqlc.arg(id);

-- name: SetMainPhotoURL :exec
UPDATE weddings SET main_photo_url = $2 WHERE id = $1;

-- name: CountMainPhotoPrefix :one
SELECT count(*) FROM weddings WHERE starts_with(main_photo_url, sqlc.arg(old_prefix)::text);

-- name: RebaseMainPhotoURL :execrows
UPDATE weddings
SET main_photo_url = sqlc.arg(new_prefix)::text || substr(main_photo_url, length(sqlc.arg(old_prefix)::text) + 1)
WHERE starts_with(main_photo_url, sqlc.arg(old_prefix)::text);

-- name: SetThemeID :execrows
UPDATE weddings SET theme_id = $2 WHERE id = $1;
