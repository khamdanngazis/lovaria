# Changelog

## [Unreleased]

### T21 (bagian 2) — Tema Elegan, Romantis, dan Modern ditulis ulang
- **Elegan**: bingkai ganda dengan ornamen sudut, foto sampul berpigura, kartu acara berbingkai, penutup gelap.
- **Romantis**: foto sampul & mempelai melengkung, ornamen bunga garis, cerita berselang-seling, kartu lembut.
- **Modern**: pembuka gelap dengan nama sangat besar, blok warna penuh, kartu acara berwarna. **Warna bawaan berubah dari hijau toska ke Deep Plum** — undangan Modern yang tidak mengganti warna ikut berubah.
- Ketiganya kini meng-override semua bagian (sampul, mempelai, cerita, acara, galeri, penutup) dan judul bagian bersama; undangan yang sudah terbit dengan tema ini ikut tampil baru.

### T22 (bagian 3) — Panel admin memakai warna brand
- Panel admin (ringkasan, customers, weddings, tema, paket, pembayaran, storage, domain, audit log) kini memakai token `lovoria-*` dan komponen `ui-*`: judul serif, tab `ui-tabs`, tabel `ui-table`.
- Penjaga warna kini memindai semua templ dashboard, auth, admin, dan komponen ui secara otomatis (berkas baru ikut terpindai); hanya tampilan undangan dan situs publik yang dikecualikan.

### T23 (bagian 2) — Registrasi, wizard, landing, dan admin untuk pembayaran
- **Daftar**: kolom "Ulangi password" (dengan validasi langsung saat mengetik).
- **Buat wedding**: alamat undangan (`/w/…`) bisa dipilih sendiri di langkah 2; kosong = otomatis.
- **Custom domain** termasuk dalam harga dan terbuka setelah undangan dibayar.
- **Landing**: bagian Harga menjadi satu harga Rp149.000 sekali bayar dengan daftar fitur, plus FAQ biaya.
- **Admin**: status pembayaran & riwayat order di detail wedding, **Tandai lunas** manual (catatan wajib, tercatat di audit log), dan tab Pembayaran.
- Syarat & Ketentuan: pembayaran sekali per undangan, tanpa langganan.

### Pembayaran dibuka di tab baru
- "Bayar & Publikasikan" dan "Lanjutkan pembayaran" membuka halaman Midtrans di tab baru; tab Lovoria pindah ke halaman status yang memperbarui diri dan berubah menjadi "Pembayaran berhasil" begitu pembayaran diterima — tidak bergantung pada redirect balik dari Midtrans.

### Perbaikan — tombol "Bayar & Publikasikan" tidak membuka halaman pembayaran
- CSP `form-action 'self'` membuat browser memblokir redirect dari form bayar ke halaman Midtrans, sehingga tombol terasa tidak berfungsi (order tetap terbuat di server). Domain halaman bayar Midtrans kini diizinkan di `form-action` (hanya sebagai tujuan navigasi).
- "Lanjutkan pembayaran" menjadi tautan langsung ke halaman bayar.
- Notifikasi uji dari dashboard Midtrans ("Test notification") kini dibalas 200 dan dicatat, bukan 422.

### T23 (bagian 1) — Pembayaran saat publikasi
- **Rp149.000 sekali bayar per wedding**, ditagih hanya saat dipublikasikan. Membuat, mengubah, dan pratinjau tetap gratis.
- Halaman **Publikasikan** baru: harga, yang didapat, tombol "Bayar & Publikasikan"; setelah lunas pasangan menekan Publikasikan. Kartu status beranda menampilkan status pembayaran.
- **Midtrans Snap** (mode redirect) + webhook `/webhooks/midtrans`: tanda tangan diverifikasi, nominal dicocokkan, status lunas dikonfirmasi ulang ke gateway, diproses idempoten, dan setiap webhook dicatat. Redirect browser tidak pernah menandai lunas.
- Pembayaran kedaluwarsa/gagal bisa diulang tanpa membuat wedding baru. Nomor order `LVR-YYYYMMDD-000001`.
- Undangan yang **sudah terbit** dan undangan contoh otomatis dianggap lunas (migration `00024`); draf lama wajib bayar saat terbit.
- Konfigurasi: `PAYMENT_GATEWAY`, `MIDTRANS_SERVER_KEY`, `MIDTRANS_ENV`, `PUBLISH_PRICE_IDR`, `PAYMENT_EXPIRY_HOURS`. **Selama `PAYMENT_GATEWAY` kosong, wedding baru belum bisa dipublikasikan.**
- Gateway simulasi untuk development/test/e2e (ditolak di production).

### Menu akun di header dashboard
- Menu hamburger diganti menu akun baru: nama & email, pintasan Beranda, Wedding saya, Buat wedding baru, Panel admin (admin), Halaman utama Lovoria, dan Keluar — masing-masing dengan ikon. Di layar lebar pemicunya avatar + nama.
- Tetap berfungsi tanpa JS (`<details>`); menutup saat klik di luar atau Escape.

