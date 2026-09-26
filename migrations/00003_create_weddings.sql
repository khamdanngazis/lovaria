-- +goose Up
-- weddings adalah akar tenant: id-nya menjadi wedding_id bagi semua tabel lain.
CREATE TABLE weddings (
    id             uuid PRIMARY KEY,
    owner_user_id  uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    slug           citext NOT NULL,
    title          text NOT NULL,
    wedding_date   date NOT NULL,
    description    text NOT NULL DEFAULT '',
    main_photo_url text,
    status         text NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'published', 'wedding_day', 'memory', 'archived')),
    theme_id       text NOT NULL DEFAULT 'elegant',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT weddings_slug_key UNIQUE (slug),
    CONSTRAINT weddings_slug_format CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);
CREATE INDEX weddings_owner_user_id_idx ON weddings (owner_user_id);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON weddings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE couples (
    id                uuid PRIMARY KEY,
    wedding_id        uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    groom_name        text NOT NULL,
    bride_name        text NOT NULL,
    groom_photo_url   text,
    bride_photo_url   text,
    groom_description text NOT NULL DEFAULT '',
    bride_description text NOT NULL DEFAULT '',
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT couples_wedding_id_key UNIQUE (wedding_id)
);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON couples
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS couples;
DROP TABLE IF EXISTS weddings;
