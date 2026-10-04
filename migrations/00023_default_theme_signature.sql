-- +goose Up
-- T21: tema bawaan wedding baru menjadi Lovoria Signature. Wedding yang sudah
-- ada tetap memakai theme_id pilihannya.
ALTER TABLE weddings ALTER COLUMN theme_id SET DEFAULT 'signature';

-- +goose Down
ALTER TABLE weddings ALTER COLUMN theme_id SET DEFAULT 'elegant';
