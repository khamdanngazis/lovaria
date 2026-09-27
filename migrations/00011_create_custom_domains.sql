-- +goose Up
CREATE TABLE custom_domains (
    id                  uuid PRIMARY KEY,
    -- Satu domain per wedding; mengganti domain = hapus baris lama dulu.
    wedding_id          uuid NOT NULL UNIQUE REFERENCES weddings (id) ON DELETE CASCADE,
    domain              text NOT NULL CHECK (domain = lower(domain)),
    cf_hostname_id      text NOT NULL DEFAULT '',
    status              text NOT NULL DEFAULT 'pending_verification'
        CHECK (status IN ('pending_verification', 'active', 'failed', 'removed')),
    verification_errors jsonb NOT NULL DEFAULT '[]'::jsonb,
    verified_at         timestamptz,
    last_checked_at     timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now()
);
-- Domain unik di antara yang masih dipakai ('removed' = dihapus dari Cloudflare di luar Lovoria).
CREATE UNIQUE INDEX custom_domains_domain_key ON custom_domains (domain) WHERE status <> 'removed';
CREATE INDEX custom_domains_pending_idx ON custom_domains (created_at) WHERE status = 'pending_verification';

-- +goose Down
DROP TABLE IF EXISTS custom_domains;