### T22 (bagian 2) — Dashboard pasangan selaras brand
- **Kerangka dashboard**: header, sidebar, bottom bar ponsel, dan banner mode admin memakai warna brand; huruf Inter + Playfair Display termuat di seluruh dashboard.
- **Semua halaman pasangan** (beranda, wizard & daftar wedding, info, mempelai, acara, cerita, tamu, bagikan, RSVP, galeri, tema, ucapan, hadiah, domain) beralih dari abu-abu dingin & dusty rose ke token `lovoria-*` dan kelas `ui-*`: tombol utama Dusty Plum, kartu bergaris hangat, warna status versi teredam.
- Judul halaman dan nama pasangan memakai Playfair Display; pratinjau & swatch tema tetap memakai warna tema masing-masing.
- Penjaga kelas palet mentah kini mencakup 15 berkas dashboard/auth. Panel admin menyusul di bagian 3.

### T22 (bagian 1) — Fondasi UI brand & halaman auth baru
- **Login, daftar, lupa & reset password** memakai tampilan brand: logo Lovoria, Dusty Plum, Playfair Display + Inter; di layar lebar dua panel dengan tagline "Your Love. Your Story. Your Forever." (halaman daftar menampilkan poin manfaat).
- **Token warna dashboard** (`lovoria-*`: primary-hover, primary-soft, surface, subtle, control, success/warning/danger/info) dan **kelas komponen bersama** `ui-*` (tombol, input, kartu, badge, alert, tabel, tab, tautan). Komponen `templates/ui` beralih ke kelas ini; tambahan `ui.Badge`, `ui.PageHeader`, `ui.EmptyState`.
- Halaman error, privasi, syarat, dan "undangan tidak ditemukan" memakai warna & huruf brand.
- Uji kontras otomatis untuk token baru dan penjaga kelas palet mentah untuk berkas yang sudah dimigrasikan. Dokumentasi: `doc/dashboard-ui.md`.
- Perbaikan: error console htmx di halaman daftar (validasi inline mewarisi `hx-disabled-elt` form).
- Kerangka dashboard dan halaman pasangan/admin menyusul di bagian berikutnya.

### T21 (bagian 2a) — Perbaikan sampul terkunci & tema Minimalis baru
- **Perbaikan**: di layar pendek tombol "Buka Undangan" bisa berada di bawah lipatan dan tidak terjangkau karena halaman dikunci. Sekarang yang dikunci adalah isi setelah sampul; sampulnya sendiri tetap bisa digulir. Berlaku untuk semua tema.
- **Tema Minimalis ditulis ulang**: nama & angka tanggal besar rata kiri, garis rambut, acara tanpa kartu, galeri grid rapat, penutup gelap; sampul selalu muat satu layar. Warna utama menjadi `#5f5661`.
- Demo Minimalis memakai foto baru; `lovoria demo seed --refresh` mengganti foto demo yang sudah ada.
- E2E: tamu menekan "Buka Undangan" di layar 360×640 (regresi tombol terpotong).

### T21 (bagian 1) — Fondasi redesign template & tema Lovoria Signature
- **Tema baru Lovoria Signature** (palet brand: Dusty Plum, Champagne, Warm Ivory; Playfair Display + Inter) dengan seluruh bagian didesain khusus. Menjadi **tema bawaan wedding baru** (migration `00023`); wedding lama tetap memakai temanya.
- **Sampul "Buka Undangan"**: isi undangan terkunci di belakang sampul sampai tombol ditekan, dengan transisi halus, lalu musik mulai. Tanpa JS halaman tetap bisa digulir; muat ulang tidak mengunci lagi.
- **Tampilan desktop** dua panel: foto sampul, nama, dan tanggal menempel di kiri; undangan menggulir di kanan.
- **Bagian bersama mengikuti tema**: hitung mundur, kutipan, RSVP, ucapan, hadiah, dan kenangan memakai judul, kartu, tombol, dan input bergaya tema (token baru Accent/Deep/Muted/Border + bentuk sudut).
- **Monogram inisial** pasangan di sampul, penutup, dan panel desktop; reveal bertingkat saat scroll; semua gerak mati dengan `prefers-reduced-motion`.
- Warna teks tombol menyesuaikan otomatis bila warna utama kustom terlalu terang; uji kontras otomatis untuk semua tema.
- Undangan contoh `contoh-signature` (jalankan ulang `lovoria demo seed`).
- Catatan: tema Elegan, Minimalis, Romantis, dan Modern ikut mendapat sampul terkunci, panel desktop, dan gaya bagian bersama yang baru; penulisan ulang penuh keempatnya menyusul di bagian 2.

### Undangan contoh lebih lengkap
- `lovoria demo seed` kini mengisi foto: sampul, potret mempelai, dan 6 foto galeri per tema (foto Unsplash, kredit di `cmd/server/demomedia/CREDITS.md`), plus kutipan contoh. Demo lama tanpa foto dilengkapi saat seed dijalankan ulang.
- Landing: kartu tema menampilkan foto sampul & nama pasangan undangan contoh.
- Foto mempelai kini menerima URL storage lokal `/media/…` (development), sama seperti pengaturan tema.

