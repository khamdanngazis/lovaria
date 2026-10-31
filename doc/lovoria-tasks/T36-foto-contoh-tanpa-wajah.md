# T36 — Foto Contoh Tanpa Wajah

**Estimasi:** 1–1,5 hari · **Depends on:** T33 (branch `t33-signature-color-theme`) · **Modul:** `cmd/server` (demo), `static/img/landing`, `static/img/themes`, `src/public-site` (landing), `src/templates/themes/*`

## Konteks

Undangan contoh dan landing page memakai foto pasangan asli dari Unsplash. Lisensi Unsplash mengizinkan pemakaian komersial, tetapi tidak menjamin ada izin dari orang yang difoto. Foto-foto ini dipakai untuk memasarkan Lunovia dan ditampilkan dengan nama pasangan fiktif, sehingga berisiko terkena Pasal 12 UU 28/2014 (potret untuk reklame komersial butuh persetujuan tertulis orang yang dipotret).

**Keputusan pemilik produk:** semua materi pemasaran dan contoh hanya memakai foto tanpa wajah yang bisa dikenali.

**Tujuan:** setelah task ini, tidak ada wajah yang bisa dikenali di undangan contoh, etalase tema, maupun landing page, termasuk file lama di storage publik.

## Aturan foto

Foto boleh dipakai bila **semua** syarat ini terpenuhi:

- Tidak ada wajah yang bisa dikenali, termasuk dari samping, dari pantulan, atau wajah tamu di latar belakang.
- Tidak ada ciri yang menunjuk orang tertentu: nama di busana, tato khas, atau plat nomor.
- Tidak ada logo, merek, atau karya pihak lain yang menonjol.
- Berlisensi Unsplash License biasa, **bukan Unsplash+**. Halaman sumbernya sudah dibuka dan dicek.
- Nama fotografer dan tautannya dicatat di `CREDITS.md`.

Sosok jauh atau dari belakang boleh dipakai selama wajahnya tidak bisa dikenali, misalnya `IfjHaIoAoqE`.

## Inventaris

### Lolos, bisa dipakai lagi (11 foto)

Sudah diperiksa secara visual, tetapi tetap perlu dikonfirmasi pada ukuran penuh saat pengerjaan.

| Berkas | Isi | Cocok untuk |
|---|---|---|
| `IfjHaIoAoqE.jpg` | Pasangan berjalan di bukit, dari jauh | Sampul, cerita |
| `5BB_atDT4oA.jpg` | Pasangan memegang buket, wajah terpotong | Sampul gelap, galeri |
| `duNBz_LBaEc.jpg` | Pasangan duduk, wajah terpotong | Sampul, cerita |
| `RHfYB_USSEc.jpg` | Tangan memasang cincin | Cerita (lamaran), galeri |
| `ZYet8yoepik.jpg` | Cincin di atas kain | Galeri |
| `M2T1j-6Fn8w.jpg` | Cincin di atas buket | Galeri, cerita |
| `UQl_-yabQiA.jpg` | Buket dipegang pengantin | Galeri |
| `Ko6XKSsO4w8.jpg` | Rangkaian bunga | Galeri |
| `B_R3rmJPeSE.jpg` | Kue pengantin dan dekorasi | Galeri |
| `4FGhD_iGuqg.jpg` | Dekorasi resepsi | Galeri |
| `qG2yK_iNspE.jpg` | Pelaminan dan lorong | Galeri |

### Harus dihapus

**Foto pasangan dengan wajah (`cmd/server/demomedia/`):**
`0cMP8HuJ6zk`, `1cyUquII5Zg`, `4LRIIA-24SI`, `4rtHl_X5GNo`, `BFqxGLypaRo`, `Lr834aonJ70`, `UjLfNJa-KY8`, `VPFnktO9TA8`, `bwo_dSnXuvU`, `l0iyOIhrZVw`, `l4EofTUIANg`, `qVtjqnWYka8`, `rMO7SR9Z6RQ`

**Potret mempelai:** semua `<tema>-groom.jpg` dan `<tema>-bride.jpg` (signature, elegant, romantic, modern, minimal).

**Landing page (`static/img/landing/`):** `story.webp` dan `g4.webp`.

