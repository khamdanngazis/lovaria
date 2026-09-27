-- +goose Up
-- Template pesan undangan per wedding (tanpa baris = template bawaan).
-- Placeholder: {guest_name}, {couple}, {date}, {link}.
CREATE TABLE share_templates (
    wedding_id uuid PRIMARY KEY REFERENCES weddings (id) ON DELETE CASCADE,
    language   text NOT NULL DEFAULT 'id' CHECK (language IN ('id', 'en')),
    body       text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON share_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS share_templates;
