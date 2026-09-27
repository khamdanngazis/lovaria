# Lovoria

Wedding invitation service — undangan pernikahan digital (modular monolith Go, SSR).

**Stack:** Go 1.25 · Echo v4 · templ · htmx 2 + Alpine.js 3 · Tailwind CSS v4 (standalone) · PostgreSQL 16 (pgx/sqlc/goose) · Cloudflare R2 · Railway.

Dokumen: [arsitektur](doc/Lovoria-Architecture-Document.md) · [task breakdown](doc/lovoria-tasks/00-README.md) · [konvensi & aturan wajib](CONTRIBUTING.md) · [konvensi database](doc/database.md).

## Quick Start

Prasyarat: Go ≥ 1.25 (atau Go lebih lama dengan `GOTOOLCHAIN=auto`), `make`, `curl`, Docker (untuk Postgres lokal).

```bash
cp .env.example .env
make db-up          # Postgres 16 lokal di port 5433
make migrate-up
make dev            # buka http://localhost:8090 (hot reload templ & tailwind)
```

Satu binary untuk semua perintah:

```bash
lovoria [serve]              # HTTP server
lovoria migrate up|down|status|version|redo
lovoria seed                 # data contoh (ditolak di production)
lovoria create-admin --email ops@lovoria.com [--name Ops]   # password: prompt tersembunyi / env LOVORIA_ADMIN_PASSWORD
lovoria media rebase-urls --from <url-lama> [--apply]       # ganti basis URL foto tersimpan (lihat bagian R2)
```

Admin production: `railway ssh --service lovaria -- lovoria create-admin --email <email>` lalu ketik password + Enter.

Akun dev dari `make seed`: `couple@lovoria.test` dan `admin@lovoria.test`, password `password123`.

Endpoint dasar: `GET /healthz` (proses hidup), `GET /readyz` (dependency siap), `/` (landing), `/dashboard`.

`make help` menampilkan semua target (`build`, `test`, `lint`, `migrate-up`, `migrate-down`, `docker-build`, ...).

## Konfigurasi

Semua lewat environment variable (lihat [`.env.example`](.env.example)), dibaca terpusat oleh `config.Load()`:

