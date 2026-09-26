# T01 — Project Bootstrap & Deploy Pipeline

**Estimasi:** 1–2 hari · **Depends on:** — · **Modul:** root, `/src`

## Konteks
Fondasi codebase modular monolith (Arsitektur §2–3). Semua task lain dibangun di atas struktur ini, jadi konvensi harus dikunci di sini.

## Scope (In)
- Inisialisasi Go module, struktur folder sesuai Arsitektur §3:
  `/src/modules/{auth,wedding,guest,gallery,guestbook,gift,theme,domain,admin}`, `/src/templates/{themes,shared}`, `/src/public-site`, `/src/dashboard`, `/cmd/server`
- Echo v4 server dengan graceful shutdown, request logging (`slog`, JSON), recover middleware, request ID
- Config via env vars (struct terpusat `config.Load()`), `.env.example`
- templ setup + 1 layout dasar dashboard & 1 layout dasar public
- Tailwind standalone CLI + build ke `/static/css/app.css`; htmx & Alpine disajikan dari `/static/js` (vendored, bukan CDN)
- Static file serving dengan cache header
- `GET /healthz` (cek proses) dan `GET /readyz` (cek DB, diisi di T02)
- `Makefile`: `dev` (air/templ watch + tailwind watch), `build`, `test`, `lint`, `migrate-up/down`
- `Dockerfile` multi-stage (binary statis, image final distroless/alpine)
- Deploy ke Railway (service + Postgres), CI GitHub Actions: build, vet, test, templ generate check

## Out of Scope
- Schema database (T02), auth (T03), fitur apa pun.

## Detail Teknis
- Tiap modul punya pola: `handler.go`, `service.go`, `repository.go` (sqlc), `routes.go` (fungsi `Register(e *echo.Group, deps)`)
- Dependency injection manual di `cmd/server/main.go` (tanpa framework DI)
- Template layout memakai `data-theme` attribute di `<html>` (dipakai T08)

## Acceptance Criteria
- [ ] `make dev` menjalankan server + hot reload templ & tailwind
- [ ] `/healthz` → 200 di lokal dan di Railway
- [ ] CI hijau di PR
- [ ] Image Docker < 50 MB
- [ ] Contoh modul kosong (`modules/example`) mendemonstrasikan pola handler/service/repository, lalu dihapus atau didokumentasikan di `CONTRIBUTING.md`

## Catatan untuk Agent
- Jangan tambahkan dependency frontend build (Vite/Webpack). Tailwind standalone cukup.
- Tulis `CONTRIBUTING.md` berisi 9 aturan wajib dari `00-README.md`.
