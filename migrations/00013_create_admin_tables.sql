-- +goose Up
-- Paket layanan (tanpa payment gateway): batas storage & lama arsip per wedding.
CREATE TABLE packages (
    id            uuid PRIMARY KEY,
    name          text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 60),
    storage_mb    integer NOT NULL CHECK (storage_mb BETWEEN 1 AND 102400),
    archive_days  integer NOT NULL CHECK (archive_days BETWEEN 1 AND 3650),
    price_display text NOT NULL DEFAULT '', -- mis. "Rp 99.000" (hanya tampilan)
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT packages_name_key UNIQUE (name)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON packages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Paket yang dipasang admin ke wedding (tanpa baris = default dari config).
CREATE TABLE wedding_packages (
    wedding_id  uuid PRIMARY KEY REFERENCES weddings (id) ON DELETE CASCADE,
    package_id  uuid NOT NULL REFERENCES packages (id) ON DELETE RESTRICT,
    assigned_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX wedding_packages_package_id_idx ON wedding_packages (package_id);

-- Jejak setiap aksi tulis admin.
CREATE TABLE admin_audit_logs (
    id            uuid PRIMARY KEY,
    admin_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    admin_email   text NOT NULL, -- disimpan juga supaya jejak tetap terbaca bila akun dihapus
    action        text NOT NULL,
    target_type   text NOT NULL,
    target_id     text NOT NULL DEFAULT '',
    details       jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX admin_audit_logs_created_at_idx ON admin_audit_logs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS admin_audit_logs;
DROP TABLE IF EXISTS wedding_packages;
DROP TABLE IF EXISTS packages;
