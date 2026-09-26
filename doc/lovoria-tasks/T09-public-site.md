# T09 — Public Site: Resolver, Rendering & Personalized Invitation

**Estimasi:** 2–3 hari · **Depends on:** T05, T06, T07, T08 · **Modul:** `/src/public-site`

## Konteks
Halaman yang dilihat tamu (Produk §7–8). Harus mobile-first, SSR, dengan OG tag dinamis untuk preview WhatsApp (Arsitektur §7).

## Scope (In)
- **Middleware `ResolveWedding`** (satu-satunya tempat resolusi, Arsitektur §3 aturan 5):
  1. Host header cocok dengan custom domain aktif → `wedding_id` (lookup lewat interface `DomainLookup`; stub mengembalikan not-found sampai T15)
  2. Fallback: path `/i/:code` → guest → wedding; atau `/w/:slug` → wedding (akses umum tanpa nama tamu)
  3. Hasil disimpan di context: `wedding`, `guest` (nullable)
- Route: `GET /i/:code`, `GET /w/:slug`, dan pada custom domain `GET /` + `GET /i/:code`
- Susun `weddingView` DTO dari service modul (wedding, couple, events, stories, gallery, theme settings) lalu panggil `theme.Render`
- Struktur halaman sesuai Produk §7: Opening → Couple → Date → Love Story → Events → Gallery → RSVP → Guestbook → Gift → Closing
- Opening dengan greeting personal "Dear {guest.name}" + tombol "Buka Undangan"; tanpa guest → "Dear Guest"
- `MarkOpened` saat invitation dengan code dibuka
- OG/Twitter meta: title "The Wedding of X & Y", description tanggal, image cover (1200×630 via versi resize dari T06), `og:url` kanonik
- Gate status: `draft` → 404 untuk publik, tetapi owner yang login bisa melihat dengan banner "Preview"
- Link Google Maps & "Add to Calendar" (.ics) per event
- Lightbox gallery + fade-in on scroll (vanilla JS/Intersection Observer)
- Cache-Control pendek (mis. 60s) + ETag

## Out of Scope
- Logic RSVP/guestbook/gift (T10, T11), transisi status (T12), verifikasi custom domain (T15).

## Acceptance Criteria
- [ ] Code invalid → halaman 404 bergaya ramah
- [ ] Preview link di WhatsApp menampilkan nama pasangan + foto cover (uji dengan debugger OG)
- [ ] Lighthouse mobile: Performance ≥ 85, Accessibility ≥ 90
- [ ] Tidak ada handler lain yang membaca Host/slug langsung (grep check)
- [ ] Total JS halaman public < 60 KB gzip

## Catatan untuk Agent
- Jangan cache halaman personal di CDN secara global tanpa `Vary`/key per path; aman karena code ada di path.
