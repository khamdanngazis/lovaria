# T10 — RSVP

**Estimasi:** 1–2 hari · **Depends on:** T07, T09 · **Modul:** `modules/guest` (rsvp), `/templates/shared/rsvp`

## Konteks
Tamu konfirmasi kehadiran langsung dari invitation (Produk §10); pasangan melihat hasilnya di dashboard.

## Scope (In)
- Komponen `RSVPSection` (shared, dipakai semua tema): Attending/Declined → jumlah orang (1..`max_pax`) → pesan opsional → submit
- Endpoint `POST /i/:code/rsvp` (htmx, balikin fragment hasil) — API-shaped: payload `{status, pax, message}`
- Hanya tersedia bila ada guest (akses via code). Akses via `/w/:slug` menampilkan pesan "Gunakan link undangan pribadi Anda untuk RSVP"
- Tamu boleh mengubah RSVP sampai tanggal wedding (setelah itu read-only)
- Pesan RSVP opsional juga bisa otomatis dibuat sebagai entri guestbook (toggle, default mati) — cukup panggil service guestbook bila T11 sudah ada
- Dashboard: halaman RSVP dengan tabel respons terbaru, filter status, total pax attending
- Rate limit per code + CSRF-less protection untuk form public (token berbasis HMAC code+timestamp di hidden field)

## Out of Scope
- Notifikasi email/WA ke couple saat RSVP masuk.

## Acceptance Criteria
- [ ] `pax` > `max_pax` ditolak di server
- [ ] RSVP tersimpan dan langsung tercermin di statistik T07
- [ ] Setelah submit, tamu melihat status RSVP-nya saat membuka link lagi
- [ ] Submit ganda cepat tidak membuat data ganda (idempotent update)
- [ ] Bekerja tanpa JS (form POST biasa fallback)

## Catatan untuk Agent
- Status `wedding` harus `published` atau `wedding_day` untuk menerima RSVP.
