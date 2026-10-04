# UI Dashboard, Auth & Admin (T22)

Landing, halaman auth, dashboard pasangan, dan panel admin memakai **satu set token brand** dan **satu set komponen**. Halaman undangan punya sistem sendiri (tema, lihat [themes.md](themes.md)).

Kode: token & kelas komponen di `src/styles/app.css` · komponen templ di `src/templates/ui` · kerangka di `src/templates/layouts`.

> Status migrasi: PR 1 (fondasi + auth + halaman publik) selesai. Kerangka dashboard & halaman pasangan menyusul di PR 2, admin + penjaga penuh di PR 3. Berkas yang sudah dimigrasikan terdaftar di `migrated` (`src/templates/ui/ui_test.go`).

## Aturan

- Di dashboard/auth/admin **selalu pakai token `lovoria-*`**. `primary`, `surface`, `ink`, `accent`, `deep`, `muted`, `line`, `on-primary` (tanpa awalan) adalah token **tema undangan** — nilainya berubah per wedding dan di luar undangan jatuh ke cadangan dusty rose.
- **Tidak ada kelas palet Tailwind mentah** (`slate-500`, `red-600`, `green-50`, …). Ditegakkan `TestNoRawPaletteInMigratedTemplates`.
- Jangan mengubah `--lv-primary` di `:root` menjadi plum: itu cadangan tema undangan dan pratinjau tema.
- Pengecualian: pratinjau & swatch tema di dashboard tetap memakai warna tema; hijau WhatsApp boleh tetap sebagai warna merek pihak ketiga.
- Status tidak boleh disampaikan hanya lewat warna — selalu sertakan teks atau ikon.
- Kelas tidak boleh dirakit dinamis (`"ui-badge-" + x`): Tailwind dan test tidak melihatnya. Pakai `ui.Badge(variant, …)` / `ui.AlertClass(variant)`.

## Token warna (`@theme` di `app.css`)

| Token | Hex | Pemakaian |
|---|---|---|
| `lovoria-primary` | `#6b4e71` | Tombol utama, tautan, menu aktif |
| `lovoria-primary-hover` | `#5a4160` | Hover/aktif tombol utama |
| `lovoria-primary-soft` | `#f1ebf2` | Latar menu aktif, badge netral, baris terpilih |
| `lovoria-deep` | `#332936` | Judul halaman, panel brand |
| `lovoria-bg` | `#faf7f5` | Latar halaman |
| `lovoria-surface` | `#ffffff` | Kartu, header, input |
| `lovoria-subtle` | `#f3eeea` | Latar sekunder: kepala tabel, placeholder foto, hover |
| `lovoria-text` | `#292529` | Teks |
| `lovoria-muted` | `#6b666b` | Teks sekunder |
| `lovoria-border` | `#e8dfd9` | Garis & tepi kartu |
| `lovoria-control` | `#9a8f96` | Tepi input & tombol sekunder (≥ 3:1 di atas putih) |
| `lovoria-accent` | `#c9a88a` | Ornamen, ikon, garis — **bukan** teks |
| `lovoria-accent-ink` | `#8c6b50` | Teks kecil beraksen |
| `lovoria-success` / `-soft` | `#3f6b55` / `#eaf2ec` | Sukses, "Hadir", terbit |
| `lovoria-warning` / `-soft` | `#8a5a1c` / `#f8efdd` | Peringatan, draf, mode admin lihat-saja |
| `lovoria-danger` / `-soft` | `#a23b45` / `#f9ebec` | Error, hapus, "Tidak hadir" |
| `lovoria-info` / `-soft` | `#4a5f7a` / `#e9eef4` | Informasi netral |

`TestBrandTokenContrast` membaca nilai ini langsung dari `app.css`: teks ≥ 4.5:1 di atas latarnya, putih di atas warna pekat, tepi kontrol ≥ 3:1.

## Huruf

`layouts.BrandFonts(blocking)` memuat Inter (400/500/600) dan Playfair Display (400/500 + miring 400). `blocking=false`: preload yang dijadikan stylesheet oleh `lovoria.js` / `landing.js` (tanpa handler inline, CSP); `blocking=true` untuk halaman `Cacheable` yang tidak memuat JS itu. Body memakai `layouts.BrandBody` (`font-ui`).

**Playfair Display (`font-display`) hanya untuk**: wordmark, judul halaman (`h1`), nama pasangan / judul wedding, dan angka besar di kartu statistik. Label, tabel, form, dan tombol tetap Inter.

## Komponen

Kelas (`@layer components`, awalan `ui-`; `lv-*` milik undangan):

| Kelas | Catatan |
|---|---|
| `ui-btn` + `ui-btn-primary` / `-secondary` / `-ghost` / `-danger`, `ui-btn-sm` | `rounded-lg`, fokus terlihat (outline plum) |
| `ui-label`, `ui-input`, `ui-hint`, `ui-error` | `ui-input[aria-invalid="true"]` bertepi danger |
| `ui-card` | `rounded-2xl`, garis `lovoria-border`, `shadow-soft` |
| `ui-badge` + `-neutral` / `-muted` / `-success` / `-warning` / `-danger` / `-info` | |
| `ui-alert` + `-success` / `-warning` / `-danger` / `-info` | |
| `ui-table` | Kepala tabel `lovoria-subtle` |
| `ui-tabs`, `ui-tab` | Aktif: `aria-current="page"` atau `ui-tab-active` |
| `ui-link` | Tautan plum |

Komponen templ (`src/templates/ui`): `Input`, `TextArea`, `Select`, `ImageUpload`, `Alert`, `Notice`, `PrimaryButton`, `SecondaryButton`, `Card`, `Badge(variant, label)`, `PageHeader(title, subtitle)` (children = aksi), `EmptyState(title, message)` (children = aksi), dan `AlertClass(variant)` untuk fragmen yang merender pesannya sendiri. Varian: `ui.Neutral`, `ui.Muted`, `ui.Success`, `ui.Warning`, `ui.Danger`, `ui.Info`.

## Halaman auth

`auth.authShell(title, benefits)` (login, daftar, lupa & reset password):

- **≥ 1024px**: dua panel — kiri panel brand `lovoria-deep` (logo putih menaut ke `/`, tagline "Your Love. Your Story. Your Forever.", satu kalimat penjelas, ornamen champagne; poin manfaat hanya di halaman daftar), kanan kartu form.
- **< 1024px**: satu kolom — logo + tagline di atas kartu; poin manfaat di bawah kartu (daftar).
- Teks tagline, penjelas, dan poin manfaat ada di konstanta `authTagline`, `authLead`, `authBenefits` (`modules/auth/views.templ`) — hanya fitur yang sudah ada.
- `id` input, teks tombol, dan atribut `hx-*` tidak berubah (dipakai e2e & handler test).

## Halaman publik Lovoria

`layouts.Public` (error, privasi, syarat, undangan tidak ditemukan / arsip ringkas) memakai warna & huruf brand; `layouts.ErrorPage` menampilkan logo + tombol `ui-btn`. Undangan (tema) tidak terpengaruh.
