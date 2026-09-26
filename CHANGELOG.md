# Changelog

## [Unreleased]

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
