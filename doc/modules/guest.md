# Modul `guest` — Dokumentasi Teknis

Daftar tamu per wedding, kode undangan personal, import/export, dan data RSVP.
Kode: `src/modules/guest` · Query: `src/modules/guest/db/queries/guests.sql` (sqlc `guestdb`) · Migration: `00006_create_guests.sql`.

## Data

Tabel `guests` (tenant, wajib difilter `wedding_id`):

| Kolom | Catatan |
|---|---|
| `phone` | Dinormalisasi `NormalizePhone`: `0812…`/`+62 812…`/`812…` → `62812…`; nomor luar negeri (`+1…`) disimpan sebagai digit. `''` bila kosong. |
| `max_pax` | 1–20 (default 1). |
| `invitation_code` | 7 karakter dari `23456789ABCDEFGHJKMNPQRSTUVWXYZ` (tanpa `0 O 1 I L`), `crypto/rand`, **unik global** (`guests_invitation_code_key`) karena URL `/i/{code}` tidak memuat slug. Bentrok → retry. |
| `rsvp_*` | Diisi `UpdateRSVP` (T10). Tidak hadir → `rsvp_pax = 0`. |
| `last_opened_at` | Diisi `MarkOpened` saat undangan dibuka (T09). |

Satu-satunya query lintas wedding: `GetGuestByCode` (`-- tenant:ignore`).

## API service (dipakai modul lain)

| Method | Pemakai |
|---|---|
| `GetByCode(ctx, code)` | Resolver public site (T09). Kode dinormalisasi (trim + uppercase) dan dicek formatnya sebelum menyentuh DB. |
| `UpdateRSVP(ctx, weddingID, guestID, status, pax, message)` | RSVP (T10). Validasi `1 ≤ pax ≤ max_pax`. |
| `Responses(ctx, weddingID, status, page)` | Halaman RSVP dashboard: tamu yang sudah menjawab, terbaru dulu, filter `attending`/`declined`. |
| `MarkOpened(ctx, weddingID, guestID)` | T09. |
| `Stats(ctx, weddingID)` | Dashboard (T13): jumlah tamu & pax per status, jumlah yang sudah membuka undangan. |
| `Origin(ctx, weddingID)` + `Link(origin, code)` | Link undangan tamu `…/i/{code}` — origin = custom domain aktif (via `SetOrigins(wedding.CanonicalOrigin)`), selain itu `BASE_URL`. Dipakai editor, export CSV, dan tombol bagikan. |

## Route dashboard

Semua di bawah `/dashboard/weddings/:weddingID` (RequireAuth + RequireWeddingOwner → user lain 404).

