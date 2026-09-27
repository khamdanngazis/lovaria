-- name: CreateAccount :one
INSERT INTO gift_accounts (id, wedding_id, type, provider, account_number, account_name, address_text, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7,
    (SELECT COALESCE(max(sort_order) + 1, 0) FROM gift_accounts WHERE wedding_id = $2))
RETURNING *;

-- name: GetAccount :one
SELECT * FROM gift_accounts WHERE id = $1 AND wedding_id = $2;

-- name: UpdateAccount :one
UPDATE gift_accounts
SET type = $3, provider = $4, account_number = $5, account_name = $6, address_text = $7
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: DeleteAccount :execrows
DELETE FROM gift_accounts WHERE id = $1 AND wedding_id = $2;

-- name: ListAccounts :many
SELECT * FROM gift_accounts WHERE wedding_id = $1 ORDER BY sort_order, created_at;

-- name: ListAccountsForUpdate :many
SELECT * FROM gift_accounts WHERE wedding_id = $1 ORDER BY sort_order, created_at FOR UPDATE;

-- name: SetAccountSortOrder :exec
UPDATE gift_accounts SET sort_order = $3 WHERE id = $1 AND wedding_id = $2;
