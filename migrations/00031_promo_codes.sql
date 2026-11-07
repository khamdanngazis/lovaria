-- +goose Up
-- T39: kode promo untuk harga publikasi. Potongan berupa persen atau nominal
-- dari harga dasar; satu kode boleh ditandai default (tampil di landing dan
-- terisi otomatis di halaman publikasi).
CREATE TABLE promo_codes (
    id             uuid PRIMARY KEY,
    code           text NOT NULL CHECK (code <> '' AND code = upper(code)),
    discount_type  text NOT NULL CHECK (discount_type IN ('percent', 'amount')),
    discount_value bigint NOT NULL CHECK (discount_value > 0),
    active         boolean NOT NULL DEFAULT true,
    is_default     boolean NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promo_codes_code_key UNIQUE (code),
    CONSTRAINT promo_codes_percent_range CHECK (discount_type <> 'percent' OR discount_value < 100)
);
-- Paling banyak satu kode default.
CREATE UNIQUE INDEX promo_codes_one_default ON promo_codes ((true)) WHERE is_default;
CREATE TRIGGER set_updated_at BEFORE UPDATE ON promo_codes
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Order mencatat kode yang dipakai dan harga sebelum potongan.
ALTER TABLE payment_orders
    ADD COLUMN promo_code      text NOT NULL DEFAULT '',
    ADD COLUMN original_amount bigint;
UPDATE payment_orders SET original_amount = amount;
ALTER TABLE payment_orders
    ALTER COLUMN original_amount SET NOT NULL,
    ADD CONSTRAINT payment_orders_original_amount_check CHECK (original_amount >= amount);

-- +goose Down
ALTER TABLE payment_orders
    DROP CONSTRAINT IF EXISTS payment_orders_original_amount_check,
    DROP COLUMN IF EXISTS original_amount,
    DROP COLUMN IF EXISTS promo_code;
DROP TABLE IF EXISTS promo_codes;
