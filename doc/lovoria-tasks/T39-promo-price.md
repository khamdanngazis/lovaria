# T39 — Harga Promo & Kode Promo

**Estimasi:** 1,5–2 hari · **Depends on:** T23 (pembayaran publikasi), T25 (`app_settings`), T32 (landing) · **Modul:** `src/modules/payment` (`promo.go`, `service.go`, `handler.go`, `views.templ`), `src/modules/admin` (`pricing.go`, `views.templ`), `src/public-site` (`landing.templ`, `seo.go`, `pages.templ`), `migrations/00031_promo_codes.sql`

## Konteks

Brief awal:

1. Landing menampilkan harga normal yang dicoret (mis. Rp240.000) dan harga promo di sebelahnya (mis. Rp59.000).
2. Pasangan bisa mengisi kode promo saat mempublikasikan undangan.
3. Harga promo di landing berasal dari kode promo yang ditandai **default**. Kode itu juga terisi otomatis di halaman publikasi.
4. Harga dan promo diatur dari panel admin.

Sebelum T39 hanya ada satu harga, yaitu `PUBLISH_PRICE_IDR`. Harga ini dibaca sekali saat start, jadi mengubahnya berarti harus deploy ulang.

## Keputusan desain

- **Jenis potongan:** persen (1–99) atau nominal rupiah, dihitung dari harga dasar.
- **Batasan kode:** hanya aktif/nonaktif. Tidak ada masa berlaku dan tidak ada kuota.
- **Tidak ada promo gratis.** Harga akhir minimal `payment.MinAmount` (Rp10.000), batas aman untuk VA gateway. Semua pembayaran tetap lewat gateway, dan CHECK `amount > 0` serta alur webhook tidak berubah. Untuk kasus gratis, admin tetap bisa "Tandai lunas".
- **Harga dasar disimpan di `app_settings`** dengan key `publish_price`, milik modul admin, mengikuti pola T25. Nilai ini dimuat saat start lewat `admin.LoadPricing` lalu `payment.Service.SetPrice` (atomic). Tanpa setelan, `PUBLISH_PRICE_IDR` tetap dipakai.
- **Kode promo disimpan di tabel `promo_codes`, milik modul payment.** Admin mengubahnya lewat service payment, karena tidak boleh ada query lintas modul. Kolom `is_default` dijaga satu baris lewat unique index parsial.
- **Order mencatat** `promo_code` dan `original_amount`. `amount` adalah harga setelah potongan, sehingga pemeriksaan nominal webhook tetap ketat.
- **Order pending dipakai ulang hanya bila kode dan nominalnya sama.** Bila pasangan mengganti kode, order lama ditutup dulu dengan alur yang sama seperti "Ganti metode".
- **Kode diperiksa sebelum bayar.** Tombol "Terapkan" mengirim `GET …/publish?promo=` dan jalan tanpa JS. Form Bayar, yang membuka gateway di tab baru, hanya membawa kode yang sudah valid.
- **Promo default yang tidak berlaku tidak tampil.** Bila kode default nonaktif atau potongannya menjadi di bawah batas, landing kembali menampilkan harga normal. Admin juga tidak bisa menurunkan harga dasar sampai potongan promo aktif menjadi kebablasan.

## Pekerjaan

### 1. Migrasi `00031_promo_codes.sql`
- Tabel `promo_codes`: `code` UPPERCASE dan unik, `discount_type`, `discount_value`, `active`, `is_default`.
- `payment_orders`: tambah kolom `promo_code` dan `original_amount`. Untuk baris lama, `original_amount` diisi sama dengan `amount`.

### 2. Payment
- File `promo.go`: `Quote`, `DefaultQuote`, `ValidatePromo`, dan CRUD promo.
- `CreateOrder` dan `ChangeMethod` menerima parameter kode promo.
- Halaman publikasi:
  - Harga normal dicoret, lalu harga promo, lalu badge "Promo KODE · hemat X".
  - Isian kode promo dengan pesan bila kodenya tidak berlaku.
  - Riwayat order menampilkan kode promo.

### 3. Admin `/admin/pricing` "Harga & Promo"
- Form harga dasar.
- Tabel promo dengan aksi jadikan/lepas default, aktif/nonaktif, dan hapus.
- Form tambah promo.
- Semua aksi tercatat di audit log: `pricing.update`, `promo.create`, `promo.default`, `promo.toggle`, `promo.delete`.
- Daftar order menampilkan kode promo.

### 4. Landing, SEO, dan legal
- `Handler.Pricing` (interface `DefaultQuote`) menggantikan `PublishPrice`.
- Bagian Harga, FAQ, `llms.txt`, dan JSON-LD `Offer.price` memakai harga promo.
- Syarat & Ketentuan mendapat satu butir tentang harga promo.

### 5. Test
- `payment/promo_test.go`: perhitungan harga, aksi admin, order berpromo, dan halaman publikasi lewat HTTP.
- `admin_test.go`: `TestPricingAndPromos` dan audit.
- `landing_test.go`: harga dicoret.
