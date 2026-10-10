# Runbook operasional Lunovia

Panduan untuk menjalankan Lunovia di produksi (Railway + Postgres + Cloudflare R2 / for SaaS). Setelan & gotcha deploy: README "Deploy (Railway)"; custom domain: [custom-domain.md](custom-domain.md).

## 1. Deploy

- Merge ke `main` → Railway build `Dockerfile` → *pre-deploy* `lovoria migrate up` → start `lovoria serve` → healthcheck `/healthz`. Migrasi gagal = deploy dibatalkan, versi lama tetap jalan.
- CI wajib hijau sebelum merge: generate-check, test + integrasi (termasuk backup/restore), lint, `govulncheck`, E2E Playwright, build image < 50 MB.
- Setelah deploy: buka `/healthz`, satu undangan publik, dan dashboard; cek log Railway tidak ada `level=ERROR`.
- Undangan contoh landing page (idempoten): `railway ssh --service lovaria -- lovoria demo seed`. Tambahkan `--refresh` untuk mengganti foto demo yang sudah ada dengan set terbaru (foto lama dihapus dari R2). Jalankan ulang setelah deploy versi yang menambah isi demo: demo lama tanpa foto akan dilengkapi foto & kutipan ("dilengkapi: …"), sedangkan yang sudah lengkap dilewati.

### Pembayaran (Midtrans, T23)

1. Buat akun di midtrans.com. Di dashboard Midtrans → **Settings → Access Keys**, salin **Server Key** (sandbox dulu).
2. Railway → service `lovaria` → Variables: `PAYMENT_GATEWAY=midtrans`, `MIDTRANS_SERVER_KEY=<server key>`, `MIDTRANS_ENV=sandbox`. Jangan menaruh kunci di repo atau chat.
3. Midtrans → **Settings → Payment → Notification URL**: `https://lunovia.id/webhooks/midtrans`. Opsional: isi Finish/Unfinish/Error Redirect URL dengan `https://lunovia.id/dashboard` sebagai cadangan — halaman bayar dibuka di tab baru dan tab Lunovia memperbarui statusnya sendiri, jadi alur tidak bergantung pada redirect balik dari Midtrans.
4. Uji sandbox: buat wedding draf → Publikasikan → Bayar → di halaman Snap pilih QRIS/VA, selesaikan lewat simulator Midtrans (`simulator.sandbox.midtrans.com`). Halaman kembali harus berubah menjadi "Pembayaran berhasil" dan beranda "Lunas ✓". Cek webhook masuk: `SELECT outcome, signature_ok, received_at FROM payment_events ORDER BY received_at DESC LIMIT 5;`
5. Uji juga: biarkan kedaluwarsa / batalkan → "Coba lagi" membuat order baru; kirim ulang notifikasi dari dashboard Midtrans → hasil `duplicate`.
6. Go-live: ganti ke Server Key **production**, `MIDTRANS_ENV=production`, dan isi Notification URL di environment production Midtrans.

Selama `PAYMENT_GATEWAY` kosong, wedding baru **belum bisa dipublikasikan** (halaman publikasi menampilkan "Pembayaran belum tersedia"); undangan yang sudah terbit tidak terpengaruh. Penelusuran keluhan "sudah bayar tapi belum lunas": cari nomor order (`LVR-…`) di `payment_orders` dan `payment_events`; bila gateway menyatakan lunas tetapi webhook gagal, kirim ulang notifikasi dari dashboard Midtrans (aman, idempoten).

## 2. Rollback

1. Railway → service **lovaria** → **Deployments** → deployment sehat sebelumnya → **Redeploy**.
2. Migrasi tidak otomatis mundur. Semua migrasi dibuat *additive* (kolom/tabel baru), jadi versi lama tetap jalan di skema baru. Bila migrasi terbaru memang harus dibatalkan: `railway ssh --service lovaria -- lovoria migrate down` (satu langkah), **setelah** backup (§3).
3. Revert commit di `main` supaya deploy berikutnya tidak mengulang masalah.

## 3. Backup & restore database

**Otomatis:** `lovoria serve` membuat backup harian pukul `BACKUP_HOUR_UTC` (default 19 = 02.00 WIB) ke bucket R2 **privat** `BACKUP_BUCKET`, format `pg_dump` custom (terkompresi), retensi `BACKUP_RETENTION_DAYS` (default 14; backup terakhir tidak pernah dihapus). Advisory lock → satu instance saja. Tambahan: aktifkan juga **Backups** bawaan Railway di service Postgres.

