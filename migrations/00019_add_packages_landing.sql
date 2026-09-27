-- +goose Up
-- Paket yang ditampilkan di bagian harga landing page (T18), berurutan.
ALTER TABLE packages ADD COLUMN show_on_landing boolean NOT NULL DEFAULT false;
ALTER TABLE packages ADD COLUMN sort_order integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE packages DROP COLUMN IF EXISTS sort_order;
ALTER TABLE packages DROP COLUMN IF EXISTS show_on_landing;
