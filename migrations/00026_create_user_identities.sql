-- +goose Up
-- Identitas login pihak ketiga (T30: Sign in with Google). Satu akun Google
-- (provider + subject) terhubung ke tepat satu user; satu user boleh punya
-- beberapa identitas.
CREATE TABLE user_identities (
    provider   text NOT NULL,
    subject    text NOT NULL,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    email      citext NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);
CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);

-- +goose Down
DROP TABLE IF EXISTS user_identities;
