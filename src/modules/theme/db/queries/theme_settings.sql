-- name: GetSettings :one
SELECT * FROM wedding_theme_settings WHERE wedding_id = $1;

-- name: UpsertSettings :exec
INSERT INTO wedding_theme_settings (
    wedding_id, primary_color, font_heading, font_body, background_value, cover_image_url,
    music_url, music_enabled, quote_text, quote_source, greeting_text, closing_text,
    hidden_sections, section_order
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (wedding_id) DO UPDATE
SET primary_color = EXCLUDED.primary_color, font_heading = EXCLUDED.font_heading,
    font_body = EXCLUDED.font_body, background_value = EXCLUDED.background_value,
    cover_image_url = EXCLUDED.cover_image_url,
    music_url = EXCLUDED.music_url, music_enabled = EXCLUDED.music_enabled,
    quote_text = EXCLUDED.quote_text, quote_source = EXCLUDED.quote_source,
    greeting_text = EXCLUDED.greeting_text, closing_text = EXCLUDED.closing_text,
    hidden_sections = EXCLUDED.hidden_sections, section_order = EXCLUDED.section_order;

-- name: SetMusicUpload :exec
-- Unggahan musik baru: dipilih & diaktifkan sekaligus (baris dibuat bila belum ada).
INSERT INTO wedding_theme_settings (wedding_id, music_upload_key, music_upload_bytes, music_url, music_enabled)
VALUES ($1, $2, $3, $4, true)
ON CONFLICT (wedding_id) DO UPDATE
SET music_upload_key = EXCLUDED.music_upload_key, music_upload_bytes = EXCLUDED.music_upload_bytes,
    music_url = EXCLUDED.music_url, music_enabled = true;

-- name: ClearMusicUpload :exec
-- Hapus unggahan; bila sedang dipakai, musik ikut dikosongkan.
UPDATE wedding_theme_settings
SET music_upload_key = NULL, music_upload_bytes = 0,
    music_url = CASE WHEN music_url = sqlc.arg(upload_url)::text THEN NULL ELSE music_url END
WHERE wedding_id = $1;
