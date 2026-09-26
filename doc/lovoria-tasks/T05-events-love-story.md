# T05 — Events & Love Story

**Estimasi:** 2 hari · **Depends on:** T04 · **Modul:** `modules/wedding` (sub-package `event`, `story`)

## Konteks
Detail acara (akad, resepsi, dll.) dan perjalanan cerita pasangan (Produk §5–6). Keduanya CRUD berurutan dengan pola UI serupa, jadi digabung dalam satu task.

## Scope (In)
**Events**
- Tabel `events` (`id`, `wedding_id`, `name`, `type` enum `akad|reception|engagement|other`, `date`, `start_time`, `end_time` nullable, `venue`, `address`, `maps_url`, `latitude`/`longitude` nullable, `description`, `sort_order`)
- CRUD dashboard dengan htmx (tambah/edit inline tanpa reload penuh)
- Validasi `maps_url` harus domain Google Maps; parse lat/lng bila tersedia di URL
- Urutan default by date+start_time, bisa di-reorder manual

**Love Story**
- Tabel `love_stories` (`id`, `wedding_id`, `date` (boleh hanya tahun), `title`, `description`, `photo_url` nullable, `sort_order`)
- CRUD + reorder (drag handle sederhana via Alpine atau tombol naik/turun)
- Field foto: URL dulu, upload disambungkan setelah T06 tersedia (buat komponen upload reusable jika T06 sudah merge)

## Out of Scope
- Tampilan public (T09), countdown (V2).

## Acceptance Criteria
- [ ] CRUD events & stories hanya untuk owner wedding (pakai `RequireWeddingOwner`)
- [ ] Reorder tersimpan dan konsisten
- [ ] Semua query memfilter `wedding_id`
- [ ] Service expose `ListEvents(weddingID)`, `ListStories(weddingID)` untuk T09
- [ ] Test service + handler

## Catatan untuk Agent
- Simpan waktu event sebagai waktu lokal + kolom `timezone` di `weddings` (default `Asia/Jakarta`); tambahkan migration kolom itu di task ini bila belum ada.
