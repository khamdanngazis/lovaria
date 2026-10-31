-- +goose Up
-- Preset warna kurasi per wedding (T33); NULL = preset pertama tema. Hanya
-- dipakai tema yang punya daftar preset di registry (saat ini Signature).
ALTER TABLE wedding_theme_settings ADD COLUMN color_preset text;

-- +goose Down
ALTER TABLE wedding_theme_settings DROP COLUMN IF EXISTS color_preset;
