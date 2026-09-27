-- name: GetByWedding :one
SELECT * FROM custom_domains WHERE wedding_id = $1;

-- name: GetActiveByDomain :one
-- tenant:ignore lookup Host header → wedding (resolver public site); domain unik global
SELECT * FROM custom_domains WHERE domain = $1 AND status = 'active';

-- name: DomainTaken :one
-- tenant:ignore cek domain sudah dipakai wedding lain (unik global)
SELECT EXISTS (
    SELECT 1 FROM custom_domains WHERE domain = sqlc.arg(domain) AND status <> 'removed' AND wedding_id <> sqlc.arg(wedding_id)
);

-- name: InsertDomain :one
INSERT INTO custom_domains (id, wedding_id, domain, cf_hostname_id, status, verification_errors, verified_at, last_checked_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateStatus :one
UPDATE custom_domains
SET status = $3, verification_errors = $4, verified_at = $5, last_checked_at = $6
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: DeleteByWedding :execrows
DELETE FROM custom_domains WHERE wedding_id = $1;

-- name: ListPending :many
-- tenant:ignore scheduler lintas wedding (seperti lifecycle T12)
SELECT * FROM custom_domains WHERE status = 'pending_verification' ORDER BY created_at;

-- name: CountActive :one
-- tenant:ignore pemantauan kuota Cloudflare for SaaS (100 hostname gratis)
SELECT count(*) FROM custom_domains WHERE status = 'active';
