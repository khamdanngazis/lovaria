-- name: GetSettings :one
SELECT * FROM wedding_theme_settings WHERE wedding_id = $1;

-- name: UpsertSettings :exec
INSERT INTO wedding_theme_settings (wedding_id, primary_color, font_heading, font_body, background_value, cover_image_url)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (wedding_id) DO UPDATE
SET primary_color = EXCLUDED.primary_color, font_heading = EXCLUDED.font_heading,
    font_body = EXCLUDED.font_body, background_value = EXCLUDED.background_value,
    cover_image_url = EXCLUDED.cover_image_url;
