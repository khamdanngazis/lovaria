-- +goose Up
-- Check-in tamu dengan QR di hari H (T31). Fitur mati secara bawaan per
-- wedding. Kolom lama guests.attendance_status ikut diisi 'present' saat
-- check-in.
ALTER TABLE weddings ADD COLUMN checkin_enabled boolean NOT NULL DEFAULT false;

ALTER TABLE guests
    ADD COLUMN checked_in_at  timestamptz,
    ADD COLUMN checked_in_pax smallint CHECK (checked_in_pax IS NULL OR checked_in_pax BETWEEN 1 AND 20),
    ADD COLUMN checked_in_via text CHECK (checked_in_via IN ('scan', 'manual', 'owner'));

-- +goose Down
ALTER TABLE guests
    DROP COLUMN IF EXISTS checked_in_via,
    DROP COLUMN IF EXISTS checked_in_pax,
    DROP COLUMN IF EXISTS checked_in_at;
ALTER TABLE weddings DROP COLUMN IF EXISTS checkin_enabled;
