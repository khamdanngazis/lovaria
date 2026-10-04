# T21 — Redesign Template Undangan: Lebih Menjual & Selaras Brand

**Estimasi:** 4–5 hari (bisa dipecah jadi 3 PR, lihat *Urutan Pengerjaan*) · **Depends on:** T08, T09, T18, T20 · **Modul:** `modules/theme` (registry, token), `templates/themes/*`, `templates/shared`, `src/styles/app.css`, `static/js/invitation.js`, `public-site` (landing), `cmd/server` (demo)

## Konteks
Brand guide (`doc/landing-page-guide.md`) meminta Lovoria terasa seperti **premium wedding brand**: romantic, elegant, modern, timeless. Landing page sudah mengikuti arah itu (Dusty Plum, Champagne, Warm Ivory, Playfair Display + Inter). Template undangannya belum:

- **Empat tema terasa satu tema.** Tiap tema hanya meng-override 3 dari 7 bagian (`registry.go`); cerita, galeri, dan seluruh bagian bersama (hitung mundur, kutipan, RSVP, ucapan, hadiah) memakai `base`/`shared` yang sama. Setelah pembuka, perbedaan antar tema tinggal warna dan huruf.
- **Palet tidak terhubung ke brand.** Emas, abu-abu, merah muda, dan hijau toska berdiri sendiri; tidak ada tema yang memakai palet Lovoria. Token hanya `Primary/Surface/Ink`, tanpa aksen, warna gelap, atau garis.
- **Tidak ada momen "buka undangan".** Tombol pembuka hanya tautan anchor ke `#undangan`; ornamen hanya garis + belah ketupat; animasi hanya fade-in.
- **Di desktop** undangan tampil sebagai kolom ponsel di atas latar polos.
- **Kartu tema di landing & dashboard** menampilkan foto + nama pasangan, bukan tampilan tema itu sendiri, jadi calon pasangan tidak bisa membandingkan tema tanpa membuka demo satu per satu.

Undangan demo adalah alat jual utama (tombol "Lihat contoh" di landing) dan setiap undangan yang dibagikan ke ratusan tamu adalah iklan Lovoria. Task ini membuat tampilan itu layak dijual.

## Tujuan
1. Tiap tema punya karakter yang terlihat **di seluruh halaman**, bukan hanya di pembuka.
2. Ada satu tema unggulan dengan palet brand Lovoria yang menjadi bawaan untuk wedding baru.
3. Kesan pertama (sampul + buka undangan) terasa premium di ponsel, dan halaman tetap bagus di desktop.
4. Landing dan pemilih tema di dashboard memperlihatkan tampilan tema yang sebenarnya.

## Scope (In)

**1. Bahasa desain bersama (fondasi)**
- Token baru per tema: `Accent` (ornamen, garis, ikon), `Deep` (bagian gelap & penutup), `Muted` (teks sekunder), `Border`. Ditulis sebagai `--lv-accent`, `--lv-deep`, `--lv-muted`, `--lv-border` dan dipetakan ke kelas Tailwind (`text-accent`, `bg-deep`, `border-line`, …).
- Skala tipografi dan ritme bagian yang konsisten: eyebrow kapital berjarak → judul → isi; jarak vertikal antar bagian seragam; selang-seling latar `surface` / `primary` tipis / `deep`.
- Set ornamen SVG inline per tema (pembatas, sudut bingkai, monogram inisial pasangan) memakai `currentColor`, tanpa berkas gambar tambahan.
- Monogram inisial (mis. "R & N") dipakai di sampul dan penutup, dihitung dari nama pasangan.

**2. Sampul & pengalaman "Buka Undangan"**
- Sampul layar penuh: monogram, nama pasangan, tanggal, sapaan + nama tamu, tombol "Buka Undangan".
- Dengan JS: isi undangan terkunci di belakang sampul; menekan tombol memicu transisi sampul (geser/pudar ≤ 700 ms), memulai musik (perilaku T20 tetap), lalu menggulir ke `#undangan`.
- Tanpa JS: perilaku sekarang dipertahankan (tautan anchor, isi tidak terkunci).
- Mode Kenangan/Arsip: tombol "Lihat Kenangan" tetap menuju `#memories`.

