-- name: NextSortOrder :one
SELECT COALESCE(MAX(sort_order) + 1, 0)::integer FROM gallery_items WHERE wedding_id = $1;

-- name: CreateItem :one
INSERT INTO gallery_items (id, wedding_id, category, object_key, thumb_key, url, thumb_url,
                           width, height, size_bytes, sort_order, caption)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetItem :one
SELECT * FROM gallery_items WHERE id = $1 AND wedding_id = $2;

-- name: ListItems :many
SELECT * FROM gallery_items WHERE wedding_id = $1
ORDER BY sort_order, created_at, id;

-- name: ListItemsForUpdate :many
SELECT * FROM gallery_items WHERE wedding_id = $1
ORDER BY sort_order, created_at, id
FOR UPDATE;

-- name: UpdateItem :one
UPDATE gallery_items SET caption = $3, category = $4
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: SetItemSortOrder :exec
UPDATE gallery_items SET sort_order = $3 WHERE id = $1 AND wedding_id = $2;

-- name: DeleteItem :one
DELETE FROM gallery_items WHERE id = $1 AND wedding_id = $2
RETURNING object_key, thumb_key, size_bytes;
