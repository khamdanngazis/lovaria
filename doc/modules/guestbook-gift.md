# Buku ucapan & amplop digital (T11)

Dua modul kecil dengan pola sama: satu section di undangan publik + satu halaman kelola di dashboard.

## Buku ucapan — `src/modules/guestbook`

Tabel `guestbook_entries` (migration `00009`): `guest_id` (nullable, `ON DELETE SET NULL` — terisi bila ditulis lewat `/i/:code`), `guest_name` ≤ 100, `message` ≤ 500 karakter, `is_hidden`, `created_at`. Index `(wedding_id, created_at DESC, id DESC)`.

| Method service | Pemakai |
|---|---|
| `Post(ctx, weddingID, guestID, name, message)` | Public site (form buku ucapan & salinan pesan RSVP). Validasi panjang; kata kasar → tersimpan dengan `is_hidden = true`. |
| `Visible(ctx, weddingID, before, limit)` | Public site: pesan tampil, terbaru dulu, cursor `before` = ID entri terakhir (cursor milik wedding lain → hasil kosong). |
| `List`, `Stats`, `SetHidden`, `Delete` | Dashboard. |
| `SetFavorite(ctx, weddingID, id, favorite)` | Dashboard: tandai / lepas ucapan favorit (T19). |
| `Favorites(ctx, weddingID, n)` | Public site (bagian kenangan) & beranda dashboard: favorit yang tidak disembunyikan, terbaru dulu. |
| `OnChange(f)` | `f(weddingID)` dipanggil setelah pasangan mengubah ucapan (sembunyikan, hapus, favorit) — dipakai untuk mengosongkan cache halaman publik. |

**Filter kata kasar** (`WordFilter`): daftar bawaan `DefaultBlockedWords` + `GUESTBOOK_BLOCKED_WORDS` (dipisah koma). Dicocokkan per kata utuh setelah normalisasi (huruf kecil, leet `4→a 1→i 0→o 3→e 5→s 7→t @→a $→s`, huruf berulang dirapatkan: "anjiiing" → "anjing"). Pesan yang cocok **disembunyikan, bukan ditolak**, jadi salah deteksi bisa dipulihkan pasangan.

**Dashboard** `GET /dashboard/weddings/:id/guestbook?filter=shown|hidden&page=` (tab **Ucapan**): daftar semua pesan (termasuk yang tersembunyi), `PATCH /guestbook/:entryID` (`hidden=1|0`), `DELETE /guestbook/:entryID`, `PATCH /guestbook/:entryID/favorite` (`favorite=1|0`, tombol ★ + badge "★ Favorit"; kolom `is_favorite`, migration `00021`). Aksi htmx mengirim filter & halaman aktif (`hx-include="#guestbook-state"`) supaya daftar yang dirender ulang tetap sama; tanpa JS → redirect.

## Amplop digital — `src/modules/gift`

Tabel `gift_accounts` (migration `00010`): `type` `bank | ewallet | address`, `provider`, `account_number`, `account_name`, `address_text`, `sort_order`.

Validasi (`validate`): bank/e-wallet wajib penyedia, nomor (angka, spasi, `-`, `+`, `.`; minimal 5 digit, ≤ 40 karakter) dan nama pemilik; alamat wajib `address_text` (≤ 500), nama penerima opsional. Kolom yang tidak relevan untuk jenisnya dikosongkan.

**Dashboard** `…/gifts` (tab **Hadiah**): tambah/ubah inline (Alpine menampilkan kolom sesuai jenis; tanpa JS semua kolom tampil), hapus, naik/turun (`PATCH …/gifts/:accountID/position`). Saran penyedia lewat `<datalist>` (`Banks`, `Ewallets`), input tetap bebas.

## Section publik

Lihat [public-site.md](public-site.md#buku-ucapan--amplop-digital-t11).

## Test

- `guestbook/filter_test.go`: normalisasi & pencocokan kata utuh.
- `guestbook/service_test.go`, `gift/service_test.go`: validasi, urutan/pagination, **isolasi tenant** (list/get/update/hide/move/delete lintas wedding → kosong / `ErrNotFound`).
- `*/handler_test.go`: alur dashboard & 404 untuk user lain di semua route.
- `public-site/guestbook_test.go`: XSS di-escape, honeypot, token, kata kasar, rate limit per IP, status (draft/memory/archived), muat lebih banyak, salinan RSVP, section hadiah (tersembunyi bila kosong, nomor rekening tidak di `<head>`/OG, tidak bocor ke wedding lain).
- `guestbook/handler_test.go` `TestDashboardFavorite`: tandai/lepas (htmx & tanpa JS), urutan favorit, favorit tersembunyi tidak tampil publik, `OnChange` terpanggil.
- `public-site/public_test.go` `TestMemoryAndArchivedPages`, `TestFavoriteInvalidatesCache` (T19).
