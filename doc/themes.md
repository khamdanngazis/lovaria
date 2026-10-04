# Sistem Tema Undangan

Kode: `src/modules/theme` (registry, token, `Render`, pengaturan per wedding, dashboard) · komponen: `src/templates/themes/<id>` · bagian bersama: `src/templates/shared`.

## Aturan

- **Satu-satunya tempat** yang memetakan tema → komponen adalah registry (`src/modules/theme/registry.go`). Tidak boleh ada `switch themeID` / `themeID == "…"` di luar modul theme — ditegakkan oleh test `TestNoThemeLogicOutsideModule`.
- **Satu-satunya pintu masuk** render undangan: `theme.Render(v view.View) templ.Component` (dipakai public site T09 dan preview dashboard).
- Tema hanya **membaca** `view.View` (`src/modules/theme/view`) — tidak memanggil service atau DB.

## Dua lapis kustomisasi

1. **Token visual** (per tema, bisa di-override per wedding): `Primary`, `Surface` (latar kolom), `Ink` (teks), `FontHeading`, `FontBody`, `Background` (warna hex atau URL gambar), `CoverImage`. `Render` menggabungkan default tema + `wedding_theme_settings`, lalu menulis CSS variable:

   ```css
   :root[data-theme="romantic"]{--lv-primary:#b76e79;--lv-surface:#fff6f6;--lv-ink:#4a3b3b;
     --lv-font-heading:"Great Vibes", cursive;--lv-font-body:"Lora", Georgia, serif;--lv-background:#fff6f6;}
   ```

   Tailwind memetakan variable ini ke kelas `text-primary`, `bg-primary`, `bg-surface`, `text-ink`, `font-heading`. Google Fonts hanya memuat **font judul & isi yang dipakai** (`GoogleFontsURL`).
2. **Token desain bawaan tema** (T21, tidak bisa diubah per wedding, tanpa kolom database):

   | Token | CSS variable | Kelas Tailwind | Dipakai untuk |
   |---|---|---|---|
   | `Accent` | `--lv-accent` | `text-accent`, `bg-accent`, `border-accent` | Ornamen, garis, ikon. **Bukan** teks isi atau tombol utama (kontras rendah). |
   | `Deep` | `--lv-deep` | `bg-deep`, `text-deep` | Bagian gelap & penutup; teks di atasnya putih. |
   | `Muted` | `--lv-muted` | `text-muted` | Teks sekunder di atas `Surface`. |
   | `Border` | `--lv-border` | `border-line` | Garis & tepi kartu. |
   | `Radius`, `ButtonRadius` | `--lv-radius`, `--lv-radius-btn` | — | Sudut kartu/foto dan tombol (`"0"`, `"0.25rem"`, `"9999px"`). |
   | — | `--lv-on-primary` | `text-on-primary` | Warna teks di atas `Primary`: putih, atau gelap bila Primary (kustom) terlalu terang (`OnColor`). |

   Nilai token divalidasi sebelum ditulis ke `<style>` (hex / panjang CSS); nilai tak dikenal diganti cadangan.
3. **Kelas komponen** (`src/styles/app.css`) yang gayanya mengikuti token, supaya markup form & kartu di `templates/shared` tetap satu untuk semua tema: `lv-card`, `lv-btn`, `lv-btn-outline`, `lv-btn-sm`, `lv-input`, `lv-photo`.
4. **Struktur layout** (per tema): komponen templ berbeda untuk Hero, Couple, Events, dst., plus **judul bagian** (`Parts.SectionTitle`).

**Kontras** (`TestThemeContrast`): untuk setiap tema bawaan `Ink`, `Muted`, dan `Primary` di atas `Surface`, teks tombol di atas `Primary`, dan putih di atas `Deep` harus ≥ 4.5:1 (`theme.Contrast`).

## Bagian halaman

`Render` menyusun halaman dari registry bagian `theme.Sections` (`sections.go`, T20):

```
Layout( Hero → shared.MemorySection (T19) → #undangan
        → [bagian tengah sesuai urutan & visibilitas pasangan]
        → Closing → shared.MusicButton )
```

