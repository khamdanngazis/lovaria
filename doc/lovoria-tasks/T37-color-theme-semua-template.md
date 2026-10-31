# T37 — Tema Warna untuk Semua Template

**Estimasi:** 2–3 hari · **Depends on:** T33 (preset warna Signature), T36 · **Modul:** `src/modules/theme` (preset, resolve, halaman Tema), `src/public-site` (etalase `/tema`), `src/templates/themes/*`

## Konteks

T33 memberi Lunovia Signature lima tema warna kurasi (`signaturePresets`). Sembilan template lain baru punya satu set warna bawaan, ditambah "Warna utama sendiri" dan "Warna latar sendiri" yang bebas. Warna bebas sering menghasilkan kombinasi yang kurang serasi atau kontrasnya buruk, karena hanya mengganti `Primary` sementara `Accent`, `Deep`, dan `Surface` tetap.

**Tujuan:** setiap template punya **5 tema warna** yang dirancang khusus untuk karakternya. Pasangan bisa mengganti suasana undangan dengan satu klik tanpa merusak desain.

## Aturan tema warna

- Tepat 5 preset per template. Preset **pertama** adalah warna bawaan saat ini, persis sama nilainya, supaya undangan pelanggan yang sudah ada tidak berubah.
- Preset hanya mengganti warna. Layout, font, radius, dan ornamen tetap milik template.
- Preset menjaga identitas template:
  - **Elegan:** tetap klasik dengan aksen logam.
  - **Minimalis:** tetap netral dan tenang.
  - **Romantis:** tetap lembut.
  - **Modern:** tetap berani.
  - **Koleksi Daerah:** tetap terasa daerahnya. Tidak ada warna neon atau pastel yang memudarkan karakter adat.
- Nama preset dalam bahasa Indonesia, plus satu baris karakter (pola `Romantis · Elegan · Lembut` seperti Signature).
- Nama preset daerah boleh evokatif, tetapi **jangan mengklaim makna adat** yang belum diverifikasi, misalnya warna khusus bangsawan.
- Semua preset lolos `TestThemeContrast`, termasuk aturan tambahan di bagian Tes.

## Palet usulan

Nilai di bawah adalah arah desain untuk `Primary`, `Accent`, `Surface`, dan `Deep`. Semua baris sudah dicek: teks tombol putih di atas `Primary`, `Primary` di atas `Surface`, dan putih di atas `Deep` ≥ 4.5:1, serta `Accent` di atas `Deep` ≥ 3:1. Saat pengerjaan, `Secondary`, `Ink`, `Muted`, `Border`, `Paper`, dan `Soft` diturunkan dari warna-warna ini, lalu semua nilai disesuaikan sampai lolos uji kontras. Preset 1 selalu sama dengan token bawaan di `registry.go`.

Signature tidak termasuk karena sudah punya 5 preset dari T33.

### Elegan — klasik formal, aksen logam

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Champagne Gold (bawaan) | Klasik · Hangat · Anggun | `#8a6a3c` | `#c9a88a` | `#fbf8f3` | `#2e2620` |
| 2 | Navy Gold | Formal · Berwibawa · Tegas | `#2c3e5c` | `#c2a36b` | `#f8f6f1` | `#1b2638` |
| 3 | Emerald Classic | Mewah · Tenang · Berkelas | `#2f5d4e` | `#c4a774` | `#f7f7f2` | `#1d3a31` |
| 4 | Burgundy Royal | Megah · Hangat · Dramatis | `#7a2e3a` | `#c9a27a` | `#fbf6f3` | `#45161f` |
| 5 | Silver Monochrome | Modern · Bersih · Elegan | `#4a4a4f` | `#b8b8bd` | `#fafafa` | `#222226` |

### Minimalis — netral, lega, tenang

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Stone Mauve (bawaan) | Tenang · Lembut · Bersih | `#5f5661` | `#b9b2b0` | `#ffffff` | `#1f1f1f` |
| 2 | Charcoal | Tegas · Sederhana · Kontras | `#333333` | `#bdbdbd` | `#ffffff` | `#111111` |
| 3 | Sage Linen | Natural · Segar · Teduh | `#56695a` | `#b7c2b2` | `#fbfbf8` | `#1f2a22` |
| 4 | Sand Clay | Hangat · Membumi · Santai | `#8a5a44` | `#d4bfae` | `#fdfbf8` | `#2b1f19` |
| 5 | Ocean Mist | Sejuk · Jernih · Tenang | `#3f5f73` | `#b3c4cf` | `#fbfcfd` | `#1a2a33` |

