# Dashboard pasangan (T13)

Paket `src/dashboard` adalah **agregator**: tidak punya tabel maupun query. Semua angka diambil dari service modul masing-masing (`TestNoQueriesInDashboard` menolak import pgx / `platform/db` / paket `…/db`).

## Alur masuk

`GET /dashboard`: belum punya wedding → wizard; satu → beranda wedding itu; lebih dari satu → `/dashboard/weddings` (kartu per wedding: status, tanggal, hitung mundur).

## Beranda wedding — `GET /dashboard/weddings/:id`

Handler milik modul wedding (header + kartu status, supaya `PATCH /status` tetap bisa merender ulang dengan error 422). Bagian lainnya disuntikkan lewat `wedding.Deps.Home` (`wedding.HomeWidgets`) = `dashboard.Home.Widgets`:

| Bagian | Sumber |
|---|---|
| Nama pasangan, tanggal, hitung mundur (`Wedding.CountdownText`: "H-45", "Hari ini", "3 hari lalu" — per hari kalender di zona waktu wedding) | wedding |
| Status + Publikasikan / Tarik publikasi + riwayat | wedding (T12) |
| Link undangan umum (`BASE_URL/w/slug`) + Salin (Alpine `copyText`), tombol Buka website / Kelola wedding / Bagikan undangan (sementara ke menu Tamu, T14 menggantinya) | — |
| Checklist onboarding (7 langkah, progress) — disembunyikan bila semua selesai | lihat di bawah |
| Tamu & RSVP: diundang, hadir (+pax), tidak hadir, belum konfirmasi; **open rate** (`last_opened_at` terisi / total) & **RSVP rate** ((hadir + tidak hadir) / total) | `guest.Stats` |
| 5 ucapan terbaru (termasuk yang disembunyikan, diberi label) | `guestbook.Recent` |
| Jumlah foto, 6 thumbnail, pemakaian storage | `gallery.Summary` |
| **Abadikan kenangan** (Kenangan & Arsip, T19): saran "Unggah foto hari bahagia" (selesai bila ada foto kategori Pernikahan) dan "Pilih ucapan favorit" (selesai bila ada favorit), tombol **Unduh kenang-kenangan (PDF)** | `gallery.ByCategory`, `guestbook.Favorites` |
| Link "Unduh kenang-kenangan (PDF)" di kartu link (Terbit & Hari H) | — |

## Kenang-kenangan PDF (T19) — `GET /dashboard/weddings/:id/keepsake.pdf`

`dashboard.Keepsake` (owner-only lewat grup wedding; `Cache-Control: private, no-store`, `Content-Disposition: attachment; filename="kenangan-<slug>.pdf"`). Dibuat on-demand dengan `github.com/go-pdf/fpdf` (A5), tidak disimpan:

1. Sampul: foto utama wedding (bila ada), "Kenang-kenangan pernikahan", nama pasangan, tanggal.
2. Cerita cinta (bila ada).
3. Kehadiran: jumlah undangan, hadir (+ orang), berhalangan, jumlah ucapan (`guest.Stats`).
4. Ucapan & doa: **semua** ucapan yang tampil (tanpa yang disembunyikan, maks. 5.000), urut terlama dulu, nama + tanggal di zona waktu wedding.

Foto sampul diambil lewat HTTP (`HTTPPhoto`: timeout 5 detik, maks. 8 MB; URL relatif storage lokal dilengkapi `BASE_URL`). Foto gagal diambil atau bukan JPEG → sampul tanpa foto (hasil proses gallery selalu JPEG).

**Font**: memakai font inti PDF (Helvetica / Times) dengan encoding Windows-1252 — cukup untuk Bahasa Indonesia termasuk tanda kutip & aksen Latin. **Emoji dan aksara non-Latin dihapus** (`pdfText`) karena tidak didukung font inti; bila nanti dibutuhkan, tambahkan font TTF (mis. Noto Sans) lewat `AddUTF8Font`. 500 ucapan ≈ 1 detik di test (`TestKeepsakePDF`).

Checklist onboarding:

| Langkah | Selesai bila | Sumber |
|---|---|---|
| Profil pasangan | foto atau deskripsi terisi untuk **kedua** mempelai | `wedding.GetCouple` |
| Acara | ≥ 1 acara | `event.CountEvents` |
| Cerita cinta | ≥ 1 cerita | `story.ListStories` |
| Galeri foto | ≥ 1 foto | `gallery.Summary` |
| Tema | pengaturan tema pernah disimpan | `theme.Configured` |
| Daftar tamu | ≥ 1 tamu | `guest.Stats` |
| Publikasikan | status bukan draft | wedding |

Analytics tanpa tracking pihak ketiga: hanya kolom milik Lunovia (`last_opened_at`, status RSVP).

## Navigasi

`wedding.Shell` (dipakai semua halaman wedding):
- **Desktop (≥ lg):** sidebar kiri — Beranda; *Konten undangan*: Info, Pasangan, Acara, Cerita, Galeri, Tema; *Tamu*: Daftar tamu, RSVP, Ucapan, Hadiah; Lihat undangan ↗.
- **Ponsel:** bottom nav tetap (Beranda, Acara, Tamu, RSVP, Menu). "Menu" berupa `<details>` (tanpa JS pun berfungsi) berisi semua menu. Konten diberi `padding-bottom` supaya tidak tertutup; `env(safe-area-inset-bottom)` untuk iPhone.

Menu baru ditambahkan di `navGroups` (dan `bottomNav` bila perlu pintasan) di `src/modules/wedding/views.templ`.

## Performa

`TestDashboardLoad500Guests`: beranda dengan 500 tamu ±20 ms per request di lokal (batas 300 ms) — semua angka tamu dari satu query agregat `GuestStats`.

## Alur buat undangan (T29)

1. **Wizard** (`/dashboard/weddings/new`, modul `wedding`): langkah 1 memilih tema, langkah 2 nama mempelai & tanggal. Pilihan tema disuntikkan lewat `wedding.Deps.Themes` (dirakit di `cmd/server/main.go` dari modul theme: tema aktif, thumbnail, undangan contoh) — modul wedding tidak mengimpor modul theme. `?tema=<id>` (dari tombol "Pakai tema ini" di `/tema/<id>`) melewati langkah 1.
2. **Buat**: `POST /dashboard/weddings` membuat draf dengan `CreateInput.ThemeID`; judul = "Pernikahan <pria> & <wanita>", alamat otomatis. Tema tak dikenal/nonaktif → tema bawaan.
3. **Pratinjau pertama**: `/dashboard/weddings/:id/start` — iframe `…/theme/preview` (bagian kosong diisi contoh) + ajakan melengkapi. Tidak menampilkan harga atau tombol terbit.

Tujuan setelah daftar/masuk dibawa parameter `next` (divalidasi `safeNext`, hanya path di situs sendiri) di form daftar, form masuk, dan tautan di antara keduanya.

### Langkah terpandu (T29 bagian 2)

`wedding.Shell` menampilkan pita langkah (`guideSteps`) dan navigasi bawah (`guideNext`) bila undangan masih draf **dan** halaman aktif termasuk urutan `guide` (Mempelai → Acara → Cerita → Galeri → Tampilan → Tamu). Karena dipasang di Shell, modul lain tidak perlu diubah — cukup tetap memanggil `Shell(w, "<akhiran>")`. Menambah/mengurutkan langkah = mengubah slice `guide` di `wedding/views.templ`. Panduan tidak menyimpan state: "Lanjut" hanyalah tautan ke langkah berikutnya, jadi semua langkah bebas dilewati.
