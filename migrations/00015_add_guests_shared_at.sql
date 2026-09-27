-- +goose Up
-- Kapan undangan tamu terakhir dibagikan (Kirim WA / Salin, T14).
ALTER TABLE guests ADD COLUMN shared_at timestamptz;

-- +goose Down
ALTER TABLE guests DROP COLUMN IF EXISTS shared_at;