**3. Lima tema (satu baru, empat didesain ulang)**

| ID | Nama | Karakter | Palet awal (Primary / Surface / Ink / Accent / Deep) | Huruf | Tata letak khas |
|---|---|---|---|---|---|
| `signature` **(baru, bawaan)** | Lovoria Signature | Editorial, hangat, premium — wajah brand | `#6B4E71` / `#FAF7F5` / `#292529` / `#C9A88A` / `#332936` | Playfair Display + Inter | Foto sampul potret besar, judul editorial besar, garis champagne tipis, penutup gelap Deep Plum |
| `elegant` | Elegan | Klasik formal | `#8A6A3C` / `#FBF8F3` / `#2B2B2B` / `#C9A88A` / `#2E2620` | Cormorant Garamond + Lato | Bingkai ganda dengan ornamen sudut, semua rata tengah, kartu acara berbingkai |
| `minimal` | Minimalis | Lega, tenang | `#5F5661` / `#FFFFFF` / `#1F1F1F` / `#B9B2B0` / `#1F1F1F` | Josefin Sans + Inter | Rata kiri, garis rambut, angka tanggal besar, galeri grid rapat tanpa sudut membulat |
| `romantic` | Romantis | Lembut, personal | `#A85A67` / `#FBF3F1` / `#4A3B3B` / `#C9A88A` / `#5A3540` | Great Vibes + Lora | Foto melengkung (arch), ornamen bunga garis, linimasa cerita berselang-seling |
| `modern` | Modern | Tegas, kontras | `#332936` / `#F6F3F1` / `#191519` / `#C9A88A` / `#191519` | Montserrat + Poppins | Blok warna penuh, pembuka gelap, tipografi sangat besar, kartu acara berwarna |

- Setiap tema meng-override **semua** bagian visual: `Hero`, `Couple`, `LoveStory`, `Events`, `Gallery`, `Closing`. `base` tetap ada sebagai cadangan untuk tema masa depan.
- Hex di atas adalah titik awal; nilai akhir harus lolos uji kontras (lihat Acceptance Criteria).
- Pembuka tanpa foto sampul harus tetap indah (ornamen + monogram), bukan versi "rusak" dari pembuka berfoto.

**4. Bagian bersama mengikuti tema**
- Hitung mundur, kutipan, RSVP, ucapan, hadiah, dan kenangan memakai judul bagian, ornamen, dan gaya kartu/tombol/input milik tema — lewat token baru + komponen judul bagian per tema yang didaftarkan di registry. Markup form dan logikanya tidak diduplikasi per tema.

**5. Gerak**
- Sampul terbuka, reveal saat scroll yang bertingkat (judul lalu isi), zoom halus foto sampul. CSS + `IntersectionObserver` yang sudah ada.
- `prefers-reduced-motion`: semua gerak dimatikan, isi langsung tampil.

**6. Tampilan desktop (≥ 1024px)**
- Dua panel: panel kiri menempel (foto sampul, nama pasangan, tanggal), kolom undangan menggulir di kanan. Di bawah 1024px tetap satu kolom seperti sekarang.

**7. Etalase tema**
- Thumbnail tangkapan layar asli per tema (`static/img/themes/<id>.webp`, potret 4:5, ≤ 60 KB) dibuat oleh skrip Playwright dari undangan demo (`make theme-thumbs`), dipakai di kartu tema landing dan pemilih tema dashboard.
- Kartu landing: tema `signature` tampil pertama dengan label "Pilihan Lovoria"; grid menyesuaikan 5 tema.
- Undangan demo untuk `signature` (`demoCouples` + media) lewat `lovoria seed demo`.