Setup satu kali:
1. Cloudflare R2 → **Create bucket** `lovaria-backups` — **jangan** aktifkan Public access.
2. Token R2 yang dipakai aplikasi (`R2_ACCESS_KEY_ID`/`R2_SECRET_ACCESS_KEY`) harus punya izin Object Read & Write ke bucket ini juga (atau "All buckets").
3. Railway → Variables: `BACKUP_BUCKET=lovaria-backups` → deploy. Log: `backup harian aktif`.
4. Uji sekarang: `railway ssh --service lovaria -- lovoria backup run` lalu `lovoria backup list`.

**Restore** (versi mayor Postgres target = produksi, saat ini **18**; image memakai `postgresql18-client`):
```
railway ssh --service lovaria -- lovoria backup list
railway ssh --service lovaria -- lovoria backup restore <file.dump> --to '<URL database tujuan>'
```
- **Uji restore berkala** (bulanan): Railway → New → Database → PostgreSQL (sementara), restore ke URL *internal*-nya, cek `SELECT count(*) FROM weddings, guests`, lalu hapus database sementara.
- **Pemulihan produksi**: pasang maintenance (scale ke 0 / hentikan deploy), restore ke database **baru**, arahkan `DATABASE_URL` ke database baru, deploy, verifikasi, baru hapus yang lama. Menimpa database aktif langsung (`--overwrite`) hanya bila benar-benar terpaksa.
- Backup produksi tidak bisa di-restore ke Postgres 16 lokal (pg_restore 18 menulis setting yang tidak dikenal server 16).

Teruji: `src/platform/backup` (dump → restore ke database baru, jumlah baris sama) di CI (Postgres 16) dan lokal dengan client & server **18** (setara produksi).

## 4. Rotasi secret

| Secret | Cara | Dampak |
|---|---|---|
| `APP_SECRET` | ganti di Railway (≥ 32 karakter, `openssl rand -hex 32`) → deploy | token form RSVP/ucapan di halaman yang sedang terbuka tidak berlaku (tamu diminta kirim ulang); cookie mode lihat-saja admin berakhir |
| `DATABASE_URL` / password Postgres | Railway → Postgres → regenerate credentials; `DATABASE_URL=${{Postgres.DATABASE_URL}}` ikut otomatis → redeploy lovoria | koneksi putus sebentar saat redeploy |
| R2 access key | buat token baru di Cloudflare → ganti `R2_ACCESS_KEY_ID`/`R2_SECRET_ACCESS_KEY` → deploy → hapus token lama | tidak ada (URL foto publik tidak berubah) |
| `CLOUDFLARE_API_TOKEN` | buat token baru (Zone → SSL and Certificates → Edit) → ganti → deploy → hapus lama | tidak ada |
| `SENTRY_DSN` | Sentry → Client Keys → buat baru, nonaktifkan lama | tidak ada |
| Sesi user | `DELETE FROM sessions` (semua keluar) atau nonaktifkan user di `/admin` | user login ulang |

## 5. Observability

- **Log**: JSON terstruktur di Railway, setiap request memuat `request_id`, `wedding_id`, `user_id`, `status`, `latency`. Cari keluhan pasangan: filter `wedding_id=<id>`; error 5xx: `level=ERROR`.
- **Error tracking**: isi `SENTRY_DSN` (Sentry free tier). Hanya error 5xx yang dikirim, tanpa data pribadi (IP/cookie).
- **Uptime** (gratis, mis. UptimeRobot / Better Stack): monitor HTTP 5 menit ke `https://lunovia.id/healthz` (harus 200) dan satu undangan contoh publik (mis. `https://lunovia.id/w/<slug-contoh>`, cari kata kunci nama pasangan). Kirim notifikasi ke email/Telegram pemilik.
- **Kuota custom domain**: log `WARN … mendekati kuota Cloudflare for SaaS` & kartu di `/admin` saat domain aktif ≥ 90.

## 6. Performa & kapasitas

Load test `tools/load/h1.js` (k6), skenario H-1 acara: 200 page view/detik + 50 RSVP/detik tersebar di 1.000 tamu, 2 menit.

