# Pembayaran publikasi (T23)

**Rp149.000 sekali bayar per wedding, ditagih hanya saat dipublikasikan.** Membuat, mengubah, dan melihat pratinjau undangan tetap gratis. Tidak ada langganan.

Kode: `src/modules/payment` (order, gateway, webhook, halaman publikasi) · gerbang publikasi di `src/modules/wedding` (`paid_at`, `IsPaid`, `MarkPaid`, `ErrPaymentRequired`).

> Status: PR 1 (inti pembayaran) selesai. Menyusul di PR 2: konfirmasi password di registrasi, slug di wizard, gerbang custom domain, harga tunggal di landing, dan aksi admin "Tandai lunas".

## Dua status yang terpisah

| | Nilai | Disimpan di |
|---|---|---|
| Status wedding | `draft → published → wedding_day → memory → archived` (lifecycle T12, tidak berubah) | `weddings.status` |
| Hak publikasi | belum lunas / lunas | `weddings.paid_at` + `paid_source` (`gateway`, `admin`, `grandfathered`, `demo`) |
| Status order | `pending`, `paid`, `expired`, `failed`, `cancelled` | `payment_orders.status` |

"UNPAID" di dokumen produk = wedding tanpa order `paid` (bukan baris order).

**Aturan publikasi**: `wedding.Service.Transition(…, published, …)` menolak wedding belum lunas dengan `ErrPaymentRequired` — untuk user maupun admin, dicek di dalam transaksi yang mengunci baris wedding. Setelah lunas, tarik publikasi lalu terbit lagi tidak perlu bayar ulang. Setelah lunas pasangan **tetap menekan Publikasikan** sendiri (tidak otomatis terbit).

**Data lama** (migration `00024`): wedding yang sudah terbit (status selain `draft`) ditandai `grandfathered`, undangan contoh `demo`. Draf lama wajib bayar saat dipublikasikan.

## Alur

```
Beranda (Draft · Belum dibayar)
   └─ Publikasikan → GET  …/publish          harga, yang didapat, tombol bayar
        └─ POST …/payment                     buat / pakai ulang order → redirect ke halaman bayar gateway
             └─ (gateway) ── webhook ──▶ POST /webhooks/<gateway>   ← sumber kebenaran
             └─ GET  …/payment/return         status; htmx memeriksa ulang tiap 4 detik selagi pending
                  └─ Lunas ✓ → …/publish → Publikasikan (PATCH …/status) → terbit
```

- **CSP**: tombol "Bayar" adalah form POST yang dibalas redirect ke halaman bayar gateway. Browser menerapkan `form-action` ke seluruh rantai redirect, jadi domain halaman bayar harus terdaftar di `checkoutOrigins` (`src/platform/server/headers.go`) — tanpa itu redirect diblokir diam-diam dan tombol terasa tidak berfungsi (`TestCSPAllowsCheckoutRedirect`). Gateway baru = tambah domainnya di sana. "Lanjutkan pembayaran" memakai tautan langsung, tidak bergantung pada redirect.
- **Redirect browser tidak pernah menandai lunas.** Halaman kembali hanya membaca status; `Service.Refresh` menanyakan status langsung ke gateway supaya pasangan tidak menunggu webhook, lewat jalur penerapan yang sama.
- Order `pending` yang masih berlaku dipakai ulang (klik "Bayar" dua kali ≠ dua order). Setelah `expired`/`failed`/`cancelled`, percobaan berikutnya membuat order baru untuk wedding yang sama.
- Nomor order: `LVR-YYYYMMDD-000001` (tanggal WIB + sequence `payment_order_seq`).

## Webhook (`Service.HandleNotification`)