Bagian tengah bawaan (`SectionIDs`): `couple, countdown, quote, events, story, gallery, rsvp, guestbook, gift`. Pasangan bisa mengurutkan semuanya dan menyembunyikan yang `Hideable` (hitung mundur, kutipan, cerita, galeri, ucapan, hadiah). Mempelai, acara, dan RSVP selalu tampil selama status mengizinkan.
- `OrderedSections(settings)`: ID tak dikenal & duplikat dibuang; bagian yang tidak disebut disisipkan setelah tetangga bawaannya.
- `SectionHidden(settings, id)`: hanya berlaku untuk bagian `Hideable`.
- `MoveSection(settings, id, up)`: dipakai tombol naik/turun tanpa JS.

Tombol "Buka Undangan" menuju anchor `#undangan` (awal bagian tengah, apa pun urutannya) atau `#memories` setelah hari H. Teks sapaan & penutup tema lewat `base.GreetingText(v, bawaan)` / `base.ClosingText(v, bawaan, bawaanKenangan)`, jadi isian pasangan berlaku di semua tema.

`theme.Parts` berisi `Layout, Hero, Couple, LoveStory, Events, Gallery, Closing, SectionTitle`. Bagian yang **tidak diisi** tema otomatis memakai tema `base` (`src/templates/themes/base`), jadi tema baru cukup meng-override bagian yang berbeda. Bagian dengan data kosong (tanpa acara/cerita/foto) tidak dirender.

**Judul bagian per tema**: `Parts.SectionTitle(title, subtitle)` dipasang ke `view.SectionTitle` oleh `theme.Prepare` (dipanggil `Render`). Semua bagian — termasuk bagian bersama (hitung mundur, RSVP, ucapan, hadiah, kenangan) — memanggil `base.Title(v, …)`, jadi mengikuti tema tanpa mengenal ID tema. Public site memanggil `theme.Prepare` juga untuk fragmen htmx RSVP/ucapan supaya judulnya tetap bergaya tema setelah di-swap.

| Tema | Override | Karakter |
|---|---|---|
| `signature` (**bawaan**, T21) | Hero, Couple, LoveStory, Events, Gallery, Closing, SectionTitle | Editorial brand Lovoria: Dusty Plum + Champagne + Warm Ivory, Playfair Display + Inter, penutup & galeri Deep Plum, ornamen bintang |
| `elegant` | semua + SectionTitle | Klasik formal: bingkai ganda dengan ornamen sudut, serba rata tengah, foto & kartu acara berpigura |
| `minimal` (ditulis ulang T21) | Hero, Couple, LoveStory, Events, Gallery, Closing, SectionTitle | Putih lega, rata kiri, garis rambut, nama & angka tanggal besar, acara tanpa kartu, galeri grid rapat tanpa sudut membulat, penutup gelap. Sampul selalu muat satu layar (foto mengisi sisa tinggi). |
| `romantic` | semua + SectionTitle | Lembut: foto melengkung (arch), ornamen bunga garis, judul tulisan tangan, cerita berselang-seling |
| `modern` | semua + SectionTitle | Tegas: pembuka berwarna, blok warna penuh (hijau toska + hijau tua), huruf kapital sangat besar, kartu acara berwarna |

Tema bawaan wedding baru: `signature` (`theme.DefaultID`, `wedding.DefaultThemeID`, default kolom `weddings.theme_id` — migration `00023`). Wedding lama tetap memakai `theme_id`-nya. Kelima tema meng-override semua bagian visual (`TestAllThemesOverrideVisualParts`); `base` tetap ada sebagai cadangan untuk tema baru.

## Sampul & "Buka Undangan" (T21)

- Sampul (`#opening`) layar penuh: monogram inisial (`base.Monogram`, `base.Initials` — aman untuk nama satu kata & huruf non-ASCII), nama, tanggal, sapaan + nama tamu, tombol pembuka (`base.OpenButton`).
- Tombol adalah tautan anchor biasa dengan `data-open="lock"`. `invitation.js` menambah kelas `lv-locked` ke `<html>` saat muat; CSS lalu menyembunyikan semua isi setelah sampul (`html.lv-locked #opening ~ *`). Scroll halaman **tidak** dimatikan: sampul yang lebih tinggi dari layar (nama panjang, layar pendek) tetap bisa digulir sampai tombolnya — jangan mengunci dengan `overflow: hidden` (pernah membuat tombol tak terjangkau di layar 360×640). Menekan tombol: isi sampul memudar ke atas (≤ 700 ms), kunci dilepas, halaman menggulir ke target, fokus pindah ke sana, dan musik mulai (T20).
- Tidak dikunci bila: tanpa JS (kunci tidak pernah dipasang), sudah dibuka di sesi ini (`sessionStorage` `lv-opened:<path>`), URL membawa hash (mis. `#rsvp`), atau preview dashboard (`base.OpenLock` mengosongkan `data-open`). Status terbuka hanya di sisi klien — halaman tetap bisa di-cache.
- Mode Kenangan/Arsip: tombol "Lihat Kenangan" menuju `#memories`.
- Foto sampul: `base.CoverImage` (`fetchpriority="high"`, `srcset` thumbnail + ukuran penuh bila sampul berasal dari galeri — `View.CoverThumb`/`CoverWidth`, zoom halus `lv-kenburns`). Sampul tanpa foto tetap utuh (bingkai/monogram di atas `Deep`).

