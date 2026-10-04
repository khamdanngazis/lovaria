# T14 — Invitation Sharing & Custom Slug

**Estimasi:** 1–2 hari · **Depends on:** T07, T12 · **Modul:** `modules/guest` (share), `modules/wedding` (slug)

## Konteks
Pasangan membagikan undangan via WhatsApp, copy link, dan sosial media (Produk §15). Custom slug termasuk Should Have.

## Scope (In)
- Template pesan undangan yang bisa diedit couple (tabel `share_templates` per wedding) dengan placeholder: `{guest_name}`, `{couple}`, `{date}`, `{link}`; default Bahasa Indonesia + opsi English
- Per tamu di daftar guest: tombol "Kirim WA" → `https://wa.me/{phone}?text={encoded}`; tombol "Salin Link" dan "Salin Pesan"
- Penanda `shared_at` per tamu (diset saat tombol diklik) + filter "belum dibagikan"
- Halaman Share: link umum (`/w/:slug`), Web Share API di mobile, tombol copy
- Custom slug: couple bisa ubah slug (validasi format, reserved words seperti `admin`, `api`, `i`, `w`), slug lama redirect 301 selama 90 hari (tabel `slug_redirects`)
- Link yang dibagikan memakai custom domain bila aktif (via fungsi `wedding.CanonicalBaseURL()` yang T15 isi), fallback domain Lunovia

## Out of Scope
- Kirim WA massal otomatis / WhatsApp Business API.

## Acceptance Criteria
- [ ] Pesan ter-encode benar (emoji, baris baru) dan terbuka di WhatsApp mobile & web
- [ ] Tamu tanpa nomor HP tetap bisa disalin link-nya
- [ ] Ubah slug tidak memutus link lama
- [ ] Preview pesan real-time saat template diedit

## Catatan untuk Agent
- Nomor sudah dinormalisasi ke `62...` oleh T07; jangan normalisasi ulang di sini.