### T20 — Personalisasi undangan
- **Hitung mundur** menuju acara pertama (hari/jam/menit/detik, zona waktu wedding): "Hari ini!" pada hari H, hilang setelahnya; tanpa JS menampilkan sisa hari. Ditautkan ke "Simpan ke Kalender".
- **Kutipan / ayat** pembuka dengan contoh siap pakai (Islam, Kristen, umum) yang bisa diedit; **teks sapaan** di pembuka dan **kalimat penutup** bisa diubah di keempat tema.
- **Susunan bagian**: sembunyikan hitung mundur/kutipan/cerita/galeri/ucapan/hadiah dan ubah urutan dengan tombol naik/turun. Preview dashboard langsung mengikuti.
- **Musik latar**: pilih dari pustaka bebas royalti (manifest `music_library.json`, berkas di R2 `music/`) atau unggah MP3 sendiri (≤ 8 MB, kuota storage paket). Tidak autoplay: diputar setelah tamu menekan "Buka Undangan", dengan tombol jeda melayang.
- Migration `00022` (kolom personalisasi di `wedding_theme_settings`). Syarat & Ketentuan: tanggung jawab hak cipta musik unggahan.

### T19 — Remember: halaman kenangan, arsip, & keepsake
- **Mode Kenangan**: undangan berganti tata letak — tombol pembuka "Lihat Kenangan", bagian **Kenangan Hari Bahagia** (foto kategori Pernikahan + ucapan favorit) tepat setelah pembuka, acara "Telah dilangsungkan pada …" tanpa tombol kalender. Berlaku di keempat tema.
- **Arsip read-only**: wedding yang diarsipkan tidak lagi hanya satu kalimat — cerita, galeri, dan ucapan tetap bisa dibaca; semua form ditutup (POST → 403), tanpa amplop digital & data RSVP pribadi. Pasangan memilih **visibilitas arsip** Publik (default) / Privat di kartu status (privat: publik melihat halaman ringkas, pemilik yang login melihat arsip lengkap). Migration `00020` (`weddings.archive_visibility`).
- **Ucapan favorit**: tombol ★ di dashboard Ucapan; favorit tampil sebagai "Ucapan Pilihan" di halaman kenangan & arsip. Perubahan langsung terlihat (cache halaman publik dikosongkan). Migration `00021` (`guestbook_entries.is_favorite`).
- **Kenang-kenangan PDF**: `…/keepsake.pdf` berisi sampul (foto utama), cerita cinta, ringkasan kehadiran, dan semua ucapan yang tampil; tersedia sejak Terbit. Emoji dihapus (font inti PDF).
- Beranda dashboard saat Kenangan/Arsip: kartu **Abadikan kenangan** (unggah foto hari bahagia, pilih ucapan favorit, unduh PDF).

### T18 — Landing page & brand
- Landing page baru sesuai `doc/landing-page-guide.md`: hero "Your Love. Your Story. Your Forever." dengan pratinjau undangan, positioning, alur Create → Invite → Experience → Remember, fitur, galeri tema + undangan contoh, harga (paket bertanda tampil), FAQ, CTA, footer. Mobile-first, tanpa Alpine; Lighthouse mobile 100/100/100/100 (lokal).
- Brand: logo monogram LV (dari `doc/vector-logo.svg`) + wordmark LOVORIA, favicon, apple-touch-icon, gambar Open Graph; token Tailwind `lovoria-*`; header dashboard memakai logo baru.
- `lovoria demo seed`: undangan contoh per tema (`/w/contoh-<tema>`), `weddings.is_demo` (migration `00018`) dikecualikan dari laporan admin & scheduler.
- Admin → Paket: "Tampil di landing page" + urutan (migration `00019`).
- `robots.txt` & `sitemap.xml` (custom domain pasangan tidak diindeks).

### T17 — Hardening & launch readiness
- **Perbaikan kritis**: resolver public site bisa membuat proses crash (`concurrent map writes`) saat request paralel pertama setelah start — ditemukan load test, diperbaiki (`sync.Once`) + test `-race`.
- **Performa**: cache data wedding 10 detik untuk halaman undangan publik (dikosongkan saat ucapan baru; preview pemilik tanpa cache), `last_opened_at` paling sering tiap 15 menit, default `DB_MAX_CONNS` 25. Load test H-1 (200 page view + 50 RSVP/detik): p95 halaman 2,91 s → 45 ms, RSVP 1,2 s → 77 ms (lokal). Script `tools/load/h1.js` + `tools/loadseed`.
- **Keamanan**: CSP (undangan tanpa eval; dashboard `'unsafe-eval'` untuk Alpine), HSTS, Permissions-Policy, `no-store` untuk dashboard/admin; semua handler inline dipindah ke `data-confirm` / `data-select-on-focus`; test isolasi tenant otomatis untuk **setiap** route dashboard (68 route).
- **Error**: halaman 404/500 bergaya (JSON bila diminta, toast untuk htmx), detail 5xx tidak bocor; Sentry opsional (`SENTRY_DSN`); log request memuat `wedding_id` & `user_id`.
- **Backup**: `lovoria backup run|list|restore`, backup harian ke bucket R2 privat (`BACKUP_BUCKET`, retensi 14 hari, advisory lock); restore teruji ke database baru (Postgres 16 & 18). Image memuat `postgresql18-client` (= server produksi).
- **E2E**: Playwright smoke test (register → … → ucapan) di job CI `e2e`.
- Halaman Kebijakan Privasi & Syarat Ketentuan (draf); Dockerfile memakai Go 1.26 (sesuai `go.mod`); `doc/runbook.md` dengan checklist launch.

