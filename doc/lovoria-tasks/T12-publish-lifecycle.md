# T12 — Publish & Wedding Lifecycle

**Estimasi:** 1–2 hari · **Depends on:** T09 · **Modul:** `modules/wedding` (lifecycle)

## Konteks
Lifecycle Draft → Published → Wedding Day → Memory → Archived (Produk §16) menentukan apa yang bisa diakses tamu.

## Scope (In)
- State machine eksplisit di service (`Transition(weddingID, to)`) dengan tabel transisi yang diizinkan:
  - `draft ⇄ published` (manual oleh couple: Publish/Unpublish)
  - `published → wedding_day` (otomatis pada tanggal wedding 00:00 zona wedding)
  - `wedding_day → memory` (otomatis H+1)
  - `memory → archived` (otomatis setelah N hari dari env/package, default 365; atau manual admin)
  - `archived → memory` (admin saja)
- Checklist pre-publish: nama pasangan, tanggal, minimal 1 event wajib; tampilkan yang kurang
- Background scheduler in-process (goroutine ticker tiap 10 menit) dengan advisory lock Postgres agar aman bila nanti >1 instance
- Tabel `wedding_status_history` (`wedding_id`, `from`, `to`, `actor` (user/system), `at`)
- Perilaku public per status:
  - `draft`: 404 (kecuali owner preview)
  - `published`/`wedding_day`: penuh
  - `memory`: RSVP ditutup, guestbook tetap buka, banner "Terima kasih telah menjadi bagian dari hari kami"
  - `archived`: halaman ringkas "Undangan ini telah diarsipkan"
- Tombol Publish/Unpublish di dashboard dengan konfirmasi

## Out of Scope
- Aturan package/berbayar (hanya konfigurasi durasi arsip).

## Acceptance Criteria
- [ ] Transisi ilegal ditolak dengan error jelas
- [ ] Test scheduler dengan clock yang bisa di-inject
- [ ] Riwayat status tercatat untuk setiap transisi
- [ ] Guard status dipakai T10 & T11 lewat fungsi `wedding.AllowsRSVP()`, `AllowsGuestbook()` — bukan cek string di handler

## Catatan untuk Agent
- Gunakan zona waktu wedding (dari T05) untuk perhitungan hari, bukan UTC server.