**Thumbnail etalase (`static/img/themes/*.webp`):** semua tema dibuat ulang, karena sebagian besar memuat wajah dari foto sampul.

### Tidak perlu diubah

- `static/img/landing/`: `bouquet`, `venue`, `g1`, `g2`, `g3`, `hero-*`, dan `qr-sample.svg`.
- `static/img/brand/`: `og-lovoria.png` dan ikon (hanya logo).
- Aset floral Signature (bukan foto orang).

## Pekerjaan

### 1. Cari foto baru (10–15 foto)

11 foto yang lolos tidak cukup untuk 5 set foto, karena tiap set butuh 1 sampul + 6 galeri + 3 cerita, dan contoh tema jadi terlihat sama. Yang paling dibutuhkan:

- Detail busana adat Indonesia: songket, batik, kebaya, melati, siger, tanpa wajah. Ini juga menggantikan peran foto adat di tema daerah.
- Tangan bergandengan, tangan berinai, atau tanda tangan buku nikah.
- Pasangan dari belakang atau siluet, untuk sampul.
- Foto bernuansa terang untuk Signature dan Elegan, serta nuansa gelap untuk Modern.
- Minimal 2–3 foto potret, supaya galeri Signature punya variasi rasio.

Unduh, kecilkan ke lebar 1200px (seperti file yang sudah ada), simpan di `cmd/server/demomedia/`, lalu catat di `CREDITS.md`.

### 2. Set foto contoh (`cmd/server/demo_media.go`)

- Susun ulang `demoMedia`, dengan foto yang cocok untuk karakter tiap tema. Bab cerita tetap bermakna: bertemu, memulai perjalanan, lamaran (cincin).
- Hapus caption yang tidak relevan lagi, seperti "Busana adat Karo" dan "Busana adat Sunda".
- Tinjau `demoMediaAlias`: tema daerah sebaiknya memakai foto detail adat bila tersedia.
- Hapus komentar "potret mempelai" yang sudah tidak berlaku.

### 3. Mempelai tanpa foto

- `ensureDemoMedia` berhenti mengunggah potret, dan foto mempelai contoh dikosongkan.
- Tiap tema sudah menampilkan nama tanpa foto (`TestThemesWithMinimalAndFullData`), tetapi tampilannya harus diperiksa di semua tema dengan data contoh yang lengkap.
- Nama, deskripsi orang tua, dan cerita fiktif tetap dipakai.

### 4. Seed yang benar-benar mengganti foto lama (`cmd/server/demo.go`)

`demo seed --refresh` saat ini **tidak cukup**:

- Hanya foto galeri yang dihapus.
- Potret mempelai dan foto cerita lama tetap di storage dan URL-nya tetap bisa dibuka publik.

Perubahan yang diperlukan untuk `--refresh`:

- Hapus objek storage potret mempelai dan foto cerita lama lewat `a.store.Delete`, lalu kosongkan URL-nya. Kunci objek diturunkan dari URL lama, atau dari awalan `weddings/<id>/couple/` dan `weddings/<id>/stories/` bila `storage.Store` mendukung daftar objek.
- Foto cerita diganti, bukan hanya diisi bila kosong (`ensureDemoStoryPhotos`).
- Hanya berjalan untuk wedding `IsDemo`, supaya foto pelanggan tidak mungkin terhapus.
- Seed ulang tetap idempoten.

Potret versi lama yang sudah pernah tertimpa seed sebelumnya tidak punya URL lagi di database. Bersihkan sekali di R2 secara manual lewat awalan `weddings/<id demo>/couple/` (lihat Rilis).

### 5. Landing page

- Ganti `story.webp` dan `g4.webp` dengan potongan foto tanpa wajah. Pakai ukuran dan rasio yang sama, dengan warna dihangatkan seperti file lain.
- Pastikan tidak ada teks atau `alt` yang masih menyebut foto lama (`src/public-site/landing.templ`).
- Kartu tema memakai `lt.Photo` dari sampul contoh, jadi otomatis ikut berganti setelah seed.

### 6. Thumbnail etalase

Jalankan `make theme-thumbs` untuk semua 10 tema setelah set foto baru siap. Periksa tiap hasilnya, termasuk batas 60 KB per file.

