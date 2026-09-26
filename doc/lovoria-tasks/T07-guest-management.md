# T07 — Guest Management

**Estimasi:** 2–3 hari · **Depends on:** T04 · **Modul:** `modules/guest`

## Konteks
Daftar tamu + invitation code personal adalah dasar personalized invitation dan RSVP (Produk §8–9). Import/export termasuk "Should Have" dan dibuat di sini karena satu domain.

## Scope (In)
- Tabel `guests` (`id`, `wedding_id`, `name`, `phone`, `email` nullable, `group_name`, `max_pax` default 1, `invitation_code` unique global, `rsvp_status` enum `pending|attending|declined` default `pending`, `rsvp_pax`, `rsvp_message`, `rsvp_at`, `attendance_status` nullable, `notes`, `last_opened_at`)
- Invitation code: 7 karakter, alfabet tanpa karakter ambigu (tanpa `0 O 1 I L`), crypto-random, retry bila bentrok
- Dashboard: list tamu dengan search, filter status & group, pagination (htmx), tambah/edit/hapus, bulk delete
- Ringkasan: total, attending, declined, pending (hitungan tamu dan hitungan pax)
- Import CSV (kolom: name, phone, email, group, max_pax) dengan preview + laporan baris error sebelum commit
- Export CSV (semua kolom termasuk link invitation lengkap)
- Normalisasi nomor HP Indonesia ke format `62xxxxxxxxxx` (dipakai T14 WhatsApp)
- Service expose: `GetByCode(code)` (lintas wedding, dipakai resolver T09), `UpdateRSVP(...)` (T10), `Stats(weddingID)` (T13), `MarkOpened(id)`

## Out of Scope
- Form RSVP public (T10), share WhatsApp (T14), QR check-in (V2).

## Acceptance Criteria
- [ ] Import 500 baris < 5 detik, baris invalid dilaporkan tanpa menggagalkan baris valid (setelah konfirmasi)
- [ ] Code unik & tidak bisa ditebak berurutan
- [ ] Export bisa dibuka di Excel/Google Sheets (UTF-8 BOM)
- [ ] Isolasi tenant diuji

## Catatan untuk Agent
- Invitation code unik secara global karena URL `lovoria.com/i/{code}` tidak memuat wedding slug.
