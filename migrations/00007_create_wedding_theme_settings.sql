-- +goose Up
-- Override tampilan per wedding. Kolom NULL = pakai default tema (weddings.theme_id).
CREATE TABLE wedding_theme_settings (
    wedding_id       uuid PRIMARY KEY REFERENCES weddings (id) ON DELETE CASCADE,
    primary_color    text CHECK (primary_color ~ '^#[0-9a-f]{6}$'),
    font_heading     text,
    font_body        text,
    background_value text,
    cover_image_url  text,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON wedding_theme_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS wedding_theme_settings;
