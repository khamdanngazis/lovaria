-- +goose Up
-- T23: pembayaran sekali per wedding, ditagih saat publikasi.

-- Hak publikasi wedding. NULL = belum lunas. paid_source mencatat asalnya:
-- gateway (webhook), admin (tandai lunas manual), grandfathered (sudah terbit
-- sebelum fitur ini), demo (undangan contoh).
ALTER TABLE weddings
    ADD COLUMN paid_at     timestamptz,
    ADD COLUMN paid_source text CHECK (paid_source IN ('gateway', 'admin', 'grandfathered', 'demo')),
    ADD CONSTRAINT weddings_paid_consistent CHECK ((paid_at IS NULL) = (paid_source IS NULL));

-- Undangan yang sudah terbit (atau pernah terbit) dan undangan contoh tidak
-- boleh tiba-tiba terkunci: dianggap lunas. Draf lama wajib bayar saat terbit.
UPDATE weddings SET paid_at = now(), paid_source = CASE WHEN is_demo THEN 'demo' ELSE 'grandfathered' END
WHERE status <> 'draft' OR is_demo;

-- Nomor order: LVR-YYYYMMDD-000001.
CREATE SEQUENCE payment_order_seq;

-- Satu baris per percobaan bayar. Wedding boleh punya banyak percobaan; hanya
-- yang 'paid' membuka publikasi ("belum bayar" = tidak ada baris paid).
CREATE TABLE payment_orders (
    id                     uuid PRIMARY KEY,
    wedding_id             uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    user_id                uuid NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    order_number           text NOT NULL,
    amount                 bigint NOT NULL CHECK (amount > 0),
    currency               text NOT NULL DEFAULT 'IDR' CHECK (currency = 'IDR'),
    status                 text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'paid', 'expired', 'failed', 'cancelled')),
    payment_method         text NOT NULL DEFAULT '',
    gateway                text NOT NULL,
    gateway_transaction_id text,
    checkout_url           text NOT NULL DEFAULT '',
    expired_at             timestamptz NOT NULL,
    paid_at                timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT payment_orders_order_number_key UNIQUE (order_number),
    CONSTRAINT payment_orders_paid_has_time CHECK ((status = 'paid') = (paid_at IS NOT NULL))
);
CREATE INDEX payment_orders_wedding_id_idx ON payment_orders (wedding_id, created_at DESC);
-- Satu transaksi gateway hanya boleh menempel ke satu order (anti aktivasi ganda).
CREATE UNIQUE INDEX payment_orders_gateway_tx_key ON payment_orders (gateway, gateway_transaction_id)
    WHERE gateway_transaction_id IS NOT NULL;
CREATE TRIGGER set_updated_at BEFORE UPDATE ON payment_orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Log setiap webhook yang diterima (termasuk yang ditolak) untuk penelusuran.
CREATE TABLE payment_events (
    id           uuid PRIMARY KEY,
    gateway      text NOT NULL,
    order_number text NOT NULL DEFAULT '',
    signature_ok boolean NOT NULL,
    outcome      text NOT NULL, -- applied | duplicate | ignored | rejected:<alasan>
    payload      jsonb NOT NULL,
    received_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX payment_events_order_number_idx ON payment_events (order_number, received_at DESC);

-- +goose Down
DROP TABLE IF EXISTS payment_events;
DROP TABLE IF EXISTS payment_orders;
DROP SEQUENCE IF EXISTS payment_order_seq;
ALTER TABLE weddings
    DROP CONSTRAINT IF EXISTS weddings_paid_consistent,
    DROP COLUMN IF EXISTS paid_source,
    DROP COLUMN IF EXISTS paid_at;
