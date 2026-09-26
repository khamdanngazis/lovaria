# T11 — Guestbook & Digital Gift

**Estimasi:** 2 hari · **Depends on:** T09 · **Modul:** `modules/guestbook`, `modules/gift`

## Konteks
Dua fitur public yang kecil namun wajib MVP (Produk §12–13). Digabung karena pola serupa: satu section public + satu halaman kelola di dashboard.

## Scope (In)
**Guestbook**
- Tabel `guestbook_entries` (`id`, `wedding_id`, `guest_id` nullable, `guest_name`, `message` max 500 char, `is_hidden` bool, `created_at`)
- Section public: form (nama auto-terisi dari guest bila via code) + daftar pesan terbaru dengan "load more" (htmx)
- `POST /i/:code/guestbook` dan `POST /w/:slug/guestbook` — honeypot field + rate limit per IP
- Filter kata kasar sederhana (wordlist bisa dikonfigurasi) → otomatis `is_hidden`
- Dashboard: list pesan, sembunyikan/tampilkan, hapus
- Guestbook tetap bisa diisi saat status `memory` (sesuai konsep Remember)

**Digital Gift**
- Tabel `gift_accounts` (`id`, `wedding_id`, `type` enum `bank|ewallet|address`, `provider` (BCA, Mandiri, GoPay, dst.), `account_number`, `account_name`, `address_text`, `sort_order`)
- Dashboard CRUD
- Section public: kartu per akun + tombol "Salin Nomor" (Alpine clipboard) + toast; alamat kirim hadiah fisik
- Section disembunyikan bila tidak ada akun

## Out of Scope
- Foto/voice/video guestbook, payment processing.

## Acceptance Criteria
- [ ] Pesan XSS di-escape saat render
- [ ] Spam bot (honeypot terisi) ditolak diam-diam
- [ ] Tombol salin bekerja di iOS Safari & Android Chrome
- [ ] Isolasi tenant diuji di kedua modul

## Catatan untuk Agent
- Jangan tampilkan nomor rekening di OG meta atau halaman lain selain section gift.