| Tanggal | Lingkungan | p95 halaman | p95 RSVP | Catatan |
|---|---|---|---|---|
| 2026-09-27 | lokal (8 core, Postgres 16 Docker, k6 di mesin sama) — sebelum perbaikan | 2,91 s | 1,2 s | pool 10 koneksi jenuh; ditemukan crash `concurrent map writes` di resolver (diperbaiki) |
| 2026-09-27 | lokal — sesudah (pool 25 + cache data wedding 10 detik) | **45 ms** | **77 ms** | 300 req/detik, 0 crash, 0 5xx; 5 × 429 (rate limit per kode, sesuai rancangan) |

Menjalankan ulang (staging / di luar jam sibuk):
```
DATABASE_URL=<db staging> go run ./tools/loadseed -weddings 10 -guests 100 -out /tmp/lovoria-load.json
BASE_URL=https://<staging> DATA=/tmp/lovoria-load.json make loadtest
```
`loadseed` menolak URL yang mengandung `railway` supaya tidak mengisi produksi tanpa sengaja. Target Arsitektur: p95 RSVP < 300 ms, p95 halaman < 500 ms di plan Railway.

## 7. Cloudflare

- **`/static/*`**: aplikasi memakai URL ber-hash (`?v=<hash>`) dengan `Cache-Control: public, max-age=31536000, immutable` (tanpa hash: `max-age=3600`) — Cache Rule: *Cache eligible*, Edge TTL "Use cache-control header", sertakan query string di cache key.
- **Foto (R2)**: saat ini `r2.dev` (dibatasi & tidak di-cache Cloudflare). Sebelum launch: R2 → bucket `lovaria` → **Custom Domains** → `media.lunovia.id`, lalu `R2_PUBLIC_URL=https://media.lunovia.id` dan `lovoria media rebase-urls --from https://pub-….r2.dev --apply`.
- **Dashboard & admin** tidak di-cache: aplikasi mengirim `Cache-Control: private, no-store` untuk `/dashboard/*` dan `/admin/*`; jangan buat Cache Rule "Cache everything" untuk path tersebut.
- Undangan publik: `/w/*` `public, max-age=60`; `/i/*` `private, no-cache` (ETag).

## 8. Keamanan (ringkas)

- Header: CSP (`script-src 'self'` di undangan; dashboard + `'unsafe-eval'` untuk Alpine), HSTS (tanpa includeSubDomains, karena domain pasangan), `X-Frame-Options: SAMEORIGIN` (iframe preview tema), `Permissions-Policy`, `nosniff`. Tidak ada script/handler inline (diuji).
- CSRF: double-submit / Sec-Fetch-Site untuk semua POST; form publik (RSVP, ucapan) memakai token HMAC + rate limit.
- Rate limit: login/register/lupa password per IP, RSVP per kode tamu, ucapan per IP.
- Isolasi tenant: `TestTenantIsolationAllDashboardRoutes` mengakses **setiap** route `/dashboard/weddings/:id/*` sebagai user lain → 404.
- `govulncheck` di CI; per 2026-09-27: 0 kerentanan yang terpanggil.

## 9. Sinyal evaluasi ulang arsitektur (Arsitektur §10.6)

| Sinyal | Cek | Tindakan pertama |
|---|---|---|
| Latency RSVP naik saat H-1 banyak wedding | log `latency` untuk `POST /i/*/rsvp`, Sentry; load test ulang | naikkan `DB_MAX_CONNS` / plan Railway; baru pertimbangkan instance kedua (rate limit & cache in-memory perlu dipindah) |
| Query > beberapa ratus ms walau ber-index | `EXPLAIN ANALYZE` query terkait, `pg_stat_user_tables` (seq_scan) | index tambahan / perbaiki query |
| Tim bertambah, modular monolith menghambat | — | pecah per modul yang sudah ada batasnya (service boundary sudah dijaga test) |
| Page builder masuk roadmap | — | lihat Arsitektur §10.4 (islands) |
| Custom domain aktif ≥ 90 | `/admin/domains`, log WARN | anggarkan biaya hostname tambahan Cloudflare for SaaS |

## 10. Checklist launch