## Out of Scope
- Page builder / drag & drop, bagian atau fitur baru (video, peta tersemat, dsb.).
- Island Svelte atau library animasi (aturan wajib #7).
- Tema berbayar / pembatasan tema per paket.
- Override per wedding untuk token baru (Accent/Deep/Muted/Border) — tetap bawaan tema; hanya `Primary`, huruf, latar, sampul yang bisa diubah pasangan seperti sekarang.
- Mengubah landing page di luar bagian Tema.

## Detail Teknis
- **Token** (`view.Tokens`, `tokens.go`): tambah `Accent`, `Deep`, `Muted`, `Border`; `TokensCSS` menulis variabelnya dengan `colorOr`. `src/styles/app.css` (`@theme`) memetakan ke warna Tailwind. **Tanpa migration** — token baru tidak disimpan per wedding.
- **Primary kustom**: pasangan yang mengganti `Primary` tetap berlaku. Teks di atas `bg-primary` dan `bg-deep` harus tetap terbaca — pakai warna teks tetap (putih) hanya bila kontras terpenuhi; validasi `primary_color` menolak warna yang terlalu terang untuk teks putih (atau turunkan `--lv-on-primary` di `TokensCSS`).
- **Registry** (`registry.go`): daftarkan `signature` paling awal; `DefaultID = "signature"`. Tambah bagian judul per tema (mis. `Parts.SectionTitle func(title, subtitle string) templ.Component`, cadangan `base.SectionTitle`) dan sediakan pembantu supaya `shared.*` mengambilnya lewat registry, bukan `switch themeID` (`TestNoThemeLogicOutsideModule`).
- **Template**: `src/templates/themes/signature/` baru; empat folder lain ditulis ulang. Ornamen di `ornaments.templ` per tema. Monogram: pembantu di `base` (`Initials(v)`), aman untuk nama satu kata dan huruf non-ASCII.
- **Sampul** (`invitation.js` + CSS): kelas pengunci ditambahkan oleh JS saat muat (bukan di HTML), sehingga tanpa JS isi tetap bisa digulir. Fokus pindah ke awal isi setelah terbuka; status "sudah dibuka" disimpan di `sessionStorage` agar muat ulang tidak mengunci lagi. Tanpa script/style inline (CSP T17).
- **Desktop**: di `base.Layout` (atau `Layout` per tema) — panel kiri `hidden lg:block sticky`; gambar panel `loading="lazy"` supaya tidak diunduh di ponsel.
- **Huruf**: tambah bobot/italic yang dibutuhkan ke whitelist `Fonts` (mis. Playfair Display 400;500 + italic). Tetap hanya memuat huruf judul & isi yang dipakai.
- **Performa**: foto sampul `fetchpriority="high"` + `srcset`; anggaran tambahan JS ≤ 2 KB gzip, CSS undangan ≤ +8 KB gzip.
- **Thumbnail**: skrip di `e2e/` (viewport 390×844, potong 4:5) menulis ke `static/img/themes/`; berkas di-commit. `landingTheme` dan kartu di `modules/theme/views.templ` membaca `static.URL("img/themes/<id>.webp")` dengan cadangan kartu warna bila berkas tidak ada.
- **Dok**: perbarui `doc/themes.md` (tabel tema, token baru, sampul, panel desktop, cara membuat thumbnail) dan `CHANGELOG.md`.

## Urutan Pengerjaan (saran pemecahan PR)
1. **Fondasi** — token baru, judul bagian per tema, sampul + buka undangan, gerak, panel desktop; diterapkan ke `base` dan tema `signature` + demo.
2. **Empat tema lama** — tulis ulang `elegant`, `minimal`, `romantic`, `modern` di atas fondasi.
3. **Etalase** — skrip thumbnail, kartu landing & dashboard, dokumentasi.

## Acceptance Criteria
- [ ] Lima tema terdaftar; `signature` menjadi bawaan wedding baru; wedding lama tetap memakai `theme_id`-nya dan tidak ada yang gagal render (`TestRenderAllThemes`)
- [ ] Tiap tema meng-override Hero, Couple, LoveStory, Events, Gallery, Closing; tangkapan layar penuh kelima demo terlihat berbeda di setiap bagian, bukan hanya pembuka
- [ ] Bagian bersama (hitung mundur, kutipan, RSVP, ucapan, hadiah, kenangan) memakai judul & gaya tema; tidak ada `switch themeID` di luar `modules/theme`
- [ ] Sampul: dengan JS isi terkunci sampai "Buka Undangan" ditekan, musik mulai (T20), muat ulang tidak mengunci lagi; tanpa JS halaman bisa digulir dan tautan anchor berfungsi; mode Kenangan menuju `#memories`
- [ ] Test kontras otomatis: `Ink` di atas `Surface`, putih di atas `Primary`, dan teks di atas `Deep` ≥ 4.5:1 untuk semua tema bawaan; `Primary` kustom yang terlalu terang ditangani (ditolak atau warna teks menyesuaikan)
- [ ] Semua tema benar pada kondisi data minim: tanpa foto sampul, tanpa foto mempelai, tanpa cerita/galeri, nama panjang (≥ 30 karakter), satu acara dan empat acara
- [ ] 375px: tidak ada scroll horizontal; ≥ 1024px: tata letak dua panel; `prefers-reduced-motion` mematikan semua gerak
- [ ] Lighthouse mobile undangan demo: Performance ≥ 90, Accessibility ≥ 95, tanpa pelanggaran CSP
- [ ] Landing & pemilih tema dashboard memakai thumbnail asli kelima tema; tema yang dinonaktifkan admin tetap tidak tampil
- [ ] Preview dashboard (`/theme/preview`) dan pengaturan T20 (urutan/visibilitas bagian, teks sapaan & penutup, kutipan, musik) tetap berlaku di kelima tema
- [ ] `doc/themes.md` & `CHANGELOG.md` diperbarui; e2e smoke tetap lolos

## Keputusan yang Perlu Dikonfirmasi Pemilik Produk
1. **Tema lama berubah tampilan untuk undangan yang sudah terbit.** ID dipertahankan (tanpa migration), sehingga undangan lama ikut tampil baru. Usulan: terima, umumkan di CHANGELOG. Alternatif: tema lama dibekukan sebagai `*-classic` dan disembunyikan dari pemilih.
2. **`modern` pindah dari hijau toska ke Deep Plum.** Paling selaras brand, tetapi paling terasa bagi pemakai `modern` yang tidak mengganti warna. Alternatif: pertahankan toska `#2F7D6D` sebagai Primary dan hanya memakai Champagne sebagai aksen.
3. **`signature` sebagai bawaan** menggantikan `elegant` untuk wedding baru.

## Catatan untuk Agent
- Jangan menyalin markup form RSVP/ucapan/hadiah ke tiap tema; variasi cukup lewat token dan komponen judul.
- Atribut `style={ … }` di templ tidak boleh berisi tanda kutip, dan ekspresi di dalam `<style>` tidak dieksekusi (lihat *Catatan templ* di `doc/themes.md`).
- Jangan menambah Alpine/htmx ke halaman undangan; jangan autoplay audio.
- Halaman undangan di-cache (`BuildPublic`): status sampul terbuka hanya di sisi klien, jangan dirender server.
- Ornamen harus dekoratif (`aria-hidden`), dan urutan heading (`h1` → `h2` → `h3`) tetap benar di semua tema.
- Uji tiap tema dengan foto asli berorientasi lanskap **dan** potret; jangan mengandalkan foto demo yang sudah rapi.
- Champagne `#C9A88A` hanya untuk aksen/ornamen, bukan warna teks isi atau tombol utama (kontras rendah; aturan brand guide §5).
