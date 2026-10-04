# T22 — Redesign Dashboard, Login & Register: Selaras Warna Brand

**Estimasi:** 3–4 hari (bisa dipecah jadi 3 PR, lihat *Urutan Pengerjaan*) · **Depends on:** T03, T13, T16, T18, T21 · **Modul:** `src/styles/app.css`, `templates/layouts`, `templates/ui`, `modules/auth`, `dashboard`, semua `modules/*/views.templ` (dashboard & admin)

## Konteks
Landing page (T18) dan undangan (T21) sudah memakai identitas Lunovia: Dusty Plum, Deep Plum, Warm Ivory, Champagne, Playfair Display + Inter (`doc/landing-page-guide.md`). Begitu calon pasangan menekan "Buat undangan", tampilannya berganti menjadi aplikasi lain:

- **Warna utama dashboard bukan warna brand.** Dashboard dan halaman auth memakai `bg-primary` / `text-primary`, yang di luar halaman undangan jatuh ke nilai bawaan `--lv-primary: #b76e79` (dusty rose) di `:root` — bukan Dusty Plum `#6B4E71`. Logo di header berwarna plum, tombol di bawahnya berwarna rose.
- **Netralnya abu-abu dingin.** ±400 kelas `slate-*` tersebar di 18 berkas templ (`bg-slate-50`, `border-slate-300`, `text-slate-500`, …), berlawanan dengan Warm Ivory dan garis hangat `#E8DFD9` milik brand.
- **Warna status mentah.** `red-600`, `green-700`, `amber-500` Tailwind dipakai langsung; brand guide §7 meminta menghindari merah jenuh.
- **Huruf brand tidak dimuat.** Inter dan Playfair Display hanya dimuat di landing; dashboard jatuh ke huruf sistem, dan wordmark `LUNOVIA` (`font-display`) di header jatuh ke Georgia.
- **Login & register terasa generik.** `authShell` menampilkan teks "Lunovia" polos (bukan `BrandLogo`) di atas kartu putih, tanpa tagline maupun kesinambungan dengan landing.
- **Gaya ditulis berulang.** Kelas input disalin di `ui/form.templ` dan `auth/views.templ`; tombol, kartu, badge, tabel, dan tab ditulis ulang di tiap modul, sehingga mengganti warna berarti menyunting ratusan baris.

Task ini murni tampilan: warna, huruf, dan komponen bersama. Tidak ada perubahan alur, route, atau data.

## Tujuan
1. Landing → register → dashboard terasa satu produk: warna, huruf, dan logo yang sama.
2. Warna dashboard berasal dari satu set token brand; tidak ada lagi kelas palet Tailwind mentah di templ dashboard/auth/admin.
3. Komponen UI bersama (tombol, input, kartu, badge, alert, tab, tabel) didefinisikan sekali di `templates/ui`.
4. Login & register memberi kesan pertama yang premium di ponsel dan desktop.

## Scope (In)

**1. Token warna dashboard**
- Dashboard dan auth memakai token `lovoria-*` yang sudah ada di `@theme` (`lovoria-primary`, `-deep`, `-bg`, `-accent`, `-accent-ink`, `-text`, `-muted`, `-border`), **bukan** `primary`/`surface`/`ink` — tiga token itu milik tema undangan dan nilainya berubah per wedding.
- Token tambahan (titik awal; nilai akhir harus lolos uji kontras):

| Token | Hex awal | Pemakaian |
|---|---|---|
| `lovoria-primary-hover` | `#5A4160` | Hover/aktif tombol utama |
| `lovoria-primary-soft` | `#F1EBF2` | Latar menu aktif, badge netral, baris terpilih |
| `lovoria-surface` | `#FFFFFF` | Kartu, header, input |
| `lovoria-subtle` | `#F3EEEA` | Latar sekunder (kepala tabel, placeholder foto, hover baris) |
| `lovoria-success` / `-success-soft` | `#3F6B55` / `#EAF2EC` | Sukses, "Hadir", terbit |
| `lovoria-warning` / `-warning-soft` | `#8A5A1C` / `#F8EFDD` | Peringatan, "Ragu", draf, mode admin lihat-saja |
| `lovoria-danger` / `-danger-soft` | `#A23B45` / `#F9EBEC` | Error, hapus, "Tidak hadir" |
| `lovoria-info` / `-info-soft` | `#4A5F7A` / `#E9EEF4` | Informasi netral |