### 7. Dokumentasi

- `cmd/server/demomedia/CREDITS.md`: hapus foto yang dibuang, tambah foto baru, perbarui bagian landing page, dan catat aset floral Signature sebagai buatan AI (Gemini).
- `doc/runbook.md`: tambahkan aturan foto dan langkah `--refresh`.
- `doc/themes.md`: catat bahwa materi contoh hanya memakai foto tanpa wajah.
- `CHANGELOG.md`: tambahkan blok di `[Unreleased]`.

## Tes

- `cmd/server/demo_test.go` (`TestSeedDemo`):
  - Sesuaikan jumlah foto, dan pastikan foto mempelai contoh kosong.
  - `--refresh` menghapus objek potret dan cerita lama dari storage.
  - `--refresh` tidak menyentuh wedding bukan demo.
- Tes baru untuk `demomedia`:
  - Tidak ada file `*-groom.jpg` atau `*-bride.jpg`.
  - Semua file yang dirujuk `demoMedia` ada di `demoMediaFS`.
  - Tidak ada file yatim yang tidak dirujuk.
  - Semua file tercatat di `CREDITS.md`.
- Tes tidak bisa mendeteksi wajah, jadi pemeriksaan wajah tetap manual (lihat Kriteria Selesai).
- `make test-integration`, `make lint`, dan `make e2e`.

## Rilis

1. Merge T33, lalu T36.
2. Tunggu deploy Railway selesai.
3. Jalankan seed:
   ```bash
   railway ssh --service lovaria -- lovoria demo seed --refresh
   ```
4. Bersihkan sisa potret lama di R2 pada awalan `weddings/<id demo>/couple/`. **Hanya untuk wedding demo.** Bersihkan juga cache CDN bila ada.
5. Buka setiap halaman di Kriteria Selesai, termasuk versi ponsel dan desktop.
6. Pastikan URL lama sudah 404: catat dulu beberapa URL potret dan foto cerita lama sebelum menjalankan langkah 3.

## Kriteria Selesai

- [ ] Tidak ada wajah yang bisa dikenali di `/w/contoh-<tema>` untuk **10 tema**, dari sampul sampai penutup.
- [ ] Tidak ada wajah di `/`, `/tema`, dan `/tema/<id>` untuk 10 tema.
- [ ] Semua thumbnail etalase sudah dibuat ulang dan bebas wajah.
- [ ] Bagian mempelai tetap rapi tanpa foto di 10 tema.
- [ ] Sampul terbaca di ponsel, tablet, dan desktop, termasuk panel kiri Signature.
- [ ] Galeri Signature tetap bervariasi rasionya.
- [ ] Objek potret dan foto cerita lama sudah tidak bisa dibuka lewat URL publik.
- [ ] Undangan pelanggan tidak berubah.
- [ ] Semua foto tercatat di `CREDITS.md`, dan halaman sumbernya sudah dicek.

## Di Luar Cakupan

- **Riwayat Git:** file lama tetap ada di riwayat repo. Tidak perlu menulis ulang riwayat selama repo privat dan file itu tidak lagi disajikan publik. Cek ulang bila repo akan dibuka.
- **Pratinjau yang sudah tersimpan:** pratinjau link di WhatsApp atau mesin pencari bisa memakai gambar lama sampai cache mereka diperbarui.
- **Foto pelanggan:** tetap tanggung jawab pasangan yang mengunggahnya, sesuai syarat layanan.
- **Testimoni:** tidak memakai foto, jadi tidak berubah.

## Risiko

- **Contoh terasa kurang personal:** foto tanpa wajah bisa terasa lebih dingin. Kompensasinya, pilih foto detail yang kuat, dan pastikan sampul serta foto utama galeri paling menarik.
- **Sampul tema tertentu:** Modern dan Minang memakai foto penuh atau gelap, sehingga butuh foto dengan area tenang untuk teks. Periksa keterbacaan satu per satu, dan sesuaikan pilihan foto per tema bila perlu.
- **Foto Unsplash bisa dihapus atau berubah:** simpan salinan di repo seperti sekarang, dan catat tanggal pemeriksaan di `CREDITS.md`.
