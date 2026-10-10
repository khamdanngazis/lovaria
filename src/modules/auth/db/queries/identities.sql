-- name: GetUserByIdentity :one
SELECT u.* FROM user_identities i JOIN users u ON u.id = i.user_id
WHERE i.provider = $1 AND i.subject = $2;

-- name: CreateIdentity :exec
INSERT INTO user_identities (provider, subject, user_id, email) VALUES ($1, $2, $3, $4);

-- name: MarkEmailVerified :exec
UPDATE users SET email_verified_at = COALESCE(email_verified_at, $2) WHERE id = $1;
