-- +goose Up
-- Undangan contoh untuk landing page (T18): tidak dihitung di statistik admin
-- dan tidak diproses scheduler lifecycle.
ALTER TABLE weddings ADD COLUMN is_demo boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE weddings DROP COLUMN IF EXISTS is_demo;
