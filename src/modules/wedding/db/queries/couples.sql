-- name: CreateCouple :one
INSERT INTO couples (id, wedding_id, groom_name, bride_name)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetCouple :one
SELECT * FROM couples WHERE wedding_id = $1;

-- name: UpdateCouple :one
UPDATE couples
SET groom_name = $2, bride_name = $3,
    groom_photo_url = $4, bride_photo_url = $5,
    groom_description = $6, bride_description = $7
WHERE wedding_id = $1
RETURNING *;

-- name: CountCouplePhotoPrefix :one
-- tenant:ignore perawatan lintas wedding: ganti basis URL media (lovoria media rebase-urls)
SELECT count(*) FROM couples
WHERE starts_with(groom_photo_url, sqlc.arg(old_prefix)::text) OR starts_with(bride_photo_url, sqlc.arg(old_prefix)::text);

-- name: RebaseGroomPhotoURL :execrows
-- tenant:ignore perawatan lintas wedding: ganti basis URL media (lovoria media rebase-urls)
UPDATE couples
SET groom_photo_url = sqlc.arg(new_prefix)::text || substr(groom_photo_url, length(sqlc.arg(old_prefix)::text) + 1)
WHERE starts_with(groom_photo_url, sqlc.arg(old_prefix)::text);

-- name: RebaseBridePhotoURL :execrows
-- tenant:ignore perawatan lintas wedding: ganti basis URL media (lovoria media rebase-urls)
UPDATE couples
SET bride_photo_url = sqlc.arg(new_prefix)::text || substr(bride_photo_url, length(sqlc.arg(old_prefix)::text) + 1)
WHERE starts_with(bride_photo_url, sqlc.arg(old_prefix)::text);