## Gerak

CSS + `IntersectionObserver`: sampul terbuka, reveal bertingkat saat scroll (`.lv-reveal > *`: judul dulu, lalu isi), zoom halus foto sampul. Semua di dalam `@media (prefers-reduced-motion: no-preference)`; dengan reduced-motion isi langsung tampil dan transisi sampul dilewati.

## Tampilan desktop (≥ 1024px)

`base.Layout` membuat dua panel: `base.SidePanel` (kiri, `sticky`, tinggi layar: foto sampul, monogram, nama, tanggal; `aria-hidden` karena isinya mengulang sampul; fotonya `loading="lazy"` sehingga tidak diunduh di ponsel) dan kolom undangan yang menggulir di kanan. Di kolom kanan foto sampul disembunyikan (`lg:hidden`) dan sampul memakai versi tanpa foto. Di bawah 1024px tetap satu kolom.

## Menambah tema baru

1. Buat folder `src/templates/themes/<id>/` berisi `<id>.templ` dengan komponen yang ingin di-override, mis.:

   ```templ
   package garden

   import (
       "github.com/khamdanngazis/lovaria/src/modules/theme/view"
       "github.com/khamdanngazis/lovaria/src/templates/themes/base"
   )

   templ Hero(v view.View) { … pakai v.Couple, v.DateText, base.GuestName(v), base.CoverOf(v) … }
   ```

2. Tambah **satu entri** di `init()` `src/modules/theme/registry.go`:

   ```go
   register(ThemeDef{
       ID: "garden", Name: "Taman", Description: "…",
       Tokens: view.Tokens{
           Primary: "#4f7a4a", Surface: "#f7faf5", Ink: "#223322", Accent: "#c9a88a", Deep: "#1f2b1f", Muted: "#5d6b5d", Border: "#dfe6dc",
           FontHeading: "Lora", FontBody: "Nunito",
       },
       Parts:  Parts{Hero: garden.Hero},
   })
   ```

3. `make generate`, lalu cek tampilan di `/dashboard/weddings/:id/theme/preview?theme_id=garden` pada lebar 375px dan 1280px. Test `TestRenderAllThemes` otomatis mencakup tema baru.

Tidak perlu mengubah routing, database, atau deploy. Font baru harus ditambahkan dulu ke whitelist `Fonts` (`tokens.go`).

## Pengaturan per wedding

Tabel `wedding_theme_settings` (`wedding_id` PK): `primary_color`, `font_heading`, `font_body`, `background_value`, `cover_image_url`. `NULL` = pakai default tema. Pilihan tema sendiri disimpan di `weddings.theme_id` lewat `wedding.Service.SetThemeID` (modul theme yang memvalidasi ID terhadap registry).

Validasi (`ValidateSettings`):
- Warna: hex `#rrggbb` saja.
- Font: harus ada di whitelist `Fonts`; font tulisan tangan (`Script`) hanya boleh untuk judul.
- Latar: hex atau URL gambar `http(s)://` / `/media/…`; sampul: URL gambar.
- URL tidak boleh berisi `" ' ( ) \ < >` atau spasi — nilai token disisipkan ke `<style>` apa adanya, jadi validasi ini yang mencegah CSS/HTML injection.

### Personalisasi (T20)

Kolom tambahan (migration `00022`): `music_url`, `music_enabled`, `music_upload_key`, `music_upload_bytes`, `quote_text` (≤ 500), `quote_source` (≤ 100), `greeting_text` (≤ 100), `closing_text` (≤ 500), `hidden_sections text[]`, `section_order text[]`. Validasi teks & bagian ada di `ValidateSettings` (`personalize.go`). Panjang dihitung per karakter, dan teks selalu di-escape templ. URL musik divalidasi di `Service.Save`: harus lagu pustaka **atau** unggahan wedding itu sendiri (`MusicChoices.Allowed`).

