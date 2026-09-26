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
