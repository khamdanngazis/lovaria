-- +goose Up
-- Slug lama setelah pasangan mengganti slug: /w/<lama> → 301 ke slug baru
-- selama 90 hari supaya link yang sudah dibagikan tidak putus.
CREATE TABLE slug_redirects (
    old_slug   citext PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX slug_redirects_wedding_id_idx ON slug_redirects (wedding_id);

-- +goose Down
DROP TABLE IF EXISTS slug_redirects;
