-- +goose Up
-- Tema yang dinonaktifkan admin untuk pasangan baru (tanpa baris = aktif).
-- Wedding yang sudah memakai tema ini tetap bisa memakainya.
CREATE TABLE disabled_themes (
    theme_id    text PRIMARY KEY,
    disabled_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS disabled_themes;