### Romantis — lembut, manis, personal

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Rose Blush (bawaan) | Manis · Hangat · Romantis | `#a85a67` | `#c9a88a` | `#fff6f6` | `#5a3540` |
| 2 | Peach Bloom | Ceria · Hangat · Segar | `#a85a42` | `#e4b49b` | `#fff7f2` | `#5b3326` |
| 3 | Lilac Dream | Lembut · Anggun · Melamun | `#7d5a8c` | `#c7b0d1` | `#fbf7fc` | `#46304f` |
| 4 | Sage Romance | Natural · Teduh · Taman | `#5e7a63` | `#c2cfb8` | `#f8faf5` | `#33453a` |
| 5 | Dusty Blue | Tenang · Halus · Klasik | `#4e6a8a` | `#b9c8d8` | `#f6f9fc` | `#2b3d52` |

### Modern — berani, kontemporer

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Teal (bawaan) | Segar · Tegas · Muda | `#2f7d6d` | `#c9a88a` | `#f4f6f5` | `#1d4a43` |
| 2 | Midnight Navy | Tegas · Elegan · Malam | `#2b4c7e` | `#e0b45c` | `#f3f5f8` | `#18294a` |
| 3 | Terracotta Pop | Hangat · Berani · Energik | `#b04a2f` | `#f0c27b` | `#f8f4f1` | `#5c2618` |
| 4 | Plum | Dramatis · Hangat · Unik | `#6b3a6e` | `#e2a6b8` | `#f6f3f6` | `#3a1d3c` |
| 5 | Graphite | Urban · Bersih · Netral | `#3a3f44` | `#c8d0d6` | `#f4f5f6` | `#1c1f22` |

### Javanese Heritage — sogan, keraton, tenang

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Sogan Gading (bawaan) | Klasik · Tenang · Keraton | `#7a4a26` | `#c9a063` | `#faf5ec` | `#3a2718` |
| 2 | Indigo Lurik | Teduh · Bersahaja · Dalam | `#2e4566` | `#c9a063` | `#f7f5ef` | `#1a2840` |
| 3 | Hijau Keraton | Anggun · Tenang · Berwibawa | `#3f5a3a` | `#c9a063` | `#f8f6ee` | `#23331f` |
| 4 | Merah Manggis | Hangat · Megah · Matang | `#7b2f2f` | `#d0a45f` | `#faf4ee` | `#431818` |
| 5 | Ireng Prada | Formal · Mewah · Kontras | `#3a3330` | `#c9a063` | `#f6f2ea` | `#1e1a18` |

### Sundanese Romance — alami, Priangan, lapang

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Sage Priangan (bawaan) | Alami · Ringan · Romantis | `#5d7256` | `#b9a77c` | `#f8f6ef` | `#36452f` |
| 2 | Biru Situ | Sejuk · Tenang · Jernih | `#4a6a7a` | `#b7c6c9` | `#f6f8f7` | `#2a3e48` |
| 3 | Kembang Kopi | Hangat · Bersahaja · Membumi | `#7a5a44` | `#c7ad8a` | `#faf6f0` | `#43311f` |
| 4 | Rose Mojang | Manis · Lembut · Muda | `#9a5f66` | `#ddb9b4` | `#fdf7f5` | `#573338` |
| 5 | Lumut Tua | Teduh · Dalam · Alami | `#3f5e4f` | `#a9b98f` | `#f5f6ef` | `#243a30` |

### Minang Heritage — megah, gonjong, songket

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Marun Emas (bawaan) | Megah · Tegas · Berkarakter | `#7b1e2b` | `#c9a24b` | `#fbf6ee` | `#4a0f1b` |
| 2 | Hitam Songket | Formal · Mewah · Kontras | `#2b2523` | `#c9a24b` | `#f8f4ec` | `#161210` |
| 3 | Merah Marawa | Berani · Meriah · Hangat | `#a3262a` | `#d8b04c` | `#fcf6ee` | `#5a1214` |
| 4 | Hijau Bukit | Teduh · Anggun · Tenang | `#2f5a43` | `#c9a24b` | `#f7f6ee` | `#1a3527` |
| 5 | Biru Samudra | Sejuk · Berwibawa · Dalam | `#24466b` | `#c9a24b` | `#f6f6f2` | `#142a43` |

### Balinese Elegance — terakota, kamboja, artistik

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Terakota Kamboja (bawaan) | Artistik · Hangat · Anggun | `#9a4a2f` | `#b8975a` | `#faf4ea` | `#3d2a1c` |
| 2 | Hitam Batu Candi | Formal · Megah · Kontras | `#2f2a27` | `#b8975a` | `#f6f2eb` | `#181513` |
| 3 | Hijau Subak | Alami · Teduh · Segar | `#4a6a3f` | `#c2a465` | `#f7f6ee` | `#283c22` |
| 4 | Merah Bata | Berani · Hangat · Meriah | `#a3372c` | `#d4a95e` | `#fbf4ee` | `#5a1b15` |
| 5 | Biru Samudra | Sejuk · Tenang · Lapang | `#2f5a6e` | `#c9b07a` | `#f5f7f6` | `#1a3441` |