- Pemetaan netral: `slate-50` → `lovoria-bg`; `slate-100` → `lovoria-subtle`; `slate-200/300` (garis) → `lovoria-border`; `slate-400/500/600` (teks sekunder) → `lovoria-muted`; `slate-700/800/900` (teks) → `lovoria-text`; judul halaman → `lovoria-deep`.
- Champagne `#C9A88A` hanya untuk ornamen, ikon, dan garis; teks kecil beraksen memakai `lovoria-accent-ink`.

**2. Huruf**
- Layout dashboard dan auth memuat Inter (400/500/600) dan Playfair Display (400/500) dengan cara yang sama seperti `landingHead`; body memakai `font-ui`.
- Playfair Display hanya untuk: wordmark, judul halaman (`h1`), nama pasangan/judul wedding, dan angka besar di kartu statistik. Label, tabel, form, tombol tetap Inter.

**3. Komponen bersama (`templates/ui`)**
- Kelas komponen di `src/styles/app.css` (`@layer components`, awalan `ui-` supaya tidak bertabrakan dengan `lv-*` milik undangan): `ui-btn` (+ `-primary`, `-secondary`, `-ghost`, `-danger`, `-sm`), `ui-input`, `ui-label`, `ui-card`, `ui-badge` (+ varian status), `ui-alert` (+ varian), `ui-table`, `ui-tabs`, `ui-link`.
- Komponen templ yang sudah ada (`Input`, `TextArea`, `Select`, `Alert`, `Notice`, `PrimaryButton`, `SecondaryButton`, `Card`, `ImageUpload`) beralih ke kelas itu. Tambahan: `ui.Badge(variant, label)`, `ui.PageHeader(title, subtitle)`, `ui.EmptyState(...)`.
- Bentuk: tombol `rounded-lg` (bukan pil), kartu `rounded-2xl` + garis `lovoria-border` + `shadow-soft`; fokus terlihat (`ring` `lovoria-primary`) di semua kontrol.
- `modules/auth` memakai `ui-input`/`ui-btn` — salinan kelas input dihapus.

**4. Login, register, lupa & reset password**
- `authShell` baru: latar `lovoria-bg`, `BrandLogo` (plum) menaut ke `/`, kartu form, judul halaman Playfair, tautan bantuan warna `lovoria-primary`, footer Privasi/Syarat.
- ≥ 1024px: dua panel — panel kiri brand (latar `lovoria-deep`, logo putih, tagline "Your Love. Your Story. Your Forever.", satu kalimat penjelas, ornamen champagne), panel kanan form. Di bawah 1024px: satu kolom, panel brand diringkas menjadi logo + tagline di atas kartu.
- Register menampilkan 2–3 poin manfaat singkat (hanya fitur yang sudah ada) di panel brand.
- Pesan error, sukses, dan rate limit (`tooManyRequests`) memakai `ui-alert`.

**5. Kerangka dashboard (`layouts.Dashboard`, `wedding.Shell`, `admin.shell`)**
- Header: latar putih, garis `lovoria-border`, `BrandLogo`, nama pengguna `lovoria-muted`, tautan Admin `lovoria-primary`. Menu ponsel (☰) dirapikan dengan komponen yang sama.
- Sidebar & bottom bar wedding: menu aktif `lovoria-primary-soft` + teks `lovoria-primary`; judul grup `lovoria-muted`; bottom bar tetap berfungsi tanpa JS (`<details>`).
- Banner mode admin lihat-saja: `lovoria-warning` (bukan `amber-500`), tetap menempel dan terbaca.
- Tab admin dan tab di modul lain memakai `ui-tabs`.

