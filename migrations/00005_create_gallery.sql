-- +goose Up
-- Total byte objek (foto + thumbnail) milik wedding di storage; dijaga oleh modul wedding.
ALTER TABLE weddings ADD COLUMN storage_used_bytes bigint NOT NULL DEFAULT 0
    CHECK (storage_used_bytes >= 0);

CREATE TABLE gallery_items (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    category   text NOT NULL DEFAULT 'wedding'
        CHECK (category IN ('cover', 'couple', 'prewedding', 'wedding')),
    object_key text NOT NULL,
    thumb_key  text NOT NULL,
    url        text NOT NULL,
    thumb_url  text NOT NULL,
    width      integer NOT NULL CHECK (width > 0),
    height     integer NOT NULL CHECK (height > 0),
    size_bytes bigint NOT NULL CHECK (size_bytes >= 0), -- foto + thumbnail
    sort_order integer NOT NULL DEFAULT 0,
    caption    text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gallery_items_wedding_id_idx ON gallery_items (wedding_id, sort_order);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON gallery_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS gallery_items;
ALTER TABLE weddings DROP COLUMN IF EXISTS storage_used_bytes;
