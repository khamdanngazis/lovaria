# T16 — Admin Panel

**Estimasi:** 2 hari · **Depends on:** T12 · **Modul:** `modules/admin`

## Konteks
Panel internal sederhana (Produk §2C, §17). MVP: fokus visibilitas dan support, bukan billing lengkap.

## Scope (In)
- Route `/admin/*` dengan `RequireRole(admin)`
- **Customers**: list user (search email/nama), detail user + wedding miliknya, nonaktifkan user (kolom `disabled_at` + blokir login)
- **Weddings**: list dengan filter status, tanggal, sort by storage; detail (ringkasan guest/RSVP, domain, storage); aksi admin: ubah status (termasuk `archived → memory`), buka website, "impersonate read-only" (lihat dashboard sebagai couple tanpa bisa mengubah) dengan audit log
- **Themes**: daftar tema dari registry + jumlah pemakai; toggle aktif/nonaktif tema untuk couple baru
- **Packages**: tabel `packages` (nama, storage limit, durasi arsip, harga display) + assign ke wedding (tanpa payment gateway)
- **Storage usage**: total & per wedding, top 20
- **Custom domains**: daftar + status + jumlah terhadap kuota 100
- **Audit log**: tabel `admin_audit_logs` untuk setiap aksi admin

## Out of Scope
- Orders/Payments otomatis, sistem tiket support (cukup tombol mailto/WA dari detail user).

## Acceptance Criteria
- [ ] Couple tidak bisa mengakses satu pun route admin (test)
- [ ] Setiap aksi tulis admin tercatat di audit log
- [ ] Admin mengakses data lewat service modul (bukan query tabel modul lain), kecuali view laporan read-only yang didokumentasikan
- [ ] List paginated dan responsif pada 1.000 wedding (seed)

## Catatan untuk Agent
- UI boleh sederhana (tabel Tailwind), tidak perlu mobile-optimized penuh.
