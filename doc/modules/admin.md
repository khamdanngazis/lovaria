# Panel admin (T16)

Route `/admin/*`, dilindungi `RequireAuth` + `RequireRole(admin)` di `cmd/server/main.go`; couple mendapat 403 di **semua** route (`TestCoupleCannotAccessAdmin` membaca daftar route dari router, jadi route baru otomatis ikut teruji). Akun admin dibuat dengan `lovoria create-admin --email …`. Setelah login, admin tanpa wedding diarahkan ke `/admin` (`GET /dashboard`); header dashboard menampilkan link **Admin**.

## Aturan data

Modul admin **hanya** memiliki tabel `packages`, `wedding_packages`, `admin_audit_logs` (migration `00013`); `TestAdminQueriesOwnTablesOnly` memeriksa query-nya. Data modul lain lewat service modul itu:

| Kebutuhan | Service |
|---|---|
| Customers (cari, detail, nonaktifkan) | `auth.SearchUsers`, `GetUser`, `SetDisabled` |
| Daftar wedding (filter status, tanggal, cari judul/slug; urut dibuat/tanggal/storage), ringkasan status, pemakaian tema, total storage | `wedding.AdminList`, `CountByStatus`, `CountByTheme`, `StorageTotal` — query laporan lintas tenant di modul wedding |
| Ubah status | `wedding.Transition` dengan aktor admin (`wedding.TargetsFor(status, ActorAdmin)`; mis. Diarsipkan → Kenangan) |
| Detail wedding | `wedding.GetCouple`, `History`; `guest.Stats`; `gallery.StorageUsage`; `domain.Get` |
| Tema | `theme.All`, `Disabled`, `SetEnabled` (tabel `disabled_themes`, migration `00014`, milik modul theme) |
| Custom domain | `domain.ListAll`, `ActiveCount`, `QuotaWarn` |

## Fitur

| Halaman | Isi |
|---|---|
| Ringkasan | jumlah customer & wedding, storage, domain aktif / 100 (peringatan saat ≥ 90), wedding per status, aksi admin terbaru |
| Customers | cari nama/email, detail + wedding milik user, tombol email, **nonaktifkan / aktifkan** |
| Weddings | filter & urutan, paginasi 25; detail: pasangan, pemilik, tamu & RSVP, storage, domain, paket, riwayat status, audit wedding itu; aksi: ubah status, pasang paket, buka website, **lihat dashboard (lihat saja)** |
| Tema | registry + jumlah pemakai; nonaktifkan untuk pasangan baru |
| Paket | buat / ubah / hapus (paket terpakai tidak bisa dihapus) |
| Storage | total + 20 wedding terbesar (terpakai / kuota) |
| Domain | semua custom domain + status, jumlah aktif terhadap kuota 100 |
| Audit log | semua aksi admin, terbaru dulu |

### Akun nonaktif
Kolom `users.disabled_at` (migration `00012`). Login ditolak dengan pesan "akun ini dinonaktifkan" — **hanya setelah password benar**, password salah tetap pesan generik. Sesi aktif langsung dihapus dan `GetActiveSession` menolak user nonaktif. Admin tidak bisa menonaktifkan akunnya sendiri.

### Paket
Wedding tanpa paket memakai default config (`STORAGE_QUOTA_MB`, `LIFECYCLE_ARCHIVE_DAYS`). Paket dipasang lewat `wedding_packages`, lalu dipakai:
- kuota upload galeri — `gallery.SetQuotaSource(admin.QuotaBytes)`;
- lama Kenangan sebelum diarsipkan — `wedding.SetArchiveDaysSource(admin.ArchiveDays)`.
Harga hanya teks tampilan (tanpa payment gateway).

### Tema nonaktif
Tidak muncul di pilihan tema pasangan dan ditolak saat disimpan ("Tema ini sedang tidak tersedia"), kecuali wedding yang sudah memakainya. Wedding baru dari wizard tetap memakai tema default (`elegant`).

### Lihat dashboard sebagai pasangan (read-only)
`POST /admin/weddings/:id/view` memasang cookie `lovoria_admin_view` (HMAC `APP_SECRET`: admin + wedding + kedaluwarsa 1 jam) lalu membuka dashboard wedding itu. `wedding.RequireWeddingOwner` memanggil `AdminAccess.ReadOnlyAccess` hanya bila user bukan pemilik; lolos → hanya **GET/HEAD** (selain itu 403), `web.ReadOnly(ctx)` menampilkan banner kuning dan membungkus konten dalam `<fieldset disabled>`. Cookie hanya berlaku untuk admin yang membuatnya dan wedding itu. Mulai & keluar (`POST /admin/view/stop`) tercatat di audit.

## Audit log
Setiap method tulis di `admin.Service` memanggil `audit()` setelah aksi berhasil: `user.disable|enable`, `wedding.status`, `wedding.package`, `wedding.view_as_couple|view_end`, `theme.disable|enable`, `package.create|update|delete`. Baris menyimpan `admin_email` (tetap terbaca bila akun admin dihapus) dan `details` (jsonb). Aksi yang gagal validasi tidak dicatat. `TestEveryWriteIsAudited` menguji semua aksi lewat HTTP.

## Performa
`TestWeddingListWith1000`: 1.000 wedding, daftar urut storage halaman 2 ±4 ms di lokal.
