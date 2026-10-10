-- Check-in tamu dengan QR di hari H (T31).

-- name: GetActiveCheckinLink :one
SELECT * FROM checkin_links WHERE wedding_id = $1 AND revoked_at IS NULL;

-- name: GetCheckinLinkByID :one
-- tenant:ignore link dicari lewat id dari token bertanda tangan; wedding di-resolve dari hasilnya
SELECT * FROM checkin_links WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeCheckinLinks :exec
UPDATE checkin_links SET revoked_at = $2 WHERE wedding_id = $1 AND revoked_at IS NULL;

-- name: CreateCheckinLink :one
INSERT INTO checkin_links (id, wedding_id, created_at) VALUES ($1, $2, $3) RETURNING *;

-- name: GetGuestByCodeInWedding :one
SELECT * FROM guests WHERE wedding_id = $1 AND invitation_code = $2;

-- name: CheckInGuest :one
-- Hanya berhasil bila belum check-in: dua pemindaian bersamaan → satu baris.
UPDATE guests
SET checked_in_at = $3, checked_in_pax = $4, checked_in_via = $5, attendance_status = 'present'
WHERE id = $1 AND wedding_id = $2 AND checked_in_at IS NULL
RETURNING *;

-- name: UndoCheckIn :execrows
UPDATE guests
SET checked_in_at = NULL, checked_in_pax = NULL, checked_in_via = NULL, attendance_status = NULL
WHERE id = $1 AND wedding_id = $2 AND checked_in_at IS NOT NULL;

-- name: SearchGuestsForCheckin :many
SELECT * FROM guests
WHERE wedding_id = $1 AND (name ILIKE '%' || sqlc.arg(q)::text || '%' OR invitation_code = upper(sqlc.arg(q)::text))
ORDER BY name
LIMIT 8;

-- name: CheckinTotals :one
SELECT
    count(*)::int AS invited,
    count(*) FILTER (WHERE checked_in_at IS NOT NULL)::int AS checked_in,
    COALESCE(sum(checked_in_pax) FILTER (WHERE checked_in_at IS NOT NULL), 0)::int AS checked_in_pax
FROM guests WHERE wedding_id = $1;

-- name: RecentCheckins :many
SELECT * FROM guests WHERE wedding_id = $1 AND checked_in_at IS NOT NULL
ORDER BY checked_in_at DESC LIMIT $2;

-- name: CreateWalkin :one
INSERT INTO checkin_walkins (id, wedding_id, name, pax, created_at) VALUES ($1, $2, $3, $4, $5) RETURNING *;

-- name: WalkinTotals :one
SELECT count(*)::int AS n, COALESCE(sum(pax), 0)::int AS pax FROM checkin_walkins WHERE wedding_id = $1;

-- name: ListWalkins :many
SELECT * FROM checkin_walkins WHERE wedding_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: DeleteWalkin :execrows
DELETE FROM checkin_walkins WHERE id = $1 AND wedding_id = $2;
