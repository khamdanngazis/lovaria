-- name: InsertStatusHistory :exec
INSERT INTO wedding_status_history (id, wedding_id, from_status, to_status, actor, actor_user_id, at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListStatusHistory :many
SELECT * FROM wedding_status_history WHERE wedding_id = $1 ORDER BY at DESC, id DESC LIMIT $2;
