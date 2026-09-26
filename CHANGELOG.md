# Changelog

## [Unreleased]

### T02 — Database foundation & migrations
- `src/platform/db`: pool `pgxpool` (setelan dari env `DB_*`), `WithTx` (commit/rollback/panic, nested → savepoint), `NewID()` UUIDv7, checker DB untuk `/readyz`.
- `/readyz` → 503 bila DB tidak bisa dihubungi; detail error hanya ke log, tidak ke response.
- goose migration di-embed ke binary; subcommand `lovoria migrate up|down|status|version|redo` dan `lovoria seed` (kerangka).
- Migration awal `00001_init.sql`: extension `citext`, `pgcrypto`, fungsi trigger `set_updated_at()`.
- `sqlc.yaml` (satu package per modul, override uuid/timestamptz/citext); `make sqlc`.
- `make lint-tenant` (`tools/linttenant`): query ke tabel ber-`wedding_id` wajib memfilter `wedding_id`; `wedding_id` wajib ter-index.
- Harness integration test `db/dbtest`: database baru per test package, di-drop setelah selesai.
- `docker-compose.yml` Postgres 16 lokal (port 5433); `make db-up`, `make test-integration`.
- Binary berganti nama menjadi `lovoria`; runtime image pindah ke Alpine 3.24 (±20 MB) supaya pre-deploy command Railway punya shell.
- Railway: pre-deploy `lovoria migrate up`. CI: service Postgres, integration test wajib jalan, migration up/down/up lewat binary.
- Konvensi schema: `doc/database.md`.

### T01 — Project bootstrap & deploy pipeline
- Go module `github.com/khamdanngazis/lovaria` (Go 1.25, toolchain go1.25.14), struktur modular monolith sesuai Arsitektur §3.
- Echo v4 server: request ID, recover, request log slog JSON, secure headers, graceful shutdown (SIGINT/SIGTERM).
- `config.Load()` terpusat dari env var + `.env.example`.
- templ layout dasar `Public` & `Dashboard` dengan `<html data-theme>`.
- Tailwind v4.3.3 standalone; htmx 2.0.11 & Alpine.js 3.17.4 di-vendor di `static/js`.
- Aset statis di-embed ke binary, URL ber-hash konten + `Cache-Control: immutable` di production.
- `GET /healthz` & `GET /readyz` (checker DB menyusul di T02).
- Modul referensi `src/modules/example` (pola handler/service/repository/routes).
- Makefile (`dev`, `build`, `test`, `lint`, `migrate-up/down`), air hot reload, Dockerfile multi-stage (distroless, ±9 MB), `railway.toml`, CI GitHub Actions.
