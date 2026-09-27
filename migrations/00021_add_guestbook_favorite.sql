-- +goose Up
-- Ucapan favorit pilihan pasangan (T19): tampil paling atas di mode Kenangan & arsip.
ALTER TABLE guestbook_entries ADD COLUMN is_favorite boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE guestbook_entries DROP COLUMN IF EXISTS is_favorite;
