-- name: CreatePasswordResetToken :exec
INSERT INTO password_reset_tokens (id, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: ConsumePasswordResetToken :one
-- Atomik: token hanya bisa dipakai sekali dan sebelum kedaluwarsa.
UPDATE password_reset_tokens
SET used_at = $2
WHERE id = $1 AND used_at IS NULL AND expires_at > $2
RETURNING user_id;

-- name: DeleteUserPasswordResetTokens :exec
DELETE FROM password_reset_tokens WHERE user_id = $1;

-- name: DeleteExpiredPasswordResetTokens :execrows
DELETE FROM password_reset_tokens WHERE expires_at <= $1;
