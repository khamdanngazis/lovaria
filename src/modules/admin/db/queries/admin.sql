-- Modul admin hanya menyentuh tabel miliknya sendiri: packages,
-- wedding_packages, admin_audit_logs. Data modul lain lewat service modul itu.

-- name: ListPackages :many
SELECT p.*, (SELECT count(*) FROM wedding_packages wp WHERE wp.package_id = p.id) AS weddings -- tenant:ignore hitung pemakai paket
FROM packages p ORDER BY p.sort_order, p.storage_mb, p.name;

-- name: GetPackage :one
SELECT * FROM packages WHERE id = $1;

-- name: CreatePackage :one
INSERT INTO packages (id, name, storage_mb, archive_days, price_display, show_on_landing, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdatePackage :one
UPDATE packages SET name = $2, storage_mb = $3, archive_days = $4, price_display = $5,
    show_on_landing = $6, sort_order = $7
WHERE id = $1
RETURNING *;

-- name: DeletePackage :execrows
DELETE FROM packages WHERE id = $1;

-- name: GetWeddingPackage :one
SELECT p.* FROM wedding_packages wp JOIN packages p ON p.id = wp.package_id
WHERE wp.wedding_id = $1;

-- name: AssignPackage :exec
INSERT INTO wedding_packages (wedding_id, package_id, assigned_at) VALUES ($1, $2, $3)
ON CONFLICT (wedding_id) DO UPDATE SET package_id = EXCLUDED.package_id, assigned_at = EXCLUDED.assigned_at;

-- name: UnassignPackage :execrows
DELETE FROM wedding_packages WHERE wedding_id = $1;

-- name: InsertAuditLog :exec
INSERT INTO admin_audit_logs (id, admin_user_id, admin_email, action, target_type, target_id, details, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListAuditLogs :many
SELECT * FROM admin_audit_logs
WHERE (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountAuditLogs :one
SELECT count(*) FROM admin_audit_logs
WHERE (sqlc.narg(target_id)::text IS NULL OR target_id = sqlc.narg(target_id));

-- name: ListLandingPackages :many
SELECT * FROM packages WHERE show_on_landing ORDER BY sort_order, storage_mb, name;
