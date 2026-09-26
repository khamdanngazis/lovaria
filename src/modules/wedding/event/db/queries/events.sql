-- name: CreateEvent :one
INSERT INTO events (id, wedding_id, name, type, event_date, start_time, end_time,
                    venue, address, maps_url, latitude, longitude, description, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: GetEvent :one
SELECT * FROM events WHERE id = $1 AND wedding_id = $2;

-- name: ListEvents :many
SELECT * FROM events WHERE wedding_id = $1
ORDER BY sort_order, event_date, start_time, id;

-- name: ListEventsForUpdate :many
-- Kunci baris event wedding ini selama transaksi reorder.
SELECT * FROM events WHERE wedding_id = $1
ORDER BY sort_order, event_date, start_time, id
FOR UPDATE;

-- name: UpdateEvent :one
UPDATE events
SET name = $3, type = $4, event_date = $5, start_time = $6, end_time = $7,
    venue = $8, address = $9, maps_url = $10, latitude = $11, longitude = $12,
    description = $13
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: SetEventSortOrder :exec
UPDATE events SET sort_order = $3 WHERE id = $1 AND wedding_id = $2;

-- name: DeleteEvent :execrows
DELETE FROM events WHERE id = $1 AND wedding_id = $2;
