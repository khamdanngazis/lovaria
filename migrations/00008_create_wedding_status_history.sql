-- +goose Up
-- Riwayat setiap perubahan status wedding (lifecycle, T12).
CREATE TABLE wedding_status_history (
    id            uuid PRIMARY KEY,
    wedding_id    uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    from_status   text NOT NULL,
    to_status     text NOT NULL,
    actor         text NOT NULL CHECK (actor IN ('user', 'system', 'admin')),
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX wedding_status_history_wedding_id_idx ON wedding_status_history (wedding_id, at DESC);

-- +goose Down
DROP TABLE IF EXISTS wedding_status_history;
