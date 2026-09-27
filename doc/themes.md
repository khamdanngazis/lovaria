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
2. **Struktur layout** (per tema): komponen templ berbeda untuk Hero, Couple, Events, dst.

## Bagian halaman

`Render` menyusun urutan sesuai Produk §7:

```
Layout( Hero → Couple (+Save the Date) → LoveStory → Events → Gallery
        → shared.RSVPSection → shared.GuestbookSection → shared.GiftSection → Closing )
```

`theme.Parts` berisi `Layout, Hero, Couple, LoveStory, Events, Gallery, Closing`. Bagian yang **tidak diisi** tema otomatis memakai tema `base` (`src/templates/themes/base`), jadi tema baru cukup meng-override bagian yang berbeda. Bagian dengan data kosong (tanpa acara/cerita/foto) tidak dirender.

| Tema | Override | Karakter |
|---|---|---|
| `elegant` (default) | Hero, Couple, Events | Emas, serif klasik, bingkai garis & ornamen |
| `minimal` | Hero, Couple, Closing | Putih lega, huruf kapital berjarak, rata kiri |
| `romantic` | Hero, Couple, Closing | Merah muda, judul tulisan tangan, foto melengkung |
| `modern` | Hero, Couple, Events | Sans tebal, blok warna penuh, kartu berwarna |

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
       Tokens: view.Tokens{Primary: "#4f7a4a", Surface: "#f7faf5", Ink: "#223322", FontHeading: "Lora", FontBody: "Nunito"},
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

## Dashboard

`/dashboard/weddings/:id/theme`: kartu pilihan tema (contoh warna & font judul), pengaturan warna/font/latar/sampul, dan **preview live** di iframe (`/theme/preview?theme_id=…&primary_color=…`). Preview memakai data wedding sendiri; bagian yang masih kosong diisi data contoh (`publicsite.FillSample`, gambar `static/img/sample-*.svg`). Override di query yang tidak valid diabaikan per field (preview tidak rusak saat user mengetik). Simpan = `PATCH /theme`.

## Catatan templ

- Ekspresi di dalam `<style>` tidak dieksekusi templ — blok CSS token dirender lewat `@templ.Raw("<style>…</style>")` (aman karena nilainya tervalidasi).
- Atribut `style={ … }` di-escape templ; jangan memakai tanda kutip di dalamnya (kutip ter-escape ganda sehingga deklarasi CSS dibuang browser). Nama font dengan spasi valid tanpa kutip.
