# T04 — Wedding Core & Setup Wizard

**Estimasi:** 2–3 hari · **Depends on:** T03 · **Modul:** `modules/wedding`, `/src/dashboard`

## Konteks
Entitas pusat seluruh platform (Produk §5). Semua modul lain bergantung pada `wedding_id` dan authorization "user ini pemilik wedding ini".

## Scope (In)
- Tabel `weddings` (`id`, `owner_user_id`, `slug unique`, `title`, `wedding_date`, `description`, `main_photo_url` nullable, `status` enum `draft|published|wedding_day|memory|archived` default `draft`, `theme_id` default `elegant`, timestamps)
- Tabel `couples` (`id`, `wedding_id unique`, `groom_name`, `bride_name`, `groom_photo_url`, `bride_photo_url`, `groom_description`, `bride_description`)
- Service: `CreateWedding`, `GetWedding`, `UpdateWeddingInfo`, `UpdateCouple`, `ListWeddingsByOwner`
- **Authorization middleware** `RequireWeddingOwner`: route `/dashboard/weddings/:weddingID/*` memastikan user adalah owner, lalu menyimpan `wedding_id` di context. Middleware ini dipakai semua modul dashboard.
- Setup wizard (multi-step, htmx): 1) nama pasangan 2) judul & tanggal 3) deskripsi → selesai ke dashboard wedding
- Halaman edit couple info & wedding info
- Slug auto-generate dari nama pasangan (`khamdan-sarah`, tambahkan suffix bila bentrok)
- Upload foto profil/main photo: **field URL saja** di task ini; tombol upload disambungkan di T06

## Out of Scope
- Events & love story (T05), status transitions (T12), tema (T08).

## Detail Teknis
- Satu user MVP boleh punya >1 wedding (dashboard list), tapi UI dioptimalkan untuk 1.
- Ekspose `wedding.Service` interface untuk dipakai modul lain (read-only getter).

## Acceptance Criteria
- [ ] User baru bisa menyelesaikan wizard dan melihat wedding di dashboard
- [ ] User A tidak bisa mengakses/mengubah wedding milik user B (test eksplisit, harus 404)
- [ ] Validasi: nama wajib, tanggal valid, slug hanya `[a-z0-9-]`
- [ ] Form bekerja di mobile 375px

## Catatan untuk Agent
- Balas 404 (bukan 403) untuk wedding milik orang lain agar tidak membocorkan keberadaan ID.
