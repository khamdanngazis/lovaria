-- name: CreateGuest :one
INSERT INTO guests (id, wedding_id, name, phone, email, group_name, max_pax, invitation_code, notes)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertGuests :copyfrom
INSERT INTO guests (id, wedding_id, name, phone, email, group_name, max_pax, invitation_code)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetGuest :one
SELECT * FROM guests WHERE id = $1 AND wedding_id = $2;

-- name: GetGuestByCode :one
-- tenant:ignore kode undangan unik global; wedding di-resolve dari hasilnya (resolver T09)
SELECT * FROM guests WHERE invitation_code = $1;

-- name: UpdateGuest :one
UPDATE guests
SET name = $3, phone = $4, email = $5, group_name = $6, max_pax = $7, notes = $8
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: UpdateRSVP :one
UPDATE guests
SET rsvp_status = $3, rsvp_pax = $4, rsvp_message = $5, rsvp_at = $6
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: MarkOpened :exec
UPDATE guests SET last_opened_at = $3 WHERE id = $1 AND wedding_id = $2;

-- name: DeleteGuest :execrows
DELETE FROM guests WHERE id = $1 AND wedding_id = $2;

-- name: DeleteGuests :execrows
DELETE FROM guests WHERE wedding_id = $1 AND id = ANY(sqlc.arg(ids)::uuid[]);

-- name: CountGuests :one
SELECT count(*) FROM guests
WHERE wedding_id = sqlc.arg(wedding_id)
  AND (sqlc.narg(status)::text IS NULL OR rsvp_status = sqlc.narg(status))
  AND (sqlc.narg(group_name)::text IS NULL OR group_name = sqlc.narg(group_name))
  AND (sqlc.narg(q)::text IS NULL
       OR name ILIKE '%' || sqlc.narg(q) || '%'
       OR phone LIKE '%' || sqlc.narg(q) || '%'
       OR email::text ILIKE '%' || sqlc.narg(q) || '%'
       OR invitation_code = upper(sqlc.narg(q)));

-- name: ListGuests :many
SELECT * FROM guests
WHERE wedding_id = sqlc.arg(wedding_id)
  AND (sqlc.narg(status)::text IS NULL OR rsvp_status = sqlc.narg(status))
  AND (sqlc.narg(group_name)::text IS NULL OR group_name = sqlc.narg(group_name))
  AND (sqlc.narg(q)::text IS NULL
       OR name ILIKE '%' || sqlc.narg(q) || '%'
       OR phone LIKE '%' || sqlc.narg(q) || '%'
       OR email::text ILIKE '%' || sqlc.narg(q) || '%'
       OR invitation_code = upper(sqlc.narg(q)))
ORDER BY lower(name), id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListAllGuests :many
SELECT * FROM guests WHERE wedding_id = $1 ORDER BY lower(name), id;

-- name: ListGroups :many
SELECT DISTINCT group_name FROM guests
WHERE wedding_id = $1 AND group_name <> ''
ORDER BY group_name;

-- name: GuestStats :one
SELECT
    count(*)                                                  AS total,
    count(*) FILTER (WHERE rsvp_status = 'attending')         AS attending,
    count(*) FILTER (WHERE rsvp_status = 'declined')          AS declined,
    count(*) FILTER (WHERE rsvp_status = 'pending')           AS pending,
    COALESCE(sum(max_pax), 0)::bigint                         AS pax_invited,
    COALESCE(sum(rsvp_pax) FILTER (WHERE rsvp_status = 'attending'), 0)::bigint AS pax_attending,
    count(*) FILTER (WHERE last_opened_at IS NOT NULL)        AS opened
FROM guests WHERE wedding_id = $1;

-- name: CountRSVPResponses :one
SELECT count(*) FROM guests
WHERE wedding_id = sqlc.arg(wedding_id)
  AND rsvp_at IS NOT NULL
  AND (sqlc.narg(status)::text IS NULL OR rsvp_status = sqlc.narg(status));

-- name: ListRSVPResponses :many
SELECT * FROM guests
WHERE wedding_id = sqlc.arg(wedding_id)
  AND rsvp_at IS NOT NULL
  AND (sqlc.narg(status)::text IS NULL OR rsvp_status = sqlc.narg(status))
ORDER BY rsvp_at DESC, id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);
