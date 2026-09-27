# T07 — Guest Management

**Estimasi:** 2–3 hari · **Depends on:** T04 · **Modul:** `modules/guest`

## Konteks
Daftar tamu + invitation code personal adalah dasar personalized invitation dan RSVP (Produk §8–9). Import/export termasuk "Should Have" dan dibuat di sini karena satu domain.

## Scope (In)
- Tabel `guests` (`id`, `wedding_id`, `name`, `phone`, `email` nullable, `group_name`, `max_pax` default 1, `invitation_code` unique global, `rsvp_status` enum `pending|attending|declined` default `pending`, `rsvp_pax`, `rsvp_message`, `rsvp_at`, `attendance_status` nullable, `notes`, `last_opened_at`)
- Invitation code: 7 karakter, alfabet tanpa karakter ambigu (tanpa `0 O 1 I L`), crypto-random, retry bila bentrok
- Dashboard: list tamu dengan search, filter status & group, pagination (htmx), tambah/edit/hapus, bulk delete
- Ringkasan: total, attending, declined, pending (hitungan tamu dan hitungan pax)
- Import CSV (kolom: name, phone, email, group, max_pax) dengan preview + laporan baris error sebelum commit
- Export CSV (semua kolom termasuk link invitation lengkap)
- Normalisasi nomor HP Indonesia ke format `62xxxxxxxxxx` (dipakai T14 WhatsApp)
- Service expose: `GetByCode(code)` (lintas wedding, dipakai resolver T09), `UpdateRSVP(...)` (T10), `Stats(weddingID)` (T13), `MarkOpened(id)`

## Out of Scope
- Form RSVP public (T10), share WhatsApp (T14), QR check-in (V2).

## Acceptance Criteria
- [ ] Import 500 baris < 5 detik, baris invalid dilaporkan tanpa menggagalkan baris valid (setelah konfirmasi)
- [ ] Code unik & tidak bisa ditebak berurutan
- [ ] Export bisa dibuka di Excel/Google Sheets (UTF-8 BOM)
- [ ] Isolasi tenant diuji

## Catatan untuk Agent
- Invitation code unik secara global karena URL `lovoria.com/i/{code}` tidak memuat wedding slug.

---

## Revisi UX — T07b (setelah review pengguna)

**Masalah:** upload CSV terlalu teknis untuk pasangan awam, dan input manual mewajibkan klik "Tambah tamu" untuk setiap tamu.

### Scope (In)
- **Tambah cepat**: baris input (Nama, No. HP, Grup) yang selalu terbuka di atas daftar. Enter = simpan; form dikosongkan, **grup terakhir diingat**, kursor kembali ke Nama. Form lengkap (email, jumlah orang, catatan) tetap tersedia lewat tautan "Pakai form lengkap" dan tombol "Ubah" per tamu.
- **Tempel daftar tamu** (`/guests/paste`): satu kotak teks untuk menempel daftar dari catatan HP, chat WhatsApp, atau sel Excel/Google Sheets — satu baris satu tamu. Parser mengenali nomor HP di posisi mana pun, email, "(N orang)", penomoran/bullet, dan data tab (salinan spreadsheet, dengan/tanpa baris judul). Grup default opsional untuk semua baris.
- **Tabel periksa yang bisa diedit** sebelum menyimpan: ubah nama/HP/grup/jumlah per baris, hapus baris (✕), tambah baris. Simpan bersifat **semua-atau-tidak**: bila ada baris invalid, tidak ada yang tersimpan dan baris tersebut ditandai merah.
- **Pilih dari kontak HP** di halaman tempel (Contact Picker API; hanya tampil di browser yang mendukung — Chrome Android). Kontak terpilih ditambahkan sebagai baris "Nama Nomor".
- Import file CSV tetap ada sebagai opsi lanjutan.

### Out of Scope
- Import file Excel `.xlsx` langsung (dievaluasi lagi bila masih diminta setelah fitur tempel tersedia).

### Acceptance Criteria
- [x] Menambah beberapa tamu berturut-turut hanya dengan mengetik + Enter, tanpa klik; fokus kembali ke Nama & grup diingat
- [x] Daftar yang ditempel dari catatan/chat/Excel terbaca tanpa menyiapkan file
- [x] Baris bermasalah ditandai dan bisa diperbaiki di layar yang sama; tidak ada penyimpanan sebagian
- [x] Semua route baru hanya untuk owner wedding (404 untuk user lain)
- [x] Diuji di viewport 375px dengan input keyboard sungguhan

### Revisi lanjutan — T07c (halaman import file)
- Kotak unggah besar di paling atas halaman (klik / seret file), memilih file langsung membuka pratinjau.
- Tombol **Unduh template** (CSV dengan `sep=,` supaya rapi di Excel regional Indonesia) + panduan 3 langkah; contoh ditampilkan sebagai tabel, bukan teks CSV mentah.
- Banner ke "Tempel daftar" sebagai cara termudah.
- [x] Kotak unggah terlihat tanpa scroll di viewport 375px

Detail teknis: [`doc/modules/guest.md`](../modules/guest.md).