1. Gateway memverifikasi **tanda tangan**; gagal → 403, dicatat `rejected:signature`.
2. Untuk status lunas: **konfirmasi ulang** ke gateway (`FetchStatus`); tidak terkonfirmasi → 422 `rejected:unconfirmed`.
3. Order dicari dari nomornya dan **dikunci** (`SELECT … FOR UPDATE`). Order tak dikenal dengan tanda tangan sah (mis. tombol "Test notification" di dashboard Midtrans, yang memakai `order_id` `payment_notif_test_…`) → dicatat `rejected:unknown-order` dan dibalas **200** supaya gateway tidak mengirim ulang; tidak ada yang berubah.
4. **Nominal & mata uang** harus sama dengan order; beda → 422 `rejected:amount`.
5. Status diterapkan sekali. `paid` final: notifikasi ulang → `duplicate` (200), `expire`/`deny` yang terlambat → `ignored`. Pengecualian: `expired → paid` (pembayaran masuk di detik terakhir tetap dihormati).
6. Order `paid` → `wedding.MarkPaid(…, "gateway")` (idempoten; diulang pada notifikasi duplikat supaya pulih bila langkah ini sebelumnya gagal).

Disimpan: `gateway_transaction_id` (unik per gateway), `payment_method`, `paid_at`. **Setiap webhook dicatat di `payment_events`** (payload, tanda tangan sah/tidak, hasil) — termasuk yang ditolak. Route `/webhooks/*` dikecualikan dari CSRF (`server.WebhookPrefix`): tidak ada sesi; keasliannya dari tanda tangan. Error 5xx membuat gateway mengirim ulang.

## Gateway

Interface `payment.Gateway` (`CreateCheckout`, `ParseNotification`, `FetchStatus`):

- **Midtrans** (`midtrans.go`, `net/http` tanpa SDK): Snap mode **redirect** (`POST /snap/v1/transactions` → `redirect_url`) — halaman bayar di domain Midtrans, tanpa script pihak ketiga di Lovoria (CSP tetap ketat). Tanda tangan `SHA512(order_id + status_code + gross_amount + server_key)`. Pemetaan: `settlement` / `capture`+`accept` → paid; `pending` → pending; `expire` → expired; `cancel` → cancelled; `deny`/`failure` → failed. Status lain (refund, chargeback) ditolak dan hanya tercatat di log event.
- **Fake** (`fake.go`): simulasi untuk development, test, dan e2e. Halaman bayar `/payment/simulasi/<order>` (wajib login, hanya pemilik order) mengirim "webhook" bertanda tangan HMAC lewat jalur yang sama. **Config menolak `PAYMENT_GATEWAY=fake` di production.**

## Konfigurasi

| Env | Bawaan | Keterangan |
|---|---|---|
| `PAYMENT_GATEWAY` | kosong | `midtrans` \| `fake`. Kosong → pembayaran belum tersedia: wedding baru belum bisa terbit (halaman publikasi menampilkan pemberitahuan). |
| `MIDTRANS_SERVER_KEY` | — | Wajib bila gateway `midtrans`. Hanya di Railway. |
| `MIDTRANS_ENV` | sandbox | Hanya nilai `production` yang memakai endpoint produksi. |
| `PUBLISH_PRICE_IDR` | `149000` | Harga per wedding. |
| `PAYMENT_EXPIRY_HOURS` | `24` | Masa berlaku satu percobaan bayar (1–168). |

Setup Midtrans & uji sandbox: lihat [runbook](../runbook.md).

## Test

- `payment/midtrans_test.go`: tanda tangan (kunci lain, isi diubah), pemetaan status, nominal pecahan, Snap & Get Status terhadap server tiruan.
- `payment/service_test.go`: nomor order, pemakaian ulang order pending, order baru setelah kedaluwarsa/gagal, webhook lunas idempoten (3× kirim ulang = satu aktivasi), status terlambat diabaikan, penolakan (tanda tangan, nominal, order tak dikenal, tidak terkonfirmasi), lunas setelah kedaluwarsa lokal, isolasi tenant, alur HTTP lengkap (webhook tanpa cookie/CSRF), halaman simulasi.
- `wedding/lifecycle_test.go`: publikasi ditolak sebelum lunas (user & admin), `MarkPaid` idempoten, terbit ulang tanpa bayar.
- `e2e/tests/smoke.spec.js`: draf → halaman harga → bayar (simulasi) → lunas → publikasikan; undangan 404 untuk publik sebelum terbit.
