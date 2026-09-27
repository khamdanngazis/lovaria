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

**Gerbang status** (guard `wedding.IsPublic()`, lihat [wedding-lifecycle.md](wedding-lifecycle.md)): `draft` → hanya pemilik yang login (banner "Preview", `Cache-Control: no-store`), selain itu 404. `memory` → banner terima kasih; `archived` → halaman ringkas. Kunjungan lewat kode tamu mencatat `guest.MarkOpened`, kecuali pemilik yang sedang preview.

## Rendering

`Handler.Invitation`: `ViewBuilder.Build` (wedding, pasangan, acara, cerita, galeri, pengaturan tema) → isi link `.ics` per acara & meta OG → `theme.Render`.

- **OG / Twitter**: `og:title` "The Wedding of X & Y", `og:description` tanggal · tempat acara pertama (personal: "Kepada Yth. {tamu} — …"), `og:image` foto sampul tema atau foto utama (URL absolut), `og:url` kanonik (`BASE_URL` + `/i/KODE` atau `/w/slug`). Uji setelah deploy dengan mengirim link ke WhatsApp atau https://www.opengraph.xyz.
- **Cache**: `/w/:slug` → `public, max-age=60`; `/i/:code` → `private, no-cache` (selalu validasi ulang, supaya status RSVP tamu langsung terlihat); keduanya dengan ETag (If-None-Match → 304). HTML undangan **tidak memuat token CSRF** (`layouts.Meta.Cacheable`) supaya bisa dibagi antar pengunjung; form publik (RSVP, T10) memakai token sendiri.
- **Privasi**: `noindex` (meta + `X-Robots-Tag`) — halaman berisi nama tamu tidak boleh masuk mesin pencari. Karena itu skor SEO Lighthouse sengaja rendah.
- **Kalender (.ics)**: waktu lokal acara dikonversi ke UTC dengan zona waktu wedding (`time/tzdata` di-embed); tanpa jam selesai → durasi 2 jam. Acara wedding lain → 404.

## RSVP (T10)

Bagian `shared.RSVPSection` (sama di semua tema), tampil bila `View.AllowRSVP` (guard `wedding.AllowsRSVP()`: published & wedding_day):

| Akses | Tampilan |
|---|---|
| `/i/:code` | Form: Hadir / Tidak hadir → jumlah orang (1..`max_pax`, disembunyikan lewat CSS `:has()` saat Tidak hadir) → pesan opsional. Jawaban tersimpan terisi ulang dan bisa diubah sampai hari H. |
| `/w/:slug` | "Gunakan link undangan pribadi Anda untuk RSVP." |
| Status Kenangan | Ringkasan jawaban tamu (read-only), tanpa form. |
| Preview dashboard | Form contoh nonaktif. |

**Endpoint** `POST /i/:code/rsvp` — payload `status` (`attending`/`declined`), `pax`, `message`, `token`. Lewat resolver (tamu & wedding), lalu `guest.UpdateRSVP` (satu `UPDATE` per tamu → kiriman ganda idempoten, jawaban terakhir menang; `pax > max_pax` → 422).
- htmx (`HX-Request`) → fragment `<section id="rsvp">` (hx-swap outerHTML) dengan pesan sukses / error.
- Tanpa JS → `303` ke `/i/:code?rsvp=ok#rsvp`, halaman menampilkan pesan sukses.
- Wedding bukan published/wedding_day → 403 "konfirmasi kehadiran sudah ditutup"; draft/kode tidak ada → 404.

**Proteksi tanpa CSRF cookie** (HTML undangan di-cache): path ini dilewati middleware CSRF global (`server.PublicFormPath`) dan diganti:
- **Token HMAC** di hidden field: `<hari>.<HMAC-SHA256(APP_SECRET, "rsvp|KODE|hari")>`. Hari (UTC, dibulatkan) membuat HTML & ETag stabil sepanjang hari; token berlaku 30 hari dan hanya untuk kode tamu itu. Token salah → 403 "kirim ulang".
- **Rate limit per kode tamu** (in-memory): 6/menit, burst 5 → 429 (pesan dirender di section).
- `APP_SECRET` kosong → kunci acak per proses (token dari halaman lama tidak berlaku setelah restart; production mencatat peringatan).

