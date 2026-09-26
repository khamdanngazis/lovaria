# Lovoria

Wedding invitation service — undangan pernikahan digital (modular monolith Go, SSR).

**Stack:** Go 1.25 · Echo v4 · templ · htmx 2 + Alpine.js 3 · Tailwind CSS v4 (standalone) · PostgreSQL 16 (pgx/sqlc/goose) · Cloudflare R2 · Railway.

Dokumen: [arsitektur](doc/Lovoria-Architecture-Document.md) · [task breakdown](doc/lovoria-tasks/00-README.md) · [konvensi & aturan wajib](CONTRIBUTING.md).

## Quick Start

Prasyarat: Go ≥ 1.25 (atau Go lebih lama dengan `GOTOOLCHAIN=auto`), `make`, `curl`.

```bash
cp .env.example .env
make dev            # buka http://localhost:8090 (hot reload templ & tailwind)
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
| `DATABASE_URL` | — | Wajib mulai T02 |
| `SHUTDOWN_TIMEOUT` | `10s` | Batas graceful shutdown |
| `STATIC_FROM_DISK` | `true` di dev | `false` → aset dari embed binary |

## Deploy (Railway)

1. Buat project Railway → **Deploy from GitHub repo** (repo ini). Railway membaca [`railway.toml`](railway.toml) dan build pakai [`Dockerfile`](Dockerfile).
2. Tambahkan service **PostgreSQL**, lalu di service app set `DATABASE_URL=${{Postgres.DATABASE_URL}}`.
3. Set `APP_ENV=production` dan `BASE_URL` sesuai domain.
4. Healthcheck deploy memakai `GET /healthz`.

CI GitHub Actions ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)): templ generate check, build, vet, test (race), golangci-lint, build image Docker (cek < 50 MB) + smoke test `/healthz`.