View mendapat `Music`, `Quote`, `Greeting`, `Closing` (diisi `theme.Render` dari Settings) dan `Countdown` (diisi `publicsite.ViewBuilder`):
- **Hitung mundur** (`shared.CountdownSection`): target = mulai acara paling awal (tanggal + jam, zona waktu wedding); tanpa acara → tanggal pernikahan 00.00.
- Sisa hari dihitung per hari kalender di zona waktu wedding. Hari H → "Hari ini!"; setelah hari acara lewat, dan saat Kenangan/Arsip, bagian ini tidak tampil.
- Tanpa JS tampil "N hari lagi"; `invitation.js` menggantinya dengan hari/jam/menit/detik. Tombol "Simpan ke Kalender" menaut ke `.ics` acara pertama.
- **Kutipan** (`shared.QuoteSection`): tidak tampil bila kosong. Contoh siap pakai `QuoteSamples` (Islam, Kristen, umum) hanya mengisi form, tidak pernah dipakai otomatis. Tambahkan contoh lain di `personalize.go`.
- **Musik** (`shared.MusicButton`): `<audio preload="none" loop>` + tombol melayang, tanpa autoplay. `invitation.js` memutar musik saat tamu menekan tombol pembuka dan menyimpan pilihan jeda di `sessionStorage`.

**Pustaka musik bawaan**: manifest `src/modules/theme/music_library.json`. Setiap lagu wajib berisi `id`, `title`, `artist`, `duration`, `file`, `license`, `source`; manifest yang tidak lengkap membuat proses panic saat start. Berkasnya diunggah operator ke bucket foto R2 dengan key `music/<file>`, sehingga URL-nya menjadi `R2_PUBLIC_URL/music/<file>`. Hanya lagu bebas royalti yang lisensinya mengizinkan pemakaian komersial dan dicatat di manifest.

**Unggahan musik**: `POST /theme/music` (multipart `file`, body ≤ 9 MB) → `gallery.UploadAudio`:
- Hanya MP3 (magic bytes ID3 / frame sync MPEG), maks. 8 MB.
- Memesan kuota storage yang sama dengan foto; disimpan di `weddings/<id>/music/<uuid>.mp3`.
- Satu unggahan per wedding: unggahan baru langsung dipilih & diaktifkan, berkas lama dihapus dan kuotanya dikembalikan.
- `DELETE /theme/music` menghapus unggahan dan mengembalikan kuotanya; bila unggahan itu sedang dipakai, pilihan musik ikut dikosongkan.

Setiap perubahan (simpan, unggah, hapus) memanggil `Service.OnChange` → `ViewBuilder.Invalidate`, sehingga halaman publik langsung berubah.

## Dashboard

`/dashboard/weddings/:id/theme`: kartu pilihan tema (contoh warna & font judul), pengaturan warna/font/latar/sampul, dan **preview live** di iframe (`/theme/preview?theme_id=…&primary_color=…`). Preview memakai data wedding sendiri; bagian yang masih kosong diisi data contoh (`publicsite.FillSample`, gambar `static/img/sample-*.svg`). Override di query yang tidak valid diabaikan per field (preview tidak rusak saat user mengetik). Simpan = `PATCH /theme`.

Kartu T20:
- **Teks pembuka & penutup**.
- **Kutipan / ayat** — pilih contoh (Alpine `quotePicker`) lalu edit.
- **Musik latar** — aktif/nonaktif, pilih lagu pustaka / unggahan dengan pratinjau `<audio>`.
- **Unggah musik sendiri** — form multipart terpisah.
- **Susunan bagian** — centang tampil, tombol ↑/↓. Alpine `sectionOrder` memindahkan baris di DOM sehingga urutan input `section_order` ikut berubah. Tanpa JS, tombol mengirim `move=<id>:up|down` dan server menggeser lalu menyimpan.

Form memiliki tombol submit tersembunyi di awal supaya Enter di kolom teks menyimpan tema, bukan memicu tombol naik/turun.

## Catatan templ

- Ekspresi di dalam `<style>` tidak dieksekusi templ — blok CSS token dirender lewat `@templ.Raw("<style>…</style>")` (aman karena nilainya tervalidasi).
- Atribut `style={ … }` di-escape templ; jangan memakai tanda kutip di dalamnya (kutip ter-escape ganda sehingga deklarasi CSS dibuang browser). Nama font dengan spasi valid tanpa kutip.
