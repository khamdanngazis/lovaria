-- name: NextOrderSeq :one
SELECT nextval('payment_order_seq')::bigint;

-- name: CreateOrder :one
INSERT INTO payment_orders (id, wedding_id, user_id, order_number, amount, currency, status, gateway, expired_at)
VALUES ($1, $2, $3, $4, $5, 'IDR', 'pending', $6, $7)
RETURNING *;

-- name: SetCheckoutURL :one
UPDATE payment_orders SET checkout_url = $3
WHERE id = $1 AND wedding_id = $2
RETURNING *;

-- name: GetOrder :one
SELECT * FROM payment_orders WHERE id = $1 AND wedding_id = $2;

-- name: GetOrderByNumberForUpdate :one
-- tenant:ignore — webhook gateway hanya membawa nomor order; wedding diambil dari barisnya.
SELECT * FROM payment_orders WHERE order_number = $1 FOR UPDATE;

-- name: LatestOrder :one
SELECT * FROM payment_orders WHERE wedding_id = $1 ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ActivePendingOrder :one
-- Order pending yang masih berlaku (dipakai ulang supaya tidak ada order ganda).
SELECT * FROM payment_orders
WHERE wedding_id = $1 AND status = 'pending' AND expired_at > sqlc.arg(now) AND checkout_url <> ''
ORDER BY created_at DESC, id DESC LIMIT 1;

-- name: ListOrders :many
SELECT * FROM payment_orders WHERE wedding_id = $1 ORDER BY created_at DESC, id DESC;

-- name: ApplyStatus :one
-- Transisi dari pending saja: order yang sudah final (paid/expired/…) tidak
-- berubah. Pengecualian: expired → paid (pembayaran masuk di detik terakhir —
-- uang yang sudah diterima gateway tetap dihormati).
UPDATE payment_orders
SET status = sqlc.arg(status),
    payment_method = sqlc.arg(payment_method),
    gateway_transaction_id = COALESCE(sqlc.narg(gateway_transaction_id), gateway_transaction_id),
    paid_at = sqlc.narg(paid_at)
WHERE id = sqlc.arg(id) AND wedding_id = sqlc.arg(wedding_id)
  AND (status = 'pending' OR (status = 'expired' AND sqlc.arg(status) = 'paid'))
RETURNING *;

-- name: SetPendingDetails :exec
-- Notifikasi "pending": catat metode & ID transaksi tanpa mengubah status.
UPDATE payment_orders
SET payment_method = sqlc.arg(payment_method),
    gateway_transaction_id = COALESCE(sqlc.narg(gateway_transaction_id), gateway_transaction_id)
WHERE id = sqlc.arg(id) AND wedding_id = sqlc.arg(wedding_id) AND status = 'pending';

-- name: ExpireStale :execrows
-- tenant:ignore — pembersihan lintas wedding: order pending yang lewat batas waktu.
UPDATE payment_orders SET status = 'expired' WHERE status = 'pending' AND expired_at <= sqlc.arg(now);

-- name: AdminListOrders :many
-- tenant:ignore — panel admin: semua order terbaru.
SELECT * FROM payment_orders ORDER BY created_at DESC, id DESC LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: InsertEvent :exec
-- tenant:ignore — log webhook mentah (belum tentu terkait order yang sah).
INSERT INTO payment_events (id, gateway, order_number, signature_ok, outcome, payload)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListEvents :many
-- tenant:ignore — penelusuran webhook per nomor order (admin / test).
SELECT * FROM payment_events WHERE order_number = $1 ORDER BY received_at DESC, id DESC;

-- name: GetOrderByNumber :one
-- tenant:ignore — halaman bayar simulasi (dev/test) hanya membawa nomor order.
SELECT * FROM payment_orders WHERE order_number = $1;