### T07 — Pilih dari kontak di form tamu
- Tambah cepat & form tamu lengkap: tombol **Pilih dari kontak HP** (Contact Picker API, Android) mengisi No. HP dan Nama (bila kosong). Disembunyikan di iPhone & desktop yang tidak mendukung.

### T14 — Bagikan undangan & custom slug
- Halaman **Bagikan**: link undangan umum (custom domain bila aktif) + Salin, Web Share API, WhatsApp; editor template pesan (Indonesia/English, placeholder `{guest_name}` `{couple}` `{date}` `{link}`) dengan preview langsung.
- Daftar tamu: **Kirim WA** (nomor tamu, atau pilih kontak bila tanpa nomor), **Salin link**, **Salin pesan**; penanda `shared_at` + filter "belum dibagikan".
- Ganti alamat undangan (slug): validasi & kata terlarang, slug lama dialihkan 301 selama 90 hari (`slug_redirects`).
- Link tamu, export CSV, dan halaman Bagikan memakai custom domain bila aktif (`wedding.CanonicalOrigin`).
- Migration `00015` (`guests.shared_at`), `00016` (`share_templates`), `00017` (`slug_redirects`).

### Perbaikan — domain Railway lama 404
- Setelah domain sendiri didaftarkan di Railway, `RAILWAY_PUBLIC_DOMAIN` berganti sehingga domain `*.up.railway.app` lama dianggap host asing dan link undangan lama menjadi 404. Config baru `EXTRA_HOSTS` (dipisah koma) untuk tetap mengenali host lama.
- Script Worker di `doc/custom-domain.md` meneruskan trafik domain Lovoria sendiri tanpa diubah (`OWN_ZONE`).

### Perbaikan — beranda akun admin
- Admin tanpa wedding kini diarahkan ke `/admin` setelah login (sebelumnya ke wizard "Buat website pernikahan"); header dashboard menampilkan link **Admin** untuk akun admin.
- Test performa daftar 1.000 wedding memakai waktu terbaik dari 3 percobaan (tidak flaky saat suite paralel).

### T16 — Panel admin
- Halaman `/admin`: ringkasan, Customers (cari, detail, nonaktifkan), Weddings (filter status/tanggal/cari, urut storage, detail, ubah status sebagai admin, buka website), Tema (jumlah pemakai, nonaktifkan untuk pasangan baru), Paket, Storage (top 20), Domain (kuota 100), Audit log.
- Migration `00012` (`users.disabled_at`: login & sesi ditolak), `00013` (`packages`, `wedding_packages`, `admin_audit_logs`), `00014` (`disabled_themes`).
- Paket mengatur kuota storage galeri & lama arsip per wedding (`gallery.SetQuotaSource`, `wedding.SetArchiveDaysSource`).
- "Lihat dashboard" sebagai pasangan: cookie HMAC 1 jam, hanya GET/HEAD, banner + form nonaktif, tercatat di audit.
- Setiap aksi tulis admin tercatat di `admin_audit_logs`; admin hanya menyentuh tabelnya sendiri (test).
- Perbaikan: halaman tema kini menampilkan error pilihan tema; scheduler dimulai setelah wiring service selesai (hindari race).
- Dokumentasi: `doc/modules/admin.md`; `doc/custom-domain.md` memakai fallback origin originless (`AAAA 100::`) untuk setup Worker.

