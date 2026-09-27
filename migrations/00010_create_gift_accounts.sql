-- +goose Up
CREATE TABLE gift_accounts (
    id             uuid PRIMARY KEY,
    wedding_id     uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    type           text NOT NULL CHECK (type IN ('bank', 'ewallet', 'address')),
    provider       text NOT NULL DEFAULT '', -- BCA, Mandiri, GoPay, …; kosong untuk alamat
    account_number text NOT NULL DEFAULT '',
    account_name   text NOT NULL DEFAULT '', -- pemilik rekening / penerima paket
    address_text   text NOT NULL DEFAULT '',
    sort_order     integer NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX gift_accounts_wedding_id_idx ON gift_accounts (wedding_id, sort_order);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON gift_accounts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE IF EXISTS gift_accounts;