**6. Halaman dashboard**
Semua berkas berikut diganti ke token + komponen bersama, tanpa mengubah struktur data atau perilaku:

| Berkas | Catatan |
|---|---|
| `src/dashboard/home.templ` | Kartu status, progres onboarding, ringkasan RSVP, ucapan terbaru |
| `src/modules/wedding/views.templ` (+ `event/`, `story/`) | Wizard, daftar wedding, ringkasan, form info & mempelai |
| `src/modules/guest/views.templ`, `share_views.templ`, `rsvp_views.templ` | Tabel tamu, badge RSVP, tombol kirim WA (hijau WhatsApp boleh tetap sebagai warna merek pihak ketiga) |
| `src/modules/gallery/views.templ` | Grid foto, unggah, kuota |
| `src/modules/theme/views.templ` | Pemilih tema, teks, kutipan, musik, susunan bagian |
| `src/modules/guestbook/views.templ`, `gift/views.templ`, `domain/views.templ` | Moderasi ucapan, rekening hadiah, status domain |
| `src/modules/admin/views.templ` | Statistik, tabel, pager, audit, paket |
| `src/templates/layouts/layouts.templ` | `ErrorPage` & footer memakai warna brand |

## Out of Scope
- Perubahan alur, route, validasi, teks/copy (kecuali tagline dan poin manfaat di halaman auth), atau struktur data.
- Mode gelap.
- Halaman undangan dan tema (T21) serta landing page (T18) — hanya dipakai sebagai acuan.
- Template email dan PDF kenang-kenangan.
- Island Svelte, library komponen, atau library ikon baru (aturan wajib #7).

## Detail Teknis
- **Token** (`src/styles/app.css`): tambah token di `@theme`. Jangan mengubah `--lv-primary` di `:root` menjadi plum sebagai jalan pintas — nilai itu cadangan tema undangan (`base`) dan pratinjau tema di dashboard.
- **Pratinjau tema di dashboard** (`modules/theme/views.templ`, `/theme/preview`): swatch dan pratinjau tetap memakai warna tema (`primary`/`surface`/`ink` atau hex tema). Hanya krom di sekelilingnya yang beralih ke `lovoria-*`.
- **Huruf**: pindahkan konstanta URL huruf dari `landing.templ` ke `layouts` (mis. `layouts.BrandFonts()`), dipakai landing, auth, dan dashboard. CSP sudah mengizinkan `fonts.googleapis.com` / `fonts.gstatic.com` (`platform/server/headers.go`).
- **`layouts.Public`** dipakai halaman auth, error, privasi, dan syarat; semuanya berada di domain utama, jadi aman beralih ke warna brand. Pastikan halaman publik yang dirender di custom domain (undangan) tidak ikut terpengaruh.
- **Penjaga regresi**: test Go yang memindai `*.templ` dashboard/auth/admin/`templates/ui` dan gagal bila menemukan kelas palet Tailwind mentah (`slate|gray|zinc|red|green|amber|sky|…-NNN`) atau `bg-primary`/`text-primary`/`border-primary`. Pengecualian eksplisit: `templates/themes/*`, `templates/shared/*`, `public-site/*`, dan blok pratinjau tema.
- **Test kontras**: perluas pola di `modules/theme/redesign_test.go` — pasangan teks/latar token baru ≥ 4.5:1 (teks `lovoria-text` & `-muted` di `-bg`/`-surface`/`-subtle`; putih di `-primary` & `-primary-hover`; tiap warna status di latar `-soft`-nya dan di putih).
- **Tanpa migration**, tanpa perubahan handler. `id` elemen form dan teks tombol dipertahankan (dipakai `e2e/tests/smoke.spec.js` dan handler test).
- **Dok**: tambah `doc/dashboard-ui.md` (token, komponen `ui-*`, kapan memakai Playfair, aturan warna status) dan perbarui `CHANGELOG.md`.

## Urutan Pengerjaan (saran pemecahan PR)
1. **Fondasi + auth** — token, huruf, kelas `ui-*`, komponen `templates/ui`, `authShell` baru (login, register, lupa/reset password), `ErrorPage`.
2. **Kerangka + halaman pasangan** — `layouts.Dashboard`, `wedding.Shell`, beranda, wedding, acara, cerita, tamu, RSVP, galeri, tema, ucapan, hadiah, domain.
3. **Admin + penjaga** — `admin/views.templ`, test pemindai kelas mentah, test kontras, dokumentasi.

## Acceptance Criteria
- [ ] Tombol utama, tautan, dan menu aktif di dashboard & auth berwarna Dusty Plum `#6B4E71`; tidak ada lagi dusty rose `#b76e79` di luar pratinjau tema
- [ ] Tidak ada kelas palet Tailwind mentah maupun `*-primary` tema di templ dashboard/auth/admin/`templates/ui` (test pemindai lolos; pengecualian terdaftar eksplisit)
- [ ] Inter dan Playfair Display termuat di dashboard & auth; wordmark `LUNOVIA` tampil dengan Playfair, bukan Georgia
- [ ] Login/register: dua panel di ≥ 1024px, satu kolom di 375px tanpa scroll horizontal; logo menaut ke `/`; validasi inline htmx, pesan error, dan rate limit tetap berfungsi
- [ ] Input, tombol, kartu, badge, alert, tab, dan tabel berasal dari `templates/ui` / kelas `ui-*`; tidak ada salinan kelas input di `modules/auth`
- [ ] Test kontras otomatis lolos untuk semua pasangan token baru (≥ 4.5:1); fokus keyboard terlihat di semua kontrol
- [ ] Pratinjau dan swatch tema di dashboard tetap menampilkan warna tema masing-masing
- [ ] Mode admin lihat-saja: banner terbaca, kontrol tampak nonaktif (`lv-readonly`) seperti sekarang
- [ ] Bottom bar dan menu "Menu" di ponsel tetap berfungsi tanpa JS
- [ ] Lighthouse mobile halaman login dan beranda dashboard: Accessibility ≥ 95, tanpa pelanggaran CSP / error console
- [ ] `go test ./...` dan e2e smoke lolos tanpa mengubah selector test; `doc/dashboard-ui.md` & `CHANGELOG.md` diperbarui

## Keputusan yang Perlu Dikonfirmasi Pemilik Produk
1. **Warna status** (sukses/peringatan/bahaya/info) di tabel di atas adalah usulan versi hangat & teredam. Alternatif: tetap hijau/merah Tailwind hanya untuk status, demi keterbacaan yang sudah dikenal.
2. **Panel brand di halaman auth**: ornamen + tagline saja (usulan, tanpa aset baru) atau memakai foto pasangan (perlu foto berlisensi di `static/img`).
3. **Tombol dashboard `rounded-lg`**, berbeda dari tombol pil di landing. Usulan: tetap `rounded-lg` karena lebih padat untuk tabel dan form; alternatif: pil di semua tempat.

## Catatan untuk Agent
- Ini task tampilan. Jangan mengubah nama field, `id`, atribut `hx-*`, `x-data`, atau urutan elemen form — handler test dan e2e bergantung padanya.
- Jangan mengganti kelas secara buta dengan cari-ganti: `text-slate-500` bisa berarti teks sekunder, placeholder, atau ikon nonaktif; pilih token menurut perannya.
- `primary`, `surface`, `ink`, `accent`, `deep`, `muted`, `line` adalah token **tema undangan**. Di dashboard selalu pakai `lovoria-*`.
- Kelas yang dirakit dinamis (`"bg-" + x`) tidak terdeteksi Tailwind; tulis nama kelas lengkap atau pakai `templ.KV`.
- Tanpa script/style inline baru (CSP T17). Atribut `style={ … }` yang sudah ada untuk lebar progres boleh dipertahankan.
- Setiap warna status harus disertai teks atau ikon — jangan menyampaikan status hanya lewat warna.
- Cek tiap halaman di 375px dan 1280px, termasuk kondisi kosong (belum ada tamu/foto/ucapan) dan kondisi error form.