### T15 — Custom domain (Cloudflare for SaaS)
- Modul `domain` (migration `00011`): daftar domain per wedding, validasi & normalisasi, client Cloudflare Custom Hostnames (create/get/delete), status `pending_verification → active / failed (72 jam) / removed`, scheduler 5 menit dengan advisory lock, hapus domain menghapus hostname di Cloudflare dulu.
- Dashboard menu **Domain**: form, instruksi CNAME untuk orang awam (Host `www`/`@`, target, peringatan domain utama), badge status, Cek ulang, Hapus.
- Resolver: lookup Host → wedding dengan cache 60 detik (dikosongkan saat status berubah); host tak dikenal → 404 generik; `CUSTOM_DOMAIN_HOST_HEADER` untuk proxy (Cloudflare Worker di depan Railway); `RAILWAY_PUBLIC_DOMAIN` dikenali sebagai host Lovoria.
- `/w/:slug` → 301 ke custom domain aktif (`/i/:code` tetap); `wedding.Service.CanonicalBaseURL`; beranda dashboard menampilkan link kanonik.
- Peringatan log saat domain aktif ≥ 90 (kuota gratis 100).
- Config baru: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ZONE_ID`, `CUSTOM_DOMAIN_CNAME_TARGET`, `CUSTOM_DOMAIN_HOST_HEADER`. Dokumentasi setup: `doc/custom-domain.md`.

### T13 — Beranda dashboard & ringkasan RSVP
- Beranda wedding: nama pasangan, tanggal + hitung mundur (zona waktu wedding), status & Publikasikan, link undangan umum + Salin, tombol cepat, checklist onboarding 7 langkah dengan progress, ringkasan tamu & RSVP (open rate, RSVP rate), 5 ucapan terbaru, ringkasan galeri & storage.
- Paket `src/dashboard` sebagai agregator murni (tanpa query; `TestNoQueriesInDashboard`), disuntikkan ke modul wedding lewat `wedding.Deps.Home`. Service baru: `guestbook.Recent`, `gallery.Summary`, `theme.Configured`, `Wedding.CountdownText`.
- Navigasi final: sidebar di desktop, bottom nav + menu di ponsel (menggantikan tab horizontal).
- Daftar wedding (> 1 wedding): badge status & hitung mundur.
- Beranda dengan 500 tamu ±20 ms (lokal).

### T11 — Buku ucapan & amplop digital
- Modul `guestbook` (migration `00009`): section Ucapan & Doa di undangan (nama terisi dari tamu, 10 pesan terbaru + "Muat lebih banyak"), terbuka juga saat Kenangan. Proteksi: honeypot (ditolak diam-diam), token HMAC, rate limit per IP, filter kata kasar (`GUESTBOOK_BLOCKED_WORDS`) → disembunyikan otomatis.
- Modul `gift` (migration `00010`): rekening bank, e-wallet, alamat kirim hadiah; CRUD + urutan di dashboard. Section Tanda Kasih dengan tombol Salin (Clipboard API + cadangan iOS Safari) dan toast; disembunyikan bila kosong.
- Dashboard: tab **Ucapan** (tampilkan/sembunyikan/hapus) dan **Hadiah**.
- RSVP: pilihan menyalin pesan ke buku ucapan (default mati, idempoten).
- `server.PublicFormPath` mencakup form buku ucapan; `ui.Field` mendapat `Mandatory` & `List`.

### T10 — RSVP
- Form RSVP di undangan (`shared.RSVPSection`, semua tema): Hadir/Tidak hadir → jumlah orang (1..max_pax) → pesan; bisa diubah sampai hari H, read-only saat Kenangan. `/w/:slug` mengarahkan ke link pribadi.
- `POST /i/:code/rsvp`: htmx fragment atau redirect 303 tanpa JS; `pax > max_pax` ditolak; update idempoten.
- Proteksi form publik tanpa CSRF cookie: token HMAC (kode + hari, `APP_SECRET`) + rate limit per kode tamu; `server.PublicFormPath` melewati CSRF global hanya untuk path ini.
- Halaman undangan tamu `Cache-Control: private, no-cache` (ETag) supaya status RSVP langsung terlihat saat dibuka lagi.
- Dashboard: tab **RSVP** — statistik, respons terbaru, filter status.
- Config baru `APP_SECRET`.

### T12 — Publish & wedding lifecycle
- State machine `wedding.Service.Transition` dengan tabel transisi & aktor (pasangan/sistem/admin), transisi ilegal ditolak dengan pesan jelas; riwayat di `wedding_status_history` (migration `00008`).
- Checklist publikasi (nama pasangan, tanggal, ≥1 acara) via `EventCounter` dari sub-modul event.
- Scheduler in-process (10 menit + saat start) dengan advisory lock Postgres; hari dihitung di zona waktu wedding; langkah tertinggal dikejar sekaligus; `AdvanceNow` setelah publikasi. `LIFECYCLE_ARCHIVE_DAYS` (default 365).
- Guard `IsPublic`, `AllowsRSVP`, `AllowsGuestbook`, `InMemory`, `IsArchived`, `IsDraft`; `TestStatusGuardsOnly` melarang cek string status di luar modul wedding.
- Halaman publik: banner terima kasih saat Kenangan, halaman ringkas saat Diarsipkan; `View.AllowRSVP/AllowGuestbook/Memory` untuk T10/T11.
- Dashboard: kartu Status undangan — checklist, Publikasikan/Tarik publikasi dengan konfirmasi, riwayat status.
- Dokumentasi: `doc/modules/wedding-lifecycle.md`.

### T09 — Public site
- `Resolver.ResolveWedding`: satu-satunya resolusi wedding (Host/custom domain via `DomainLookup` stub → `/i/:code` → `/w/:slug`), ditegakkan `TestSingleResolver`. Gerbang status: draft/archived hanya untuk pemilik (banner Preview), selain itu 404 ramah.
- Halaman undangan `/w/:slug` & `/i/:code` (sapaan personal, `MarkOpened`), `/` di custom domain; meta OG/Twitter (nama pasangan, tanggal, foto sampul absolut, URL kanonik); `noindex`.
- File kalender `.ics` per acara (zona waktu wedding → UTC, `time/tzdata` di-embed).
- `invitation.js`: lightbox galeri (swipe/Esc) + fade-in saat scroll.
- Cache `public, max-age=60` + ETag/304; HTML undangan tanpa token CSRF per pengunjung (`layouts.Meta.Cacheable`); gzip untuk respons teks.
- Performa & aksesibilitas: font non-blocking, tanpa Alpine di halaman undangan, prioritas foto sampul rendah, heading berurutan, label link galeri, favicon, warna tema bawaan lolos kontras AA. Lighthouse mobile keempat tema: Performance 100, Accessibility 100; JS ±18 KB gzip.
- Dashboard: tautan "Lihat undangan" (preview untuk draft) di ringkasan wedding.
- Dokumentasi: `doc/modules/public-site.md`.

### T08 — Theme system & registry
- Modul `theme`: registry (satu-satunya pemetaan tema → komponen), `Render(view)` sebagai satu-satunya pintu masuk render undangan, kontrak DTO `theme/view.View`.
- 4 tema: `elegant` (default), `minimal`, `romantic`, `modern` — masing-masing meng-override Hero/Couple/Events/Closing; bagian lain dari tema `base`. Placeholder `shared.RSVPSection`, `GuestbookSection`, `GiftSection` (tampil hanya di preview, diisi T10/T11).
- Token CSS per tema + override per wedding (warna utama, font judul/isi, latar warna/gambar, foto sampul); whitelist 12 Google Fonts, font tulisan tangan hanya untuk judul; validasi hex & URL aman (anti CSS injection). Google Fonts hanya memuat font yang dipakai.
- Migration `00007_create_wedding_theme_settings.sql`; `wedding.Service.SetThemeID`.
- Dashboard Tema: kartu pilihan tema, pengaturan, preview live di iframe (data wedding + data contoh untuk bagian kosong), `publicsite.ViewBuilder` (dipakai juga T09).
- `layouts.Document` dengan slot `<head>`; komponen `ImageUpload` memicu event `image-change`.
- Test: render keempat tema, escape nama tamu, fallback tema, larangan logic tema di luar modul (`TestNoThemeLogicOutsideModule`), regresi select opsional & style font-family. Dicek visual di 375px & 1280px.
- Dokumentasi: `doc/themes.md` (termasuk cara menambah tema).

### Fix — URL publik foto R2
- Config menolak `R2_PUBLIC_URL` berupa endpoint S3 API (`*.r2.cloudflarestorage.com`), yang membuat foto gagal dibuka browser (`InvalidArgument: Authorization`), dengan pesan yang menunjuk ke URL Public access / custom domain.
- Perintah `lovoria media rebase-urls --from <url-lama> [--to <url-baru>] [--apply]`: memindah basis URL foto yang sudah tersimpan (gallery, foto utama, foto pasangan, foto cerita); simulasi secara default, idempoten. Setiap modul mengubah tabelnya sendiri lewat service (`RebaseMediaURLs`).

### T07c — Halaman import file lebih ramah
- Kotak unggah besar (klik atau seret file) di paling atas; memilih file langsung membuka pratinjau.
- Tombol "Unduh template" (`GET /guests/import/template`, CSV dengan `sep=,` untuk Excel regional Indonesia); parser mendukung baris `sep=`.
- Panduan 3 langkah & contoh dalam bentuk tabel (menggantikan contoh CSV di `<pre>` yang ikut ter-indent oleh `templ fmt`); banner ke "Tempel daftar".

### T07b — UX tamu
- **Tambah cepat**: baris Nama/HP/Grup selalu terbuka; Enter menyimpan, form dikosongkan, grup terakhir diingat, fokus kembali ke Nama (`POST /guests` dengan `quick=1`).
- **Tempel daftar tamu** (`/guests/paste`): tempel dari catatan HP/chat/Excel, parser teks bebas & tabel (`ParseList`), tabel periksa yang bisa diedit (hapus/tambah baris), simpan semua-atau-tidak (`AddMany`).
- **Pilih dari kontak HP** (Contact Picker API, Chrome Android).
- Kartu statistik lebih ringkas di mobile; ikon emoji dihapus (tidak tampil di sebagian HP).
- Dokumentasi teknis modul: `doc/modules/guest.md`; revisi scope di `doc/lovoria-tasks/T07-guest-management.md`.

### T07 — Guest management
- Migration `00006_create_guests.sql`: tabel `guests` (HP ternormalisasi `62…`, grup, `max_pax` 1–20, `invitation_code` unik global + CHECK format, data RSVP, `attendance_status`, `last_opened_at`).
- Modul `guest` (sqlc `guestdb`): tambah/ubah/hapus, hapus massal, daftar dengan pencarian (nama, HP format lokal/internasional, email, kode; wildcard diperlakukan literal), filter status & grup, pagination 25/halaman; `GetByCode` (lintas wedding, untuk T09), `UpdateRSVP` (T10), `Stats` (T13), `MarkOpened`.
- Kode undangan 7 karakter tanpa `0 O 1 I L`, crypto-random, retry bila bentrok.
- Import CSV: pemisah `,` atau `;`, BOM, nama kolom Indonesia (`nama`, `hp`, `grup`, `jumlah`), pratinjau dengan nomor baris error, konfirmasi stateless yang divalidasi ulang di server, insert lewat `COPY` (500 baris ±30 ms). Maks. 2000 baris / 2 MB.
- Export CSV UTF-8 BOM dengan link undangan lengkap; nilai diawali `= + - @` dinetralkan (formula injection).
- Dashboard Tamu (htmx): kartu statistik tamu & pax, pencarian tanpa kehilangan fokus (filter di luar area swap, statistik & grup lewat out-of-band swap), tambah/ubah, hapus satu & massal, import/export. Diuji di Chrome 375px.

### T06 — R2 storage & gallery
- `platform/storage`: interface `Storage { Put, Delete, PublicURL }`, driver R2 (aws-sdk-go-v2, `Cache-Control: immutable`) dan `local` untuk dev (disajikan di `/media/*`, URL relatif). Config menolak `STORAGE_DRIVER=local` di production.
- `platform/imageproc`: MIME dari magic bytes (JPEG/PNG/WebP; HEIC ditolak dengan pesan), maks. 10 MB & 40 MP (dicek sebelum decode), orientasi EXIF diterapkan lalu EXIF/GPS dibuang, resize 2048px + thumbnail 480px (JPEG), resize area-averaging hemat memori, maks. 2 proses bersamaan.
- Migration `00005_create_gallery.sql`: `weddings.storage_used_bytes`, tabel `gallery_items`.
- Modul `gallery`: upload (kompensasi: objek dihapus & kuota dikembalikan bila gagal), hapus (objek R2 ikut terhapus, kuota berkurang), ubah keterangan/kategori, geser urutan, jadikan foto utama; `ListGallery`, `StorageUsage`.
- Kuota per wedding (`STORAGE_QUOTA_MB`, default 500) lewat `wedding.Service.ReserveStorage` (atomik).
- Dashboard Galeri: multi-upload (maks. 2 bersamaan, progress per file), filter kategori, grid, menu aksi; komponen `ui.ImageUpload` dipakai untuk foto utama, foto pasangan, dan foto cerita.
- Batas body global 12 MB (sebelum middleware yang membaca form); upload 11 MB per request.
- `newApp()` + `routes()` dipisah dari `serve` dan diuji end-to-end (`cmd/server/wiring_test.go`).
- `GOMEMLIMIT=256MiB` di image Docker. Diuji: 10 foto 12 MP sekaligus di viewport 375px → selesai ±12 dtk, RAM puncak ±170 MB.

### T05 — Events & love story
- Migration `00004_create_events_love_stories.sql`: kolom `weddings.timezone` (default `Asia/Jakarta`, pilihan WIB/WITA/WIT di form info wedding), tabel `events` (jenis `akad|reception|engagement|other`, tanggal + jam lokal, maps, lat/lng) dan `love_stories` (tanggal boleh hanya tahun / tahun+bulan).
- Sub-modul `wedding/event` & `wedding/story` (sqlc `eventdb`, `storydb`): CRUD, `MoveX` (naik/turun), `SortXByDate`, `ListEvents` / `ListStories` untuk public site (T09). Semua query memfilter `wedding_id`.
- Urutan: item baru disisipkan kronologis tanpa mengubah urutan manual lain; reorder dalam transaksi `FOR UPDATE`, `sort_order` selalu 0..n-1.
- Validasi link Google Maps (maps.google.*, google.*/maps, maps.app.goo.gl, goo.gl/maps) + ambil koordinat dari `!3d!4d`, `@lat,lng`, `q=`, `query=`, `ll=`.
- UI dashboard htmx: tambah & ubah inline, hapus dengan konfirmasi, ▲/▼, "Urutkan per tanggal"; tanpa JS tetap jalan. Tab Acara & Cerita di halaman wedding (tab aktif otomatis terlihat di mobile).
- `wedding.Register` mengembalikan group per wedding untuk sub-modul; `wedding.Shell` diekspor; `ui.Select`; `web.Retarget`.

