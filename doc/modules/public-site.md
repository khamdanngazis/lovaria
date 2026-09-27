# Public site — Dokumentasi Teknis

Halaman undangan yang dilihat tamu. Kode: `src/public-site` · render: `theme.Render` (lihat [doc/themes.md](../themes.md)).

## Route

| Route | Keterangan |
|---|---|
| `GET /w/:slug` | Undangan umum ("Kepada Yth. Tamu Undangan"). |
| `GET /i/:code` | Undangan personal untuk satu tamu (kode tidak peka huruf besar/kecil). |
| `GET /w/:slug/events/:id.ics`, `GET /i/:code/events/:id.ics` | File kalender satu acara. |
| `GET /` | Domain utama: landing page. Custom domain (T15): undangan wedding itu. |
| `GET /events/:id.ics` | Kalender di custom domain. |

## Resolusi wedding — `Resolver.ResolveWedding`

Satu-satunya tempat yang membaca Host header / slug / kode undangan (Arsitektur §3 aturan 5), ditegakkan test `TestSingleResolver` (grep seluruh `src/`). Urutan:

1. **Host header** bukan domain utama (`BASE_URL`, localhost) → `DomainLookup.WeddingIDByHost` (stub `NoDomains` sampai T15).
2. **`/i/:code`** → `guest.GetByCode` → wedding tamu itu. Di custom domain, kode wajib milik wedding domain tersebut.
3. **`/w/:slug`** → `wedding.GetWeddingBySlug`.

Hasil (`Resolved{Wedding, Guest, Preview, Origin, Prefix}`) disimpan di context (`publicsite.FromContext`). Kode/slug tidak ditemukan → halaman 404 ramah.

**Gerbang status:** `published`, `wedding_day`, `memory` → publik. `draft`/`archived` → hanya pemilik yang login (banner "Preview", `Cache-Control: no-store`); selain itu 404. Kunjungan lewat kode tamu mencatat `guest.MarkOpened`, kecuali pemilik yang sedang preview.

## Rendering

`Handler.Invitation`: `ViewBuilder.Build` (wedding, pasangan, acara, cerita, galeri, pengaturan tema) → isi link `.ics` per acara & meta OG → `theme.Render`.

- **OG / Twitter**: `og:title` "The Wedding of X & Y", `og:description` tanggal · tempat acara pertama (personal: "Kepada Yth. {tamu} — …"), `og:image` foto sampul tema atau foto utama (URL absolut), `og:url` kanonik (`BASE_URL` + `/i/KODE` atau `/w/slug`). Uji setelah deploy dengan mengirim link ke WhatsApp atau https://www.opengraph.xyz.
- **Cache**: `public, max-age=60` + ETag (If-None-Match → 304). Aman di CDN karena kode tamu ada di path. HTML undangan **tidak memuat token CSRF** (`layouts.Meta.Cacheable`) supaya bisa dibagi antar pengunjung; form publik (RSVP, T10) memakai token sendiri.
- **Privasi**: `noindex` (meta + `X-Robots-Tag`) — halaman berisi nama tamu tidak boleh masuk mesin pencari. Karena itu skor SEO Lighthouse sengaja rendah.
- **Kalender (.ics)**: waktu lokal acara dikonversi ke UTC dengan zona waktu wedding (`time/tzdata` di-embed); tanpa jam selesai → durasi 2 jam. Acara wedding lain → 404.

## JS & performa

Halaman undangan hanya memuat htmx (untuk RSVP) + `static/js/invitation.js` (±1 KB gzip: fade-in saat scroll via IntersectionObserver — dimatikan bila `prefers-reduced-motion`; lightbox galeri dengan tombol, swipe, dan Esc). Alpine & komponen dashboard tidak dimuat. Total JS ≈ 18 KB gzip (batas 60 KB).

Keputusan performa (hasil Lighthouse mobile — keempat tema Performance 100, Accessibility 100):
- Google Fonts dimuat tanpa memblokir render (`preload` + `onload`, fallback `<noscript>`).
- Foto sampul `fetchpriority="low"`: elemen LCP pembuka adalah teks nama, dan foto besar yang diprioritaskan menunda font judul di jaringan lambat.
- Respons teks dikompres gzip (middleware global, `/media/` dilewati).
- Warna bawaan tema memenuhi kontras WCAG AA (≥ 4.5:1).
