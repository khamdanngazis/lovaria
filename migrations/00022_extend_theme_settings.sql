-- +goose Up
-- T20: personalisasi undangan — musik latar, kutipan, teks sapaan/penutup,
-- susunan & visibilitas bagian.
ALTER TABLE wedding_theme_settings
    ADD COLUMN music_url          text,
    ADD COLUMN music_enabled      boolean NOT NULL DEFAULT false,
    -- Unggahan MP3 milik wedding (satu per wedding): key storage + ukuran untuk
    -- mengembalikan kuota saat diganti / dihapus.
    ADD COLUMN music_upload_key   text,
    ADD COLUMN music_upload_bytes bigint  NOT NULL DEFAULT 0 CHECK (music_upload_bytes >= 0),
    ADD COLUMN quote_text         text CHECK (char_length(quote_text) <= 500),
    ADD COLUMN quote_source       text CHECK (char_length(quote_source) <= 100),
    ADD COLUMN greeting_text      text CHECK (char_length(greeting_text) <= 100),
    ADD COLUMN closing_text       text CHECK (char_length(closing_text) <= 500),
    ADD COLUMN hidden_sections    text[]  NOT NULL DEFAULT '{}',
    ADD COLUMN section_order      text[]  NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE wedding_theme_settings
    DROP COLUMN music_url,
    DROP COLUMN music_enabled,
    DROP COLUMN music_upload_key,
    DROP COLUMN music_upload_bytes,
    DROP COLUMN quote_text,
    DROP COLUMN quote_source,
    DROP COLUMN greeting_text,
    DROP COLUMN closing_text,
    DROP COLUMN hidden_sections,
    DROP COLUMN section_order;