### T04 — Wedding core & setup wizard
- Migration `00003_create_weddings.sql`: `weddings` (akar tenant; slug citext unik + CHECK `[a-z0-9-]`, status `draft|published|wedding_day|memory|archived`, `theme_id` default `elegant`) dan `couples` (`wedding_id` unik).
- Modul `wedding` (sqlc `weddingdb`): `CreateWedding`, `GetWedding`, `GetWeddingForOwner`, `GetWeddingBySlug`, `GetCouple`, `UpdateWeddingInfo`, `UpdateCouple`, `ListWeddingsByOwner`; interface baca-saja `wedding.Reader` untuk modul lain.
- Slug otomatis dari nama pasangan (`samuel-sarah`, diakritik dibuang), suffix `-2`, `-3`… bila bentrok, retry saat race; `ValidateSlug` + daftar kata terlarang (dipakai T14).
- `RequireWeddingOwner` + `wedding.OwnerGroup`: wedding milik orang lain / ID tidak valid → 404; `wedding_id` tersedia di `web.WeddingID(ctx)`.
- Setup wizard 3 langkah (htmx, stateless, tombol Kembali, tanpa JS tetap jalan); halaman ringkasan, edit info wedding, edit pasangan (URL foto; upload di T06).
- `/dashboard` → wizard (belum punya wedding), langsung ke wedding (1), atau daftar (>1).
- Method override global (`_method`) untuk `PATCH` dari form tanpa JS; komponen form `templates/ui`; `web.FormatDateID`.
- Diuji di browser headless 375px: wizard sampai selesai, halaman edit, tanpa scroll horizontal.

