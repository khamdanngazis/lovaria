# T38 — Preview QR Kehadiran di Semua Template

**Estimasi:** 1–1,5 hari · **Depends on:** T31 (QR kehadiran), T33 (Wedding Guest Pass Signature), T37 · **Modul:** `src/templates/shared` (`CheckinSection`), `src/templates/themes/signature`, `src/public-site` (`view.go`, `handler.go`), `static/img`

## Konteks

QR kehadiran (T31) muncul di undangan dalam tiga keadaan. Saat ini hanya Signature yang menangani ketiganya:

| Keadaan | Kapan | Signature | 9 template lain (`shared.CheckinSection`) |
|---|---|---|---|
| QR asli | Link pribadi tamu, atau undangan contoh (tamu contoh) | Kartu Wedding Guest Pass | Kartu QR sederhana |
| Contoh | Preview dashboard (`Checkin.Sample`) | Kartu contoh berisi teks "Contoh — QR pribadi tamu tampil di sini" | **Tidak tampil** |
| Info | Link umum saat check-in dibuka (`Checkin.Available`) | Paragraf cara check-in | **Tidak tampil** |

Akibatnya, pasangan yang memakai 9 template lain tidak melihat bagian QR sama sekali di preview dashboard. Mereka tidak tahu seperti apa QR yang akan diterima tamu. Tamu yang membuka link umum juga tidak mendapat penjelasan cara check-in.

**Tujuan:** di semua template, preview dashboard menampilkan contoh kartu QR yang jelas terlihat sebagai QR. Link umum juga menampilkan info cara check-in.

## Keputusan desain

- **Gambar QR contoh, bukan teks.** Preview menampilkan gambar QR sungguhan bertanda "CONTOH", supaya pasangan melihat wujud akhirnya. Teks pengganti di Signature ikut diganti.
- **QR contoh tidak mengarah ke mana pun.** Isinya teks biasa, misalnya `CONTOH-QR-LUNOVIA`, bukan URL. Bila dipindai di halaman check-in, hasilnya "tidak ditemukan", jadi tidak bisa disalahgunakan atau membingungkan penerima tamu.
  - Gambar dibuat sekali dengan `src/platform/qr`, lalu disimpan sebagai aset statis `static/img/sample-qr.png`, berdampingan dengan `sample-cover.svg` dan aset contoh lain.
  - Jangan memakai `static/img/landing/qr-sample.svg`. Isi QR itu belum diperiksa, dan aset landing sebaiknya tidak dipakai bersama.
- **Preview mengikuti saklar check-in.** Contoh kartu hanya tampil bila pasangan menyalakan check-in (`CheckinEnabled`). Saat ini `FillSample` selalu menyetel `Checkin.Sample`, sehingga Signature menampilkan contoh kartu walaupun check-in dimatikan.
  - Status undangan (draf atau terbit) tidak dipakai, karena preview draf tetap perlu menampilkan bagian ini.
- **Satu kartu bersama untuk 9 template.** `shared.CheckinSection` ditingkatkan dan memakai token tema, sehingga tampil serasi di semua preset T37. Kartu khusus per template berada di luar cakupan. Signature tetap memakai Wedding Guest Pass miliknya.
- **Tampil atau tidaknya diatur saklar check-in.** Bagian `checkin` tidak bisa disembunyikan lewat pengaturan bagian (`Hideable: false` di `sections.go`), dan T38 tidak mengubah hal itu.

## Pekerjaan

### 1. Aset QR contoh (`static/img/sample-qr.png`)

- Buat PNG 512×512 dari `qr.PNG("CONTOH-QR-LUNOVIA", 512)`. Simpan generatornya sebagai tes atau perintah kecil supaya bisa dibuat ulang, dan catat isi QR di komentar.
- Ukurannya kecil (QR satu warna) dan di-commit seperti aset contoh lain.

### 2. Kartu bersama (`src/templates/shared/sections.templ`, `CheckinSection`)

Tiga keadaan, dengan struktur yang sama untuk QR asli dan contoh:

