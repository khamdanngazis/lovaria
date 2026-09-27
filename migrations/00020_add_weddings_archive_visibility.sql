-- +goose Up
-- Siapa yang bisa melihat undangan setelah diarsipkan (T19): public = semua
-- (read-only: cerita, galeri, ucapan), private = hanya pemilik yang login.
ALTER TABLE weddings ADD COLUMN archive_visibility text NOT NULL DEFAULT 'public'
    CHECK (archive_visibility IN ('public', 'private'));

-- +goose Down
ALTER TABLE weddings DROP COLUMN IF EXISTS archive_visibility;