### Fix
- `lovoria create-admin`: prompt password tidak lagi macet lewat `railway ssh` (Enter dikirim sebagai `\r`), input disembunyikan di terminal, dan password bisa diberikan lewat env `LOVORIA_ADMIN_PASSWORD`.

### T03 — Authentication & session
- Migration `00002_create_auth.sql`: `users` (email citext unique, role `couple|admin`, `email_verified_at`), `sessions`, `password_reset_tokens`. Token disimpan sebagai sha256, nilai mentah hanya di cookie/email.
- Modul `auth` (sqlc `authdb`): register (auto login), login (pesan error generik + dummy hash anti-timing), logout, lupa & reset password (token sekali pakai, 1 jam, semua session dihapus setelah reset).
- Password argon2id (PHC, parameter OWASP, rehash otomatis bila parameter berubah).
- Session server-side, cookie `HttpOnly` + `Secure` (production) + `SameSite=Lax`, rolling expiry 30 hari (diperpanjang maks. 1x/hari); cleanup session kedaluwarsa tiap jam.
- Middleware `LoadSession`, `RequireAuth` (redirect ke `/login?next=`, `HX-Redirect` untuk htmx), `RequireRole` (403), `CurrentUser(ctx)`. `/dashboard/*` & `/admin/*` terlindungi.
- CSRF global (Echo: `Sec-Fetch-Site` + fallback token double-submit, header `X-CSRF-Token` / field `_csrf`), gagal → 403.
- Rate limit per IP (in-memory) untuk login, register, lupa/reset password → 429.
- Form login/register/lupa/reset password (templ + htmx, validasi inline saat blur, tetap jalan tanpa JS), dicek di viewport 375px.
- `platform/mail`: interface `Mailer` dengan driver `log` (default), `smtp`, `resend`.
- CLI `lovoria create-admin`; seeder dev `couple@lovoria.test` / `admin@lovoria.test`.
- `BASE_URL` otomatis dari `RAILWAY_PUBLIC_DOMAIN` bila kosong.