Pilihan "Tampilkan juga pesan ini di buku ucapan" (checkbox, default mati, hanya bila buku ucapan terbuka): pesan RSVP disalin lewat `guestbook.Post` dengan nama tamu — hanya bila pesannya baru/berubah, jadi kiriman ganda tidak membuat entri ganda.

## Buku ucapan & amplop digital (T11)

Modul & dashboard: [guestbook-gift.md](guestbook-gift.md).

**Buku ucapan** (`shared.GuestbookSection`, tampil bila `wedding.AllowsGuestbook()`: published, wedding_day, **memory**): form nama (terisi nama tamu bila lewat `/i/:code`) + ucapan, lalu 10 pesan terbaru dan "Muat lebih banyak".

| Route | Fungsi |
|---|---|
| `POST /i/:code/guestbook`, `/w/:slug/guestbook`, `/guestbook` (custom domain) | Kirim ucapan. htmx → section baru (pesan sukses + daftar terbaru); tanpa JS → 303 ke `…?guestbook=ok#guestbook`. Validasi → 422; status tertutup → 403; draft/preview → 404. |
| `GET` path yang sama `?before=<id>` | Potongan pesan berikutnya (htmx menukar tombol "Muat lebih banyak"; tanpa JS halaman sederhana). |

Proteksi (tanpa CSRF cookie, lihat `server.PublicFormPath`):
- **Honeypot** `website` (disembunyikan dari manusia & pembaca layar, `.lv-hp`): terisi → dibalas seolah sukses, **tidak disimpan**.
- **Token HMAC** `guestbook|<wedding id>|<hari>` (sama seperti RSVP).
- **Rate limit per IP** (`c.RealIP()`, in-memory): 5/menit, burst 5 → 429 dengan pesan di section.
- Filter kata kasar → pesan disembunyikan otomatis.
- Semua teks dirender lewat templ (ter-escape) — pesan berisi HTML tampil apa adanya sebagai teks.

**Amplop digital** (`shared.GiftSection`): tidak dirender bila wedding tidak punya akun. Kartu per akun (penyedia, nomor, a.n.) dan kartu alamat. Tombol **Salin Nomor / Salin Alamat** diaktifkan `invitation.js` (tanpa Alpine — halaman undangan sengaja tidak memuat Alpine demi performa): `navigator.clipboard.writeText` di konteks HTTPS, cadangan `execCommand('copy')` dengan textarea `contentEditable` + `setSelectionRange` (iOS Safari), toast "Tersalin". Nomor disalin tanpa spasi/strip. Tanpa JS tombol tetap tersembunyi dan nomor bisa diseleksi (`select-all`). Nomor rekening **hanya** ada di section ini — tidak di OG meta, halaman arsip, atau file kalender.

## JS & performa

Halaman undangan hanya memuat htmx (untuk RSVP) + `static/js/invitation.js` (±1 KB gzip: fade-in saat scroll via IntersectionObserver — dimatikan bila `prefers-reduced-motion`; lightbox galeri dengan tombol, swipe, dan Esc). Alpine & komponen dashboard tidak dimuat. Total JS ≈ 18 KB gzip (batas 60 KB).

Keputusan performa (hasil Lighthouse mobile — keempat tema Performance 100, Accessibility 100):
- Google Fonts dimuat tanpa memblokir render (`preload` + `onload`, fallback `<noscript>`).
- Foto sampul `fetchpriority="low"`: elemen LCP pembuka adalah teks nama, dan foto besar yang diprioritaskan menunda font judul di jaringan lambat.
- Respons teks dikompres gzip (middleware global, `/media/` dilewati).
- Warna bawaan tema memenuhi kontras WCAG AA (≥ 4.5:1).