- **QR asli:** sama seperti sekarang, yaitu QR, nama tamu, jumlah orang, "Simpan QR", dan petunjuk singkat. Tambahkan kode undangan, yang dipakai penerima tamu untuk mencari manual bila QR tidak terbaca (seperti `passHolder` Signature).
- **Contoh** (`Checkin.Sample && v.Guest != nil`):
  - Judul "QR Kehadiran" dengan subjudul "Contoh kartu tamu. Tiap tamu mendapat QR pribadi di undangannya."
  - Gambar `sample-qr.png` dengan label "CONTOH" yang menimpa sebagian kecil sudut gambar, tanpa menutupi pola finder QR.
  - Nama tamu contoh ("Nama Tamu") dan jumlah orang.
  - Tombol "Simpan QR" **tidak** tampil.
- **Info** (`Checkin.Available`): paragraf cara check-in, dengan teks yang sama seperti Signature.

Semua warna memakai token (`lv-card`, `text-muted`, `text-primary`). Area QR tetap berlatar putih supaya kontrasnya tinggi di preset gelap.

### 3. Signature (`src/templates/themes/signature/signature.templ`, `Checkin`)

- Keadaan contoh memakai `sample-qr.png` dan label "CONTOH" yang sama, menggantikan teks pengganti.
- Perbarui tes `TestSignatureCheckinStates`: preview kini memuat `sample-qr.png`, tetapi tetap tidak memuat `qr.png` milik tamu.

### 4. Preview mengikuti saklar check-in (`src/public-site/view.go`)

- `ViewBuilder.Preview` hanya menyetel `Checkin.Sample` bila `w.CheckinEnabled`. Pindahkan penyetelan dari `FillSample`, atau beri `FillSample` parameter. Pilih yang paling sedikit mengubah pemanggil lain.
- Jangan ubah perilaku link pribadi, link umum, maupun undangan contoh di `handler.go`.

### 5. Dokumentasi

- `doc/themes.md`: catat tiga keadaan QR kehadiran, dan bahwa preview mengikuti saklar check-in.
- `CHANGELOG.md`: tambahkan blok di `[Unreleased]`.

## Tes

- Render semua tema (`theme.All()`) dalam tiga keadaan:
  - QR asli: memuat `src` QR tamu, nama, kode undangan, dan "Simpan QR".
  - Contoh: memuat `sample-qr.png` dan "CONTOH", tidak memuat "Simpan QR" maupun `qr.png` tamu.
  - Info: memuat teks cara check-in, tanpa gambar QR apa pun.
- Check-in mati: tidak ada `id="checkin"` di tema mana pun.
- Preview dashboard (`handler_test.go` di `src/modules/theme`):
  - Check-in menyala: contoh kartu tampil.
  - Check-in mati: contoh kartu tidak tampil.
- Aset: `static/img/sample-qr.png` ada, dan isinya `CONTOH-QR-LUNOVIA` (dekode dengan pustaka QR yang sudah dipakai, atau bandingkan dengan hasil `qr.PNG`).
- `make test-integration`, `make lint`, dan `make e2e`.

## Kriteria Selesai

- [ ] Di preview dashboard, kesepuluh template menampilkan contoh kartu QR bergambar QR bertanda "CONTOH", bila check-in menyala.
- [ ] Bila check-in dimatikan, preview tidak menampilkan bagian QR di template mana pun, termasuk Signature.
- [ ] Link umum menampilkan info cara check-in di kesepuluh template.
- [ ] Link pribadi dan undangan contoh tetap menampilkan QR asli yang berfungsi.
- [ ] QR contoh tidak berisi URL, dan dipindai di halaman check-in hasilnya "tidak ditemukan".
- [ ] Kartu terbaca di kelima tema warna setiap template, di ponsel dan desktop.

## Di Luar Cakupan

- **Kartu QR khusus per template** seperti Wedding Guest Pass. Kesembilan template memakai kartu bersama.
- **Etalase `/tema`.** Tombol "Lihat contoh" sudah membuka undangan contoh yang menampilkan QR asli tamu contoh.
- **Halaman check-in penerima tamu** (pemindai).

## Risiko

- **QR contoh disalahartikan sebagai QR asli:** label "CONTOH" dan isi non-URL mencegah tamu memakainya. Pastikan label tetap terbaca di ukuran ponsel.
- **Perubahan tampilan untuk pengguna Signature:** preview Signature dengan check-in mati tidak lagi menampilkan contoh kartu. Ini perbaikan, karena preview sekarang sesuai dengan undangan yang akan dilihat tamu.
