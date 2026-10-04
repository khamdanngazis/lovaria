# T18 — Landing Page & Brand

**Estimasi:** 2 hari · **Depends on:** T08, T09, T16, T17 · **Modul:** `public-site` (landing), `templates/layouts`, `static/img`

## Konteks
Beranda `lovoria.my.id` sekarang hanya dua baris ("Lunovia — Undangan pernikahan digital yang personal"). Calon pasangan tidak melihat apa itu Lunovia, kenapa berbeda, atau bagaimana memulai. Landing page adalah tempat positioning di `doc/brand-positioning.md` disampaikan: *"Lunovia is more than a wedding invitation. It is a digital home for a couple's love story and wedding memories."*

## Scope (In)
- **Hero**: nama Lunovia, tagline utama **"Your Love. Your Story. Your Forever."**, satu kalimat penjelas Bahasa Indonesia, tombol utama **"Buat undangan"** (→ `/register`) dan sekunder **"Lihat contoh"** (→ undangan demo).
- **Alur produk** 4 langkah sesuai Product Framework: **Create → Invite → Experience → Remember**, masing-masing 1 judul + 1 kalimat + ikon/ilustrasi ringan.
- **Fitur inti** (kartu): cerita cinta, acara & peta, galeri, RSVP, ucapan & doa, amplop digital, link pribadi per tamu + kirim WA, custom domain, status Kenangan setelah hari H. Hanya fitur yang **sudah ada** — jangan menjanjikan fitur Future Experience.
- **Galeri tema**: 4 tema dari theme registry (`theme.All()`, hanya yang aktif — tema nonaktif dari admin T16 tidak ditampilkan) dengan thumbnail + tombol "Lihat contoh" per tema.
- **Undangan demo**: satu wedding contoh publik per tema (data contoh, bukan data pengguna) yang bisa dibuka tanpa login, mis. `/w/contoh-elegant`. Dibuat lewat seeder produksi yang idempoten (`lovoria seed demo`), ditandai `is_demo` supaya tidak muncul di statistik admin & tidak bisa diubah.
- **Paket & harga**: tabel paket dari admin T16 (`packages`, hanya yang ditandai tampil di landing) — nama, kuota foto, lama arsip, harga tampilan. Bila belum ada paket bertanda tampil: bagian ini disembunyikan dan CTA tetap "Buat undangan gratis".
- **FAQ** singkat (5–7 pertanyaan): apakah tamu perlu install aplikasi, bisa pakai domain sendiri, berapa lama undangan aktif, apakah data tamu aman, cara RSVP, dst.
- **Footer**: link Kebijakan Privasi, Syarat & Ketentuan, kontak.
- **SEO & share**: `<title>`, meta description, Open Graph (gambar `static/img/og-lovoria.png` 1200×630) supaya link `lovoria.my.id` tampil bagus di WhatsApp. Landing **boleh diindeks** (beda dengan undangan yang `noindex`); tambah `robots.txt` & `sitemap.xml` (landing, privacy, terms saja).
- Header landing: logo, link "Masuk" (→ `/login`) dan "Buat undangan"; bila sudah login → "Dashboard".

## Out of Scope
- Blog, halaman harga terpisah, multi-bahasa (EN) landing.
- Pembayaran / checkout (paket hanya ditampilkan).
- Animasi berat / island JS (aturan wajib #7).

## Detail Teknis
- Route tetap `GET /` di domain utama lewat `publicsite.Handler.Home` (custom domain tetap menampilkan undangan).
- Template `src/public-site/landing.templ` (dipisah dari `pages.templ`), tanpa Alpine; JS hanya bila perlu (mis. FAQ pakai `<details>`).
- Kolom baru `packages.show_on_landing boolean NOT NULL DEFAULT false` + urutan (`sort_order`) — migration di modul admin; toggle di halaman Paket admin. Landing mengambil paket lewat service admin (method baca baru), bukan query langsung.
- Kolom `weddings.is_demo boolean NOT NULL DEFAULT false` (modul wedding); `AdminList`, `CountByStatus`, `CountByTheme`, statistik ringkasan admin mengecualikan demo. Pemilik demo = akun sistem tanpa password yang bisa dipakai login.
- Thumbnail tema: screenshot statis per tema di `static/img/themes/<id>.webp` (≤ 60 KB), atau render `theme.Render` dengan data contoh `FillSample` bila lebih praktis — jangan panggil Chrome saat runtime.
- Cache: `Cache-Control: public, max-age=300`; CSP publik (`StrictCSP`) tetap berlaku — tidak ada script/handler inline.
- `robots.txt`: `Allow: /`, `Disallow: /dashboard, /admin, /i/, /w/` (undangan pribadi tetap tidak diindeks).

## Acceptance Criteria
- [ ] Landing tampil baik di 375px dan 1280px; Lighthouse mobile Performance ≥ 90, Accessibility ≥ 95, SEO ≥ 90
- [ ] Semua CTA berfungsi: Buat undangan → register, Masuk → login, Lihat contoh → undangan demo tiap tema terbuka tanpa login
- [ ] Hanya tema aktif & paket bertanda tampil yang muncul; tanpa paket, bagian harga hilang
- [ ] Undangan demo tidak terhitung di statistik admin dan tidak bisa diedit pemilik mana pun
- [ ] Preview link `https://lovoria.my.id` di WhatsApp menampilkan judul, deskripsi, dan gambar OG
- [ ] Tidak ada pelanggaran CSP / error console (cek seperti T17)
- [ ] Test: handler landing (tema nonaktif tidak tampil, paket tersembunyi bila kosong), seeder demo idempoten, demo dikecualikan dari statistik

## Catatan untuk Agent
- Teks landing Bahasa Indonesia, tagline boleh tetap Bahasa Inggris (sesuai brand). Jangan menulis "unlimited", "selamanya tersimpan", atau janji teknis lain — lihat catatan "Forever" di brand-positioning §5.
- Gambar/ilustrasi harus milik sendiri atau lisensi bebas; simpan di `static/img`, bukan R2.
- Konten final (kalimat hero, FAQ, harga) dikonfirmasi pemilik produk sebelum merge; siapkan draf yang mudah diubah (konstanta/teks di satu file).
