-- +goose Up
-- Akun yang dinonaktifkan admin (T16): tidak bisa login, sesi aktif ditolak.
ALTER TABLE users ADD COLUMN disabled_at timestamptz;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS disabled_at;
