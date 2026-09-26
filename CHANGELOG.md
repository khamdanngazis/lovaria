# Changelog

## [Unreleased]

### T04 — Wedding core & setup wizard
- Migration `00003_create_weddings.sql`: `weddings` (akar tenant; slug citext unik + CHECK `[a-z0-9-]`, status `draft|published|wedding_day|memory|archived`, `theme_id` default `elegant`) dan `couples` (`wedding_id` unik).
- Modul `wedding` (sqlc `weddingdb`): `CreateWedding`, `GetWedding`, `GetWeddingForOwner`, `GetWeddingBySlug`, `GetCouple`, `UpdateWeddingInfo`, `UpdateCouple`, `ListWeddingsByOwner`; interface baca-saja `wedding.Reader` untuk modul lain.
- Slug otomatis dari nama pasangan (`khamdan-sarah`, diakritik dibuang), suffix `-2`, `-3`… bila bentrok, retry saat race; `ValidateSlug` + daftar kata terlarang (dipakai T14).
- `RequireWeddingOwner` + `wedding.OwnerGroup`: wedding milik orang lain / ID tidak valid → 404; `wedding_id` tersedia di `web.WeddingID(ctx)`.
- Setup wizard 3 langkah (htmx, stateless, tombol Kembali, tanpa JS tetap jalan); halaman ringkasan, edit info wedding, edit pasangan (URL foto; upload di T06).
- `/dashboard` → wizard (belum punya wedding), langsung ke wedding (1), atau daftar (>1).
- Method override global (`_method`) untuk `PATCH` dari form tanpa JS; komponen form `templates/ui`; `web.FormatDateID`.
- Diuji di browser headless 375px: wizard sampai selesai, halaman edit, tanpa scroll horizontal.

### Fix
- `lovoria create-admin`: prompt password tidak lagi macet lewat `railway ssh` (Enter dikirim sebagai `\r`), input disembunyikan di terminal, dan password bisa diberikan lewat env `LOVORIA_ADMIN_PASSWORD`.

### T03 — Authentication & session
- Migration `00002_create_auth.sql`: `users` (email citext unique, role `couple|admin`, `email_verified_at`), `sessions`, `password_reset_tokens`. Token disimpan sebagai sha256, nilai mentah hanya di cookie/email.
- Modul `auth` (sqlc `authdb`): register (auto login), login (pesan error generik + dummy hash anti-timing), logout, lupa & reset password (token sekali pakai, 1 jam, semua session dihapus setelah reset).
- Password argon2id (PHC, parameter OWASP, rehash otomatis bila parameter berubah).
- Session server-side, cookie `HttpOnly` + `Secure` (production) + `SameSite=Lax`, rolling expiry 30 hari (diperpanjang maks. 1x/hari); cleanup session kedaluwarsa tiap jam.
- Middleware `LoadSession`, `RequireAuth` (redirect ke `/login?next=`, `HX-Redirect` untuk htmx), `RequireRole` (403), `CurrentUser(ctx)`. `/dashboard/*` & `/admin/*` terlindungi.
- CSRF global (Echo: `Sec-Fetch-Site` + fallback token double-submit, header `X-CSRF-Token` / field `_csrf`), gagal → 403.
- Rate limit per IP (in-memory) untuk login, register, lupa/reset password → 429.
- Form login/register/lupa/reset password (templ + htmx, validasi inline saat blur, tetap jalan tanpa JS), dicek di viewport 375px.
- `platform/mail`: interface `Mailer` dengan driver `log` (default), `smtp`, `resend`.
- CLI `lovoria create-admin`; seeder dev `couple@lovoria.test` / `admin@lovoria.test`.
- `BASE_URL` otomatis dari `RAILWAY_PUBLIC_DOMAIN` bila kosong.

### Deploy
- Hapus `railway.toml`: Railway sudah tidak membaca Config as Code. Setelan deploy (start `lovoria serve`, pre-deploy `lovoria migrate up`, healthcheck `/healthz`, `DATABASE_URL`) kini disimpan di service Railway dan didokumentasikan di README.

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
