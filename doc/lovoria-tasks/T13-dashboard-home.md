# T13 — Dashboard Home & RSVP Summary

**Estimasi:** 1–2 hari · **Depends on:** T10, T11, T12 · **Modul:** `/src/dashboard`

## Konteks
Halaman pertama couple setelah login (Produk §4), plus "Basic analytics" dan "RSVP dashboard" dari Should Have.

## Scope (In)
- Dashboard home per wedding menampilkan: nama wedding, tanggal + hitung mundur hari, status + tombol aksi (Publish/Unpublish), ringkasan guest & RSVP (attending/declined/pending, total pax), 5 pesan guestbook terbaru, jumlah & thumbnail gallery, penggunaan storage, invitation link umum
- Tombol cepat: Open Website, Manage Wedding, Share Invitation (link ke T14)
- Checklist onboarding (progress): info pasangan, event, love story, gallery, tema, tamu, publish
- Basic analytics: jumlah invitation yang dibuka (`last_opened_at` not null), open rate, RSVP rate — tanpa tracking pihak ketiga
- Navigasi dashboard final (sidebar desktop, bottom nav mobile) yang menghubungkan semua modul
- Halaman list wedding bila user punya >1 wedding

## Out of Scope
- Grafik time-series/advanced analytics (V2).

## Acceptance Criteria
- [ ] Semua angka berasal dari service modul terkait (tidak ada query lintas modul di dashboard)
- [ ] Load dashboard < 300 ms di data 500 tamu (lokal)
- [ ] Layout nyaman di 375px
- [ ] User baru melihat checklist onboarding yang benar

## Catatan untuk Agent
- Dashboard adalah agregator: panggil `guest.Stats`, `guestbook.Recent`, `gallery.Summary`, dst.