| Var | Default | Keterangan |
|---|---|---|
| `APP_ENV` | `development` | `development` \| `production` \| `test` |
| `PORT` | `8080` | Diisi otomatis oleh Railway |
| `BASE_URL` | `http://localhost:8080` | URL publik utama (link di email). Kosong + di Railway → `https://$RAILWAY_PUBLIC_DOMAIN` |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `DATABASE_URL` | — | Wajib (kecuali `APP_ENV=test`) |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | `10` / `0` | Ukuran pool pgx |
| `DB_MAX_CONN_LIFETIME` / `DB_MAX_CONN_IDLE_TIME` | `30m` / `5m` | Umur koneksi pool |
| `DB_CONNECT_TIMEOUT` | `5s` | Timeout membuka koneksi |
| `MAIL_DRIVER` | `log` | `log` (tulis ke log) \| `smtp` \| `resend` |
| `MAIL_FROM` | `Lovoria <no-reply@lovoria.local>` | Alamat pengirim |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` | — / `587` | Untuk `MAIL_DRIVER=smtp` (STARTTLS) |
| `RESEND_API_KEY` | — | Untuk `MAIL_DRIVER=resend` |
| `STORAGE_DRIVER` | `local` | `local` (disk, **hanya dev** — ditolak di production) \| `r2` |
| `STORAGE_LOCAL_DIR` | `$TMPDIR/lovoria-media` | Folder driver `local` (disajikan di `/media/*`) |
| `STORAGE_QUOTA_MB` | `500` | Kuota foto per wedding |
| `R2_ACCOUNT_ID` / `R2_ACCESS_KEY_ID` / `R2_SECRET_ACCESS_KEY` / `R2_BUCKET` | — | Wajib untuk `STORAGE_DRIVER=r2` |
| `R2_PUBLIC_URL` | — | URL publik bucket, mis. `https://media.lovoria.com` atau `https://pub-xxx.r2.dev` |
| `R2_ENDPOINT` | `https://<account>.r2.cloudflarestorage.com` | Override endpoint S3 (opsional) |
| `APP_SECRET` | acak per proses | Kunci HMAC token form RSVP publik (min. 32 karakter). **Isi di production**, mis. `openssl rand -hex 32` |
| `LIFECYCLE_ARCHIVE_DAYS` | `365` | Wedding berstatus Kenangan diarsipkan otomatis setelah N hari |
| `DATABASE_URL_TEST` | — | Hanya untuk integration test (user harus boleh `CREATE DATABASE`) |
| `SHUTDOWN_TIMEOUT` | `10s` | Batas graceful shutdown |
| `STATIC_FROM_DISK` | `true` di dev | `false` → aset dari embed binary |

## Deploy (Railway)

Production: https://lovaria-production.up.railway.app — project `affectionate-grace`, environment `production`, service `lovaria` + `Postgres`. Setiap push/merge ke `main` otomatis di-build dari [`Dockerfile`](Dockerfile).

Railway sudah tidak membaca `railway.toml` (Config as Code deprecated), jadi setelan disimpan langsung di service `lovaria` → **Settings**:

| Setelan | Nilai |
|---|---|
| Source | repo GitHub ini, branch `main` |
| Builder | Dockerfile (terdeteksi otomatis) |
| Custom Start Command | `lovoria serve` |
| Pre-deploy step | `lovoria migrate up` — gagal → deploy dibatalkan, versi lama tetap jalan |
| Healthcheck Path | `/healthz` |
| Networking | domain Railway, target port `8080` |

Variabel service `lovaria` (tab **Variables**):

| Variabel | Nilai |
|---|---|
| `DATABASE_URL` | `${{Postgres.DATABASE_URL}}` (reference ke service `Postgres`) |
| `APP_ENV` | tidak perlu diisi — Dockerfile sudah men-set `production` |
| `BASE_URL` | kosong → otomatis `https://$RAILWAY_PUBLIC_DOMAIN`; isi saat domain final sudah ada |
| `MAIL_DRIVER` + kredensial | isi (`resend`/`smtp`) supaya email reset password benar-benar terkirim; default `log` |
| `STORAGE_DRIVER=r2` + `R2_*` | **wajib** — tanpa ini aplikasi menolak start di production |

### Cloudflare R2 (foto)

1. R2 → **Create bucket** (mis. `lovoria-media`).
2. Bucket → Settings → **Public access**: sambungkan custom domain (mis. `media.lovoria.com`, di balik CDN Cloudflare) atau aktifkan subdomain `r2.dev` untuk awal. Nilai ini = `R2_PUBLIC_URL`.
3. R2 → **Manage R2 API Tokens** → Create API token, permission **Object Read & Write**, dibatasi ke bucket tersebut → `R2_ACCESS_KEY_ID` & `R2_SECRET_ACCESS_KEY`.
4. `R2_ACCOUNT_ID` = ID akun (bagian depan `https://<account>.r2.cloudflarestorage.com`).

⚠️ `R2_PUBLIC_URL` **bukan** endpoint `https://<account>.r2.cloudflarestorage.com` (itu S3 API yang butuh tanda tangan — browser mendapat `InvalidArgument: Authorization`). Aplikasi menolak start bila nilainya endpoint API.

**Mengganti domain publik foto** (mis. memperbaiki `R2_PUBLIC_URL`, atau pindah dari `r2.dev` ke `media.lovoria.com`): URL foto disimpan lengkap di DB, jadi setelah variabel diganti & ter-deploy jalankan:

```bash
railway ssh --service lovaria -- lovoria media rebase-urls --from <url-lama>          # simulasi: hitung baris terdampak
railway ssh --service lovaria -- lovoria media rebase-urls --from <url-lama> --apply  # terapkan (default --to = R2_PUBLIC_URL)
```

Objek di bucket tidak berubah; hanya URL tersimpan (gallery, foto utama, foto pasangan, foto cerita). Aman dijalankan ulang.

Upload selalu lewat server (resize + strip EXIF), jadi bucket tidak butuh CORS. Objek bersifat immutable (key unik) dan disajikan dengan `Cache-Control: immutable`.

Catatan: perubahan setelan/variabel di dashboard masuk sebagai *staged changes* — klik **Deploy / Apply changes** supaya berlaku. `GET /readyz` → 503 bila DB tidak bisa dihubungi.

CI GitHub Actions ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)): templ/sqlc generate check, build, vet, lint-tenant, test unit + integration (service Postgres), migration up/down/up lewat binary, golangci-lint, build image Docker (cek < 50 MB) + smoke test `/healthz`.
