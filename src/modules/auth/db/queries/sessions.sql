-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5);

-- name: GetActiveSession :one
SELECT sqlc.embed(sessions), sqlc.embed(users)
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.id = $1 AND sessions.expires_at > $2 AND users.disabled_at IS NULL;

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = $2, last_seen_at = $3 WHERE id = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= $1;