### Bugis Royal — merah anggur, lipa sabbe, emas

| # | Nama | Karakter | Primary | Accent | Surface | Deep |
|---|---|---|---|---|---|---|
| 1 | Merah Anggur (bawaan) | Megah · Tegas · Mewah | `#6e1630` | `#d1a94a` | `#fbf5ee` | `#3a0d1c` |
| 2 | Hijau Lipa Sabbe | Anggun · Tenang · Berkelas | `#1f5a48` | `#d1a94a` | `#f6f6f0` | `#10352a` |
| 3 | Ungu Senja | Dramatis · Hangat · Unik | `#4f2a5e` | `#d1a94a` | `#f8f4f8` | `#2b1435` |
| 4 | Biru Pinisi | Berwibawa · Sejuk · Dalam | `#1f3f66` | `#d1a94a` | `#f5f6f8` | `#11243e` |
| 5 | Hitam Emas | Formal · Mewah · Kontras | `#2a2224` | `#d1a94a` | `#f7f3ee` | `#151012` |

## Pekerjaan

### 1. Data preset (`src/modules/theme/presets.go`, `registry.go`)

- Tambahkan `elegantPresets`, `minimalPresets`, dan seterusnya untuk 9 template, lalu isi `ThemeDef.Presets`. Untuk tema daerah, tambahkan parameter preset ke `regionalTheme`.
- Token bawaan tiap tema diambil dari preset pertama (`xxxPresets[0].apply(...)`), seperti Signature, supaya nilainya tidak ditulis dua kali.
- Semua field `Preset` wajib terisi.
  - `Secondary`, `Paper`, dan `Soft` belum dipakai template non-Signature. Nilainya tetap diisi, supaya `--lv-secondary` tidak jatuh ke nilai bawaan Signature di `app.css` bila nanti dipakai.
- ID preset unik **dalam satu tema** (kebab-case). Pencarian preset selalu per tema, jadi ID yang sama di dua tema tidak bentrok.
- `ThemeDef.Fixed` tidak berubah. Hanya Signature yang terkunci.

### 2. Urutan penerapan warna (`resolve` di `tokens.go`)

Untuk tema tidak Fixed, warna diterapkan berurutan, dan setiap langkah menimpa langkah sebelumnya:

1. Token bawaan tema
2. Preset terpilih
3. "Warna utama sendiri"
4. "Warna latar sendiri"

`resolve` sudah bekerja dengan urutan ini, jadi cukup ditambahkan tes.

**Keputusan:** warna bebas tetap ada sebagai pengaturan lanjutan, supaya pengaturan pasangan yang sudah tersimpan tetap berlaku. Di UI:
- Letakkan warna bebas di bawah pilihan tema warna.
- Saat warna bebas aktif, tampilkan catatan bahwa warna utama atau latar menimpa tema warna.

### 3. Picker tema warna (`views.templ`, halaman Tema dashboard)

Masalah saat ini:
- `presetChoices` selalu mengambil preset dari **tema pertama yang punya preset**, yaitu Signature.
- `presetCard` hanya tampil untuk tema Fixed (`x-show="fixed"`).

Kedua hal ini harus diubah:

- Render grup preset per tema. Tampilkan hanya grup milik tema yang sedang dipilih di grid tema. Pemilihan tema dilakukan di sisi klien lewat `pick()` Alpine, jadi pilihan warna harus ikut berganti tanpa reload.
- Radio di grup yang tersembunyi diberi `disabled`, supaya hanya `color_preset` milik tema terpilih yang terkirim. `Save` menolak ID yang bukan milik tema (`"color_preset": "Pilih salah satu warna tema"`), jadi ID dari tema lain tidak boleh ikut terkirim.
- Saat pindah tema, pilihan otomatis kembali ke preset pertama tema baru, kecuali ID tersimpan memang milik tema itu.
- Tanpa JavaScript, halaman tetap benar: grup preset yang tampil adalah milik tema tersimpan.
- Swatch memakai `Primary`, `Secondary`, `Accent`, dan `Surface` seperti sekarang.

### 4. Audit template

Preset baru menguji template dengan warna yang belum pernah dipakai. Periksa setiap template dengan **kelima** presetnya:

- `text-accent` muncul di 29 tempat. Accent di atas `Surface` hanya sekitar 1.4–2.5:1, jadi `text-accent` hanya boleh untuk ornamen dan teks dekoratif besar. Teks kecil yang berisi informasi, misalnya label, tanggal, dan keterangan, dipindah ke `text-primary` atau `text-muted`. Jangan menggelapkan `Accent`, karena ornamen akan rusak dan warna bawaan berubah.
- Overlay sampul dengan `rgba` tetap (`base.templ`, `modern.templ`, `regional/heroes.templ`) harus tetap membuat nama terbaca di semua preset.
- Ornamen SVG daerah (kawung, gonjong, sulapa eppa, patra) harus mengikuti token, bukan warna tetap.
- Tidak boleh ada warna hex tetap di template tema. Saat ini sudah bersih, jadi pertahankan.

### 5. Etalase tema (`src/public-site/themes.templ`)

- `/tema/<id>` sudah menampilkan daftar preset bila `len(t.Presets) > 0`. Pastikan tampil rapi untuk semua template.
- Teks kartu untuk tema dengan preset sudah menyebut "N pilihan warna kurasi". Periksa wording-nya untuk tema yang juga punya warna bebas, misalnya "5 pilihan warna kurasi, atau warna sendiri".
- Thumbnail etalase dan undangan contoh tetap memakai preset pertama.

### 6. Dokumentasi

- `doc/themes.md`: bagian `ThemeDef.Presets` tidak lagi khusus Signature. Catat urutan penerapan warna di §2 dan tabel preset per tema.
- `CHANGELOG.md`: tambahkan blok di `[Unreleased]`.

## Tes

- `TestThemeContrast` (`redesign_test.go`) sudah mencakup semua preset. Tambahkan pemeriksaan `Accent` di atas `Deep` ≥ 3:1, yaitu batas WCAG untuk teks besar seperti nama dan ornamen di sampul gelap.
  - Jangan menuntut 4.5:1. Bawaan Modern (4.47) dan Sunda (4.33) tidak memenuhinya.
  - Jangan menuntut `Accent` di atas `Surface` ≥ 3:1. Accent adalah warna dekorasi yang terang, sehingga semua bawaan saat ini hanya sekitar 1.4–2.5:1. Teks kecil berwarna aksen ditangani lewat audit di Pekerjaan §4.
- Tes baru:
  - Setiap tema punya tepat 5 preset, ID unik dalam tema, semua field terisi hex valid.
  - Preset pertama setiap tema sama persis dengan token bawaan sebelum T37. Ini menjamin undangan pelanggan tidak berubah.
  - `resolve` untuk tema tidak Fixed: preset diterapkan, lalu warna bebas menimpanya. Tanpa preset tersimpan (`ColorPreset == ""`), hasilnya sama dengan bawaan.
  - `Save` menolak `color_preset` milik tema lain.
- Perbarui `TestPresetsOnlyChangeColors` dan `TestOtherThemesKeepCustomization` (`presets_test.go`) supaya berlaku untuk semua tema.
- Tes render halaman Tema: grup preset ada untuk setiap tema, dan hanya grup tema tersimpan yang aktif.
- `make test-integration`, `make lint`, dan `make e2e`.

## Kriteria Selesai

- [ ] Kesepuluh template punya 5 tema warna, dan preset pertama sama dengan warna sebelumnya.
- [ ] Undangan pelanggan yang sudah ada tidak berubah tampilannya, termasuk yang memakai warna bebas.
- [ ] Di halaman Tema, pilihan warna mengikuti tema yang dipilih tanpa reload, dan menyimpan tidak pernah gagal karena ID preset tema lain.
- [ ] Setiap preset diperiksa visual di ponsel dan desktop, dari sampul sampai penutup, untuk kesepuluh template.
- [ ] Nama pasangan di sampul gelap terbaca di semua preset.
- [ ] `/tema/<id>` menampilkan 5 pilihan warna untuk setiap template.

## Di Luar Cakupan

- **Font dan layout per preset:** preset hanya mengganti warna.
- **Thumbnail etalase per preset:** etalase tetap memakai preset pertama.
- **Aset gambar berwarna per preset** seperti floral Signature: template lain memakai ornamen berbasis token, jadi tidak perlu aset baru.
- **Menghapus warna bebas:** tetap tersedia untuk tema tidak Fixed (lihat Pekerjaan §2).

## Risiko

- **Teks berwarna aksen:** di latar terang, accent tidak memenuhi kontras teks kecil di preset mana pun, termasuk bawaan. Audit di Pekerjaan §4 wajib dikerjakan, bukan opsional.
- **Preset terasa tidak cocok dengan adat:** palet daerah adalah usulan. Minta pemilik produk meninjau nama dan warna tema daerah sebelum diimplementasikan.
- **Picker berganti di sisi klien:** form bisa mengirim ID preset tema lain bila logika `disabled` salah. Tes `Save` dan E2E berganti tema menjaga hal ini.
