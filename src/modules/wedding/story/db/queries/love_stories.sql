-- name: CreateStory :one
INSERT INTO love_stories (id, wedding_id, date_year, date_month, date_day, title, description, photo_url, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: GetStory :one
SELECT * FROM love_stories WHERE id = $1 AND wedding_id = $2;

-- name: ListStories :many
SELECT * FROM love_stories WHERE wedding_id = $1
ORDER BY sort_order, date_year, date_month NULLS FIRST, date_day NULLS FIRST, id;

-- name: ListStoriesForUpdate :many
SELECT * FROM love_stories WHERE wedding_id = $1
ORDER BY sort_order, date_year, date_month NULLS FIRST, date_day NULLS FIRST, id
FOR UPDATE;

-- name: UpdateStory :one
UPDATE love_stories
SET date_year = $3, date_month = $4, date_day = $5, title = $6, description = $7, photo_url = $8
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: SetStorySortOrder :exec
UPDATE love_stories SET sort_order = $3 WHERE id = $1 AND wedding_id = $2;

-- name: DeleteStory :execrows
DELETE FROM love_stories WHERE id = $1 AND wedding_id = $2;

-- name: CountStoryPhotoPrefix :one
-- tenant:ignore perawatan lintas wedding: ganti basis URL media (lovoria media rebase-urls)
SELECT count(*) FROM love_stories WHERE starts_with(photo_url, sqlc.arg(old_prefix)::text);

-- name: RebaseStoryPhotoURL :execrows
-- tenant:ignore perawatan lintas wedding: ganti basis URL media (lovoria media rebase-urls)
UPDATE love_stories
SET photo_url = sqlc.arg(new_prefix)::text || substr(photo_url, length(sqlc.arg(old_prefix)::text) + 1)
WHERE starts_with(photo_url, sqlc.arg(old_prefix)::text);