### Deploy
- Hapus `railway.toml`: Railway sudah tidak membaca Config as Code. Setelan deploy (start `lovoria serve`, pre-deploy `lovoria migrate up`, healthcheck `/healthz`, `DATABASE_URL`) kini disimpan di service Railway dan didokumentasikan di README.

### T02 — Database foundation & migrations
- `src/platform/db`: pool `pgxpool` (setelan dari env `DB_*`), `WithTx` (commit/rollback/panic, nested → savepoint), `NewID()` UUIDv7, checker DB untuk `/readyz`.
- `/readyz` → 503 bila DB tidak bisa dihubungi; detail error hanya ke log, tidak ke response.
- goose migration di-embed ke binary; subcommand `lovoria migrate up|down|status|version|redo` dan `lovoria seed` (kerangka).
- Migration awal `00001_init.sql`: extension `citext`, `pgcrypto`, fungsi trigger `set_updated_at()`.
- `sqlc.yaml` (satu package per modul, override uuid/timestamptz/citext); `make sqlc`.
- `make lint-tenant` (`tools/linttenant`): query ke tabel ber-`wedding_id` wajib memfilter `wedding_id`; `wedding_id` wajib ter-index.
- Harness integration test `db/dbtest`: database baru per test package, di-drop setelah selesai.
- `docker-compose.yml` Postgres 16 lokal (port 5433); `make db-up`, `make test-integration`.
- Binary berganti nama menjadi `lovoria`; runtime image pindah ke Alpine 3.24 (±20 MB) supaya pre-deploy command Railway punya shell.
- Railway: pre-deploy `lovoria migrate up`. CI: service Postgres, integration test wajib jalan, migration up/down/up lewat binary.
- Konvensi schema: `doc/database.md`.

### T01 — Project bootstrap & deploy pipeline
- Go module `github.com/khamdanngazis/lovaria` (Go 1.25, toolchain go1.25.14), struktur modular monolith sesuai Arsitektur §3.
- Echo v4 server: request ID, recover, request log slog JSON, secure headers, graceful shutdown (SIGINT/SIGTERM).
- `config.Load()` terpusat dari env var + `.env.example`.
- templ layout dasar `Public` & `Dashboard` dengan `<html data-theme>`.
- Tailwind v4.3.3 standalone; htmx 2.0.11 & Alpine.js 3.17.4 di-vendor di `static/js`.
- Aset statis di-embed ke binary, URL ber-hash konten + `Cache-Control: immutable` di production.
- `GET /healthz` & `GET /readyz` (checker DB menyusul di T02).
- Modul referensi `src/modules/example` (pola handler/service/repository/routes).
- Makefile (`dev`, `build`, `test`, `lint`, `migrate-up/down`), air hot reload, Dockerfile multi-stage (distroless, ±9 MB), `railway.toml`, CI GitHub Actions.
