-- +goose Up
CREATE TABLE guestbook_entries (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    -- Diisi bila ditulis lewat link tamu (/i/:code); tamu dihapus → entri tetap ada.
    guest_id   uuid REFERENCES guests (id) ON DELETE SET NULL,
    guest_name text NOT NULL CHECK (char_length(guest_name) BETWEEN 1 AND 100),
    message    text NOT NULL CHECK (char_length(message) BETWEEN 1 AND 500),
    -- Disembunyikan pasangan, atau otomatis oleh filter kata kasar.
    is_hidden  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX guestbook_entries_wedding_id_idx ON guestbook_entries (wedding_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS guestbook_entries;
