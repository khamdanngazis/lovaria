-- +goose Up
-- Setelan aplikasi yang bisa diganti admin tanpa deploy (T25: nomor WhatsApp
-- bantuan). Kunci → nilai teks; tanpa baris = belum diatur.
CREATE TABLE app_settings (
    key        text PRIMARY KEY,
    value      text NOT NULL,
    updated_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS app_settings;
