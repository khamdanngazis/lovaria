-- +goose Up
-- Waktu event disimpan sebagai waktu lokal; zona waktunya milik wedding.
ALTER TABLE weddings ADD COLUMN timezone text NOT NULL DEFAULT 'Asia/Jakarta';

CREATE TABLE events (
    id          uuid PRIMARY KEY,
    wedding_id  uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    name        text NOT NULL,
    type        text NOT NULL DEFAULT 'other'
        CHECK (type IN ('akad', 'reception', 'engagement', 'other')),
    event_date  date NOT NULL,
    start_time  time NOT NULL,
    end_time    time,
    venue       text NOT NULL,
    address     text NOT NULL DEFAULT '',
    maps_url    text,
    latitude    double precision CHECK (latitude BETWEEN -90 AND 90),
    longitude   double precision CHECK (longitude BETWEEN -180 AND 180),
    description text NOT NULL DEFAULT '',
    sort_order  integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX events_wedding_id_idx ON events (wedding_id, sort_order);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON events
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Tanggal cerita boleh hanya tahun, tahun+bulan, atau lengkap.
CREATE TABLE love_stories (
    id          uuid PRIMARY KEY,
    wedding_id  uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    date_year   smallint NOT NULL CHECK (date_year BETWEEN 1900 AND 2100),
    date_month  smallint CHECK (date_month BETWEEN 1 AND 12),
    date_day    smallint CHECK (date_day BETWEEN 1 AND 31),
    title       text NOT NULL,
    description text NOT NULL DEFAULT '',
    photo_url   text,
    sort_order  integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT love_stories_day_needs_month CHECK (date_day IS NULL OR date_month IS NOT NULL)
);
CREATE INDEX love_stories_wedding_id_idx ON love_stories (wedding_id, sort_order);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON love_stories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS love_stories;
DROP TABLE IF EXISTS events;
ALTER TABLE weddings DROP COLUMN IF EXISTS timezone;
