-- +goose Up
-- Link penerima tamu (T31): halaman pemindai dibuka tanpa akun lewat token.
-- Token = id link + tanda tangan HMAC (APP_SECRET), jadi tidak ada rahasia yang
-- disimpan di sini; mencabut link = mengisi revoked_at.
CREATE TABLE checkin_links (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);
-- Paling banyak satu link aktif per wedding.
CREATE UNIQUE INDEX checkin_links_active_idx ON checkin_links (wedding_id) WHERE revoked_at IS NULL;

-- Tamu tambahan (tanpa undangan) yang dicatat penerima tamu di pintu masuk.
CREATE TABLE checkin_walkins (
    id         uuid PRIMARY KEY,
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    name       text NOT NULL,
    pax        smallint NOT NULL DEFAULT 1 CHECK (pax BETWEEN 1 AND 20),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX checkin_walkins_wedding_id_idx ON checkin_walkins (wedding_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS checkin_walkins;
DROP TABLE IF EXISTS checkin_links;
