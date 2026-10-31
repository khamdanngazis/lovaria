-- +goose Up
-- Check-in QR aktif secara bawaan untuk wedding baru; pasangan tetap bisa
-- mematikannya. Wedding yang sudah ada tidak diubah.
ALTER TABLE weddings ALTER COLUMN checkin_enabled SET DEFAULT true;

-- +goose Down
ALTER TABLE weddings ALTER COLUMN checkin_enabled SET DEFAULT false;
