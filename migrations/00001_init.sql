-- +goose Up
-- Extension dipasang di schema public supaya tersedia untuk semua schema.
CREATE EXTENSION IF NOT EXISTS citext WITH SCHEMA public;
CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;

-- set_updated_at dipasang sebagai trigger BEFORE UPDATE di setiap tabel yang punya updated_at:
--   CREATE TRIGGER set_updated_at BEFORE UPDATE ON <tabel>
--     FOR EACH ROW EXECUTE FUNCTION set_updated_at();
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS pgcrypto;
DROP EXTENSION IF EXISTS citext;