| Method & path | Fungsi |
|---|---|
| `GET /rsvp?status=&page=` | Tab **RSVP**: statistik (hadir, total orang hadir, tidak hadir, belum konfirmasi) + respons terbaru dengan pesan tamu. Form RSVP publik: [public-site.md](public-site.md#rsvp-t10). |
| `GET /guests?q=&status=&group=&page=` | Daftar (htmx → fragment `#guests` + statistik & opsi grup via out-of-band swap). |
| `POST /guests` | Tambah. `quick=1` (baris Tambah cepat) → respons daftar + form tambah cepat baru (OOB, grup diingat, autofocus); error → `HX-Retarget: #guest-quick`. |
| `GET /guests/new`, `GET /guests/:id/edit`, `PATCH /guests/:id`, `DELETE /guests/:id` | Form lengkap & ubah/hapus satu. |
| `DELETE /guests?ids=…` | Hapus massal (ID milik wedding lain diabaikan oleh query). |
| `GET /guests/paste` → `POST /guests/paste` → `POST /guests/paste/confirm` | Tempel daftar → tabel periksa → simpan (`AddMany`, semua-atau-tidak). |
| `GET /guests/import` → `POST /guests/import` → `POST /guests/import/confirm` | Import file CSV → pratinjau → simpan baris valid (`Import`). |
| `GET /guests/import/template` | Template CSV kosong (`sep=,` + judul kolom). |
| `GET /guests/export` | CSV UTF-8 BOM, termasuk `invitation_link`. |

Form filter (`#guest-filters`) sengaja berada **di luar** area swap supaya input pencarian tidak kehilangan fokus; aksi lain menyertakannya lewat `hx-include`.

## Tempel daftar (`ParseList`)

Satu baris = satu tamu, baris kosong dilewati, maksimal 2000 baris / 2 MB.

**Mode teks bebas** (tidak ada karakter tab), urutan ekstraksi per baris:
1. Buang penomoran/bullet di awal (`1.`, `2)`, `-`, `•`, `*`, `a.`).
2. Email (`x@y.z`).
3. Jumlah orang: `(2 orang)`, `3 org`, `2 pax` — diambil **sebelum** nomor HP supaya angkanya tidak ikut terbaca sebagai nomor.
4. Nomor HP: kandidat terpanjang yang lolos `NormalizePhone` (boleh berisi spasi, `-`, `.`, `()` dan `+`).
5. Sisa teks = nama (dirapikan dari pemisah `- : ; , | /`).

**Mode tabel** (ada tab — hasil salin sel Excel/Google Sheets):
- Bila baris pertama berisi judul yang dikenali (`nama/name`, `hp/phone/whatsapp`, `email`, `grup/group`, `jumlah/pax/max_pax`), kolom dipetakan dari judul.
- Tanpa judul, setiap sel ditebak: email → HP → angka 1–20 (jumlah orang) → teks pertama = nama → teks berikutnya = grup.

Grup default dari form dipakai untuk baris yang tidak menyebut grup. Hasil parse tidak langsung disimpan; ditampilkan di tabel periksa yang bisa diedit. Konfirmasi mengirim kolom paralel (`name[] phone[] group_name[] max_pax[] email[]`) dan **divalidasi ulang di server**; baris yang semua kolomnya kosong diabaikan.

## Import file CSV (`ParseCSV`)

**Halaman import** (mobile-first): kotak unggah besar di paling atas (klik atau seret file; memilih file langsung mengirim form → pratinjau, tanpa tombol tambahan; `<noscript>` menampilkan tombol biasa), lalu panduan "Belum punya file?" 3 langkah dengan tombol **Unduh template** dan contoh dalam bentuk tabel. Banner di atas mengarahkan ke "Tempel daftar" sebagai cara termudah.

**Template** (`TemplateCSV`): baris `sep=,` + judul kolom `nama,hp,email,grup,jumlah`, sengaja **tanpa contoh tamu** (contoh yang lupa dihapus akan ikut ter-import). Baris `sep=` membuat Excel dengan pengaturan regional Indonesia (pemisah `;`) tetap memecah kolom dengan benar; parser membaca pemisahnya lalu membuang baris itu. Nomor baris di pesan error mengikuti tampilan Excel (baris `sep=` tidak ditampilkan Excel).

Pemisah `,` atau `;` (dari baris `sep=` bila ada, selain itu dideteksi dari baris judul), BOM diabaikan, nama kolom sama dengan mode tabel di atas; kolom `name` wajib ada. Pratinjau menampilkan nomor baris file untuk setiap error; konfirmasi membawa baris valid sebagai CSV di hidden field, divalidasi ulang, lalu disimpan lewat `COPY` dalam satu transaksi (500 baris ±30 ms). Baris invalid dilewati.

## Bagikan undangan (T14)

**Template pesan** (`share_templates`, migration `00016`, satu per wedding; tanpa baris = bawaan Bahasa Indonesia): placeholder `{guest_name}`, `{couple}` (nama depan "Samuel & Sarah"), `{date}` (sesuai bahasa), `{link}` — wajib ada supaya pesan selalu berisi link. Bahasa `id`/`en`, maks. 2000 karakter; teks di antara `*bintang*` tampil tebal di WhatsApp.

**Halaman Bagikan** `GET …/share` (menu Tamu → Bagikan):
- Link undangan umum = `wedding.CanonicalBaseURL` (custom domain bila aktif) + Salin, **Bagikan…** (Web Share API, hanya muncul bila browser mendukung), Kirim lewat WhatsApp (`wa.me/?text=`, pilih kontak), Salin pesan.
- Editor template dengan **preview langsung** (Alpine `sharePreview`, nilai contoh "Budi Santoso"; preview dirender sebagai teks, bukan HTML). `POST …/share/template`, `POST …/share/template/reset`.
- Ganti alamat undangan (form ke `PATCH …/slug`, modul wedding) + daftar alamat lama yang masih dialihkan.

**Per tamu di daftar tamu**: **Kirim WA** (`https://wa.me/{62…}?text=…`; tanpa nomor → `wa.me/?text=` untuk memilih kontak), **Salin link**, **Salin pesan**. Klik salah satunya memanggil `POST …/guests/:id/shared` (204) yang mengisi `shared_at` (migration `00015`) dan menampilkan badge "Sudah dibagikan". Filter daftar: Semua / Belum dibagikan / Sudah dibagikan (`shared=no|yes`).

**Encoding WhatsApp** (`WhatsAppURL`): `url.QueryEscape` dengan spasi `%20` (bukan `+`); baris baru `%0A`, emoji UTF-8, `& # ?` ter-encode — diuji dengan decode ulang. Nomor tidak dinormalisasi ulang (sudah `62…` dari T07).

## Export CSV

UTF-8 dengan BOM (Excel & Google Sheets). Nilai teks yang diawali `= + - @` diberi awalan `'` untuk mencegah formula injection saat file dibuka di spreadsheet.

## Pilih dari kontak

Memakai Contact Picker API (`navigator.contacts.select`) — hanya tersedia di Chrome / Edge / Samsung Internet **Android** (HTTPS). iPhone (Safari maupun Chrome) dan desktop tidak mendukung; tombol disembunyikan (`x-show="supported"`), jadi pengguna tidak pernah melihat tombol yang tidak berfungsi. Situs hanya menerima kontak yang dipilih pengguna, bukan seluruh buku kontak.

- **Tempel daftar tamu** — `contactPicker` (`static/js/lovoria.js`): pilih **banyak** kontak (`multiple: true`), hasilnya ditambahkan ke kotak tempel sebagai `Nama Nomor` per baris, lalu melewati alur tempel yang sama.
- **Tambah cepat & form tamu lengkap** — `contactFill`: pilih **satu** kontak; No. HP diisi, Nama diisi hanya bila masih kosong. Tambah cepat memakai ikon di dalam kolom No. HP, form lengkap memakai link "Pilih dari kontak HP" di bawah kolom.

Nomor dari kontak (`+62 857-1111-2222`, `0857…`) dinormalisasi ke `62…` di server seperti input manual.

## Test

- `paste_test.go`, `csv_test.go`, `helpers_test.go`: parser, normalisasi HP, kode undangan.
- `service_test.go`: CRUD, filter/pencarian, statistik, `GetByCode`, isolasi tenant, import 500 baris < 5 detik, `AddMany` semua-atau-tidak.
- `handler_test.go`: alur HTTP (tambah cepat, tempel → periksa → simpan, import, export, hapus massal, halaman RSVP) dan 404 untuk user lain di semua route.
- Alur RSVP publik diuji di `src/public-site/rsvp_test.go`.
