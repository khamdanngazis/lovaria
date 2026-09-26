-- +goose Up
CREATE TABLE guests (
    id                uuid PRIMARY KEY,
    wedding_id        uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    name              text NOT NULL,
    phone             text NOT NULL DEFAULT '', -- dinormalisasi: 62xxxxxxxxxx
    email             citext,
    group_name        text NOT NULL DEFAULT '',
    max_pax           smallint NOT NULL DEFAULT 1 CHECK (max_pax BETWEEN 1 AND 20),
    -- Unik global: URL /i/{code} tidak memuat slug wedding.
    invitation_code   text NOT NULL CHECK (invitation_code ~ '^[2-9A-HJKMNP-Z]{7}$'),
    rsvp_status       text NOT NULL DEFAULT 'pending'
        CHECK (rsvp_status IN ('pending', 'attending', 'declined')),
    rsvp_pax          smallint NOT NULL DEFAULT 0 CHECK (rsvp_pax >= 0),
    rsvp_message      text NOT NULL DEFAULT '',
    rsvp_at           timestamptz,
    attendance_status text CHECK (attendance_status IN ('present', 'absent')),
    notes             text NOT NULL DEFAULT '',
    last_opened_at    timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT guests_invitation_code_key UNIQUE (invitation_code)
);
CREATE INDEX guests_wedding_id_idx ON guests (wedding_id, created_at);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON guests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS guests;
