-- name: CreateUser :one
INSERT INTO users (id, email, password_hash, name, role)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2 WHERE id = $1;

-- name: SearchUsers :many
-- Panel admin (T16): cari nama/email, terbaru dulu.
SELECT * FROM users
WHERE sqlc.narg(q)::text IS NULL OR name ILIKE '%' || sqlc.narg(q) || '%' OR email::text ILIKE '%' || sqlc.narg(q) || '%'
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountSearchUsers :one
SELECT count(*) FROM users
WHERE sqlc.narg(q)::text IS NULL OR name ILIKE '%' || sqlc.narg(q) || '%' OR email::text ILIKE '%' || sqlc.narg(q) || '%';

-- name: SetUserDisabled :one
UPDATE users SET disabled_at = $2 WHERE id = $1 RETURNING *;
