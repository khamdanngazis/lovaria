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
```

Endpoint dasar: `GET /healthz` (proses hidup), `GET /readyz` (dependency siap), `/` (landing), `/dashboard`.

`make help` menampilkan semua target (`build`, `test`, `lint`, `migrate-up`, `migrate-down`, `docker-build`, ...).

## Konfigurasi

Semua lewat environment variable (lihat [`.env.example`](.env.example)), dibaca terpusat oleh `config.Load()`:

| Var | Default | Keterangan |
|---|---|---|
| `APP_ENV` | `development` | `development` \| `production` \| `test` |
| `PORT` | `8080` | Diisi otomatis oleh Railway |
| `BASE_URL` | `http://localhost:8080` | URL publik utama |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |
| `DATABASE_URL` | — | Wajib (kecuali `APP_ENV=test`) |
| `DB_MAX_CONNS` / `DB_MIN_CONNS` | `10` / `0` | Ukuran pool pgx |
| `DB_MAX_CONN_LIFETIME` / `DB_MAX_CONN_IDLE_TIME` | `30m` / `5m` | Umur koneksi pool |
| `DB_CONNECT_TIMEOUT` | `5s` | Timeout membuka koneksi |
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
| `BASE_URL` | isi saat domain final sudah ada |

Catatan: perubahan setelan/variabel di dashboard masuk sebagai *staged changes* — klik **Deploy / Apply changes** supaya berlaku. `GET /readyz` → 503 bila DB tidak bisa dihubungi.

CI GitHub Actions ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)): templ/sqlc generate check, build, vet, lint-tenant, test unit + integration (service Postgres), migration up/down/up lewat binary, golangci-lint, build image Docker (cek < 50 MB) + smoke test `/healthz`.