- [x] CI hijau: test, integrasi, lint, `govulncheck` (0 temuan), E2E smoke test, image < 50 MB
- [x] Isolasi tenant diuji otomatis untuk semua route dashboard
- [x] Security headers & halaman error bergaya; Privacy & Terms (draf)
- [x] Load test dengan target tercapai (lokal) — baseline tercatat di §6
- [x] Restore backup teruji ke database baru (lokal, Postgres 18 = produksi)
- [ ] `BACKUP_BUCKET` dibuat (privat) & diisi; `lovoria backup run` + restore uji ke database sementara Railway
- [ ] Backups bawaan Railway diaktifkan di service Postgres
- [ ] `SENTRY_DSN` diisi; uji dengan error buatan (atau tunggu 5xx pertama)
- [ ] Uptime monitor `/healthz` + satu undangan publik aktif, notifikasi ke pemilik
- [ ] R2 custom domain `media.…` + `rebase-urls` (foto lewat CDN)
- [ ] Teks final Kebijakan Privasi & Syarat Ketentuan (hapus label "Draf")
- [ ] `APP_SECRET` ≥ 32 karakter terpasang; token/secret disimpan di password manager
- [ ] Load test ulang di Railway (di luar jam sibuk) — opsional, biaya/izin
- [ ] Postgres dev/CI disamakan ke 18 (saat ini 16) — lihat catatan §3

## Sign in with Google (T30)

**Setup satu kali**
1. Google Cloud Console → buat/pilih project → **APIs & Services → OAuth consent screen**: tipe *External*, nama aplikasi "Lunovia", email dukungan, domain `lunovia.id`, tautan Kebijakan Privasi (`https://lunovia.id/privacy`) dan Syarat (`https://lunovia.id/terms`). Scope cukup `openid`, `email`, `profile` (tidak perlu verifikasi tambahan). Publikasikan aplikasinya (*In production*) supaya semua akun Google bisa masuk.
2. **Credentials → Create credentials → OAuth client ID** → *Web application*. **Authorized redirect URIs**: `https://lunovia.id/auth/google/callback` (harus persis sama dengan `BASE_URL` + `/auth/google/callback`).
3. Isi `GOOGLE_CLIENT_ID` dan `GOOGLE_CLIENT_SECRET` di Railway, lalu deploy. Tombol "Lanjutkan dengan Google" muncul di halaman masuk dan daftar.

**Cara kerja** (`src/modules/auth/google.go`, `google_handler.go`)
- Alur authorization code + PKCE + nonce di sisi server; tidak ada script Google di halaman. State disimpan di cookie `lovoria_oauth` bertanda tangan HMAC (10 menit, `Path=/auth/google`).
- Identitas disimpan di `user_identities` (provider `google` + ID akun Google). Email Google harus terverifikasi.
- Akun yang emailnya sama dihubungkan otomatis. Bila akun itu dibuat dengan password dan emailnya belum pernah terverifikasi, **password lamanya dimatikan dan semua sesinya dicabut** saat dihubungkan (mencegah orang yang lebih dulu mendaftarkan email itu tetap bisa masuk); pengguna diberi tahu dan bisa membuat password baru lewat "Lupa password".
- Akun yang dinonaktifkan admin tidak bisa masuk lewat Google.

**Bila gagal**: pengguna kembali ke halaman masuk dengan pesan umum; penyebabnya ada di log ("auth: masuk dengan Google gagal"). Yang paling sering: redirect URI di Google Cloud Console tidak sama persis, atau client secret salah.

## Check-in QR di hari H (T31) — panduan singkat

**Sebelum acara (pasangan)**
1. Dashboard → Tamu → Check-in → **Aktifkan check-in QR**. Pastikan undangan sudah terbit; QR muncul di undangan pribadi tiap tamu.
2. **Buat link penerima tamu** dan kirim ke petugas pintu masuk (WhatsApp). Minta mereka membukanya sekali sebelum acara untuk memberi izin kamera.

**Di pintu masuk (penerima tamu)**
1. Buka link → **Nyalakan kamera** → arahkan ke QR tamu.
2. Hijau: periksa nama, sesuaikan jumlah orang, tekan **Check-in**, lalu **Pindai berikutnya**. Kuning: undangan sudah dipakai. Merah: bukan undangan acara ini.
3. Tamu tanpa QR: **Cari manual**. Tamu tanpa undangan: **Tamu tanpa undangan** (dicatat terpisah).
4. Sinyal lemah: halaman menampilkan "Koneksi bermasalah" — ulangi pemindaian. Belum ada mode offline.

**Bila ada masalah**
- Link bocor / petugas berganti: **Buat link baru** (link lama langsung mati) atau **Cabut link**.
- Salah pindai: "Batalkan check-in ini" di halaman pemindai, atau **Batalkan** di dashboard.
- Kamera tidak mau menyala: periksa izin kamera browser untuk situs ini; cari manual tetap bisa dipakai.
