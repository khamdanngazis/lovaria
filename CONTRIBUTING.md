# Contributing ke Lovoria

Dokumen ini mengunci konvensi codebase. Baca sebelum mengerjakan task apa pun di `doc/lovoria-tasks/`.

## 9 Aturan Wajib

Sumber: `doc/lovoria-tasks/00-README.md` dan Arsitektur §3 & §10.5. PR yang melanggar tidak di-merge.

1. **Tidak ada query lintas modul.** Modul A memanggil *service* modul B, bukan tabel B.
2. **Setiap tabel tenant wajib punya `wedding_id`**, dan setiap query tenant wajib memfilter `wedding_id`.
3. **Logic tema hanya lewat theme registry** (`src/modules/theme`).
4. **Endpoint "API-shaped"**: route & payload masuk akal walau saat ini mengembalikan HTML fragment.
5. **Foto selalu ke R2**, tidak pernah ke disk/volume Railway.
6. **Resolusi wedding hanya lewat 1 middleware** (Host header dulu → fallback slug/kode).
7. **Island** (Svelte/animasi berat) di luar MVP; jangan pernah mengambil alih routing SSR.
8. **Mobile-first** untuk semua halaman public (cek di viewport 375px).
9. **Setiap task menyertakan test** (unit untuk service, handler test untuk endpoint penting) dan **migration yang reversible** (`-- +goose Down` wajib diisi).

## Struktur Folder

```text
cmd/server/            entrypoint; dependency injection manual (tanpa framework DI)
src/platform/          infrastruktur lintas modul (bukan fitur)
  config/              config.Load() — satu-satunya tempat membaca env var
  db/                  pgxpool, WithTx, NewID (UUIDv7), migrate; db/dbtest = harness integration test
  seed/                kerangka `lovoria seed`
  logger/              slog JSON
  server/              Echo + middleware global + graceful shutdown
  health/              /healthz (proses hidup) & /readyz (dependency, mis. DB)
  web/                 helper HTTP (Render, Redirect, IsHTMX, FormatDateID) + user, wedding_id & token CSRF di context
  mail/                interface Mailer (log | smtp | resend)
src/modules/<nama>/    modul bisnis: auth, wedding, guest, gallery, guestbook, gift, theme, domain, admin
src/templates/
  layouts/             layout dasar Public & Dashboard (<html data-theme="...">)
  ui/                  komponen form dashboard (Input, TextArea, Card, Alert, tombol)
  themes/<tema>/       layout .templ per tema (T08)
  shared/              komponen lintas tema: RSVP form, guestbook, gift
src/public-site/       routing & rendering website wedding (package publicsite)
src/dashboard/         UI couple
src/styles/app.css     input Tailwind
static/                aset statis (di-embed ke binary); css/app.css hasil build, js/ vendored
migrations/            SQL migration goose (di-embed ke binary)
tools/linttenant/      cek aturan wedding_id pada query & index (make lint-tenant)
```

Konvensi schema, migration, sqlc, transaksi, dan integration test: **[doc/database.md](doc/database.md)**.

## Pola Modul

Contoh lengkap ada di [`src/modules/example`](src/modules/example) (hanya dipasang saat `APP_ENV=development` di `/_example`). Setiap modul mengikuti pola yang sama:

| File | Isi |
|---|---|
| `repository.go` | Akses data lewat query **sqlc** di `db/` (package `<m>db`). Setiap method tenant menerima `weddingID` dan memfilternya. Tidak diekspor ke modul lain. |
| `db/queries/*.sql` | Query sqlc modul. Hasil generate di `db/*.go` (di-commit). |
| `service.go` | Business logic + validasi. **Satu-satunya API publik modul** — modul lain hanya boleh memanggil `Service`. Error domain sebagai `var ErrXxx = errors.New(...)`. |
| `handler.go` | HTTP handler Echo. Parse input → panggil service → render templ fragment. Map error domain ke status HTTP (422 validasi, 404 tidak ditemukan). |
| `routes.go` | `type Deps struct{...}` dan `func Register(g *echo.Group, deps Deps)`. |
| `views.templ` | Komponen templ milik modul. |
| `*_test.go` | Unit test service + handler test (`httptest`). |

Wiring di `cmd/server/main.go`:

```go
repo := wedding.NewRepository(pool)
svc := wedding.NewService(repo)
wedding.Register(e.Group("/dashboard/wedding"), wedding.Deps{Service: svc})
```

Bila modul butuh modul lain, suntikkan **service**-nya lewat `Deps` (bukan repository/pool DB-nya).

### Auth & CSRF

- Route couple dipasang di group `/dashboard` (sudah `RequireAuth`); route admin di `/admin` (`RequireAuth` + `RequireRole(admin)`). Jangan cek login manual di handler.
- User yang login: `auth.CurrentUser(ctx)` / `web.CurrentUser(ctx)` (di template: `web.CurrentUser(ctx)`). Modul lain cukup mengimpor `platform/web`, bukan modul `auth`.
- Semua request non-GET dilindungi CSRF secara global. Form htmx otomatis mengirim header `X-CSRF-Token` (dari `hx-headers` di `<body>`); form biasa wajib menyertakan `@layouts.CSRFField()`.
- Validasi gagal → status **422** + fragment form berisi pesan (htmx dikonfigurasi men-swap 422). Sukses → `web.Redirect(c, url)` (otomatis `HX-Redirect` untuk htmx, 303 untuk form biasa).

### Route dashboard per wedding

Semua halaman dashboard milik satu wedding berada di `/dashboard/weddings/:weddingID/...` dan **wajib** dipasang lewat `wedding.OwnerGroup(...)`, yang menjalankan `RequireWeddingOwner`:

```go
owned := wedding.Register(dash.Group("/weddings"), wedding.Deps{Service: weddingSvc})
event.Register(owned, event.Deps{...}) // → /dashboard/weddings/:weddingID/events
```

- Wedding milik user lain, ID tidak valid, atau tidak ada → **404** (bukan 403), supaya keberadaan ID tidak bocor.
- Handler membaca `wedding_id` dari `web.WeddingID(ctx)` — **jangan** dari `c.Param("weddingID")`.
- Test wajib mencakup kasus "user A mengakses wedding user B → 404" untuk route baru.

- Halaman per wedding memakai `wedding.Shell(w, "/suffix")` (judul + tab). Tab baru ditambahkan di `wedding/views.templ`.
- Pola CRUD daftar (lihat `wedding/event`): satu fragment `<section id="...">` yang di-swap ulang setelah setiap perubahan; validasi gagal → 422 + `web.Retarget(c, "#form-id")` supaya hanya form yang dirender ulang; tanpa JS → redirect 303 ke daftar.
- Urutan manual (`sort_order`): pakai `wedding/internal/order` dan transaksi `... FOR UPDATE` (lihat `ListEventsForUpdate`).

### Komponen form & method

- Komponen form dashboard: `src/templates/ui` (`ui.Input`, `ui.TextArea`, `ui.Card`, `ui.Alert`, `ui.Notice`, tombol).
- Update memakai `PATCH`/`DELETE`: htmx langsung `hx-patch`; form tanpa JS memakai `method="post"` + `<input type="hidden" name="_method" value="PATCH">` (method override global).
- Tanggal untuk tampilan: `web.FormatDateID(t)` → "Sabtu, 12 Desember 2026".

### Route "API-shaped"

Gunakan resource & verb HTTP yang wajar, mis. `GET /weddings/:weddingID/guests`, `POST /weddings/:weddingID/guests`, `PATCH /guests/:id`. Hindari `/doSomething` atau `/get-guest-list-html`. Payload form memakai nama field yang sama dengan yang akan dipakai JSON nantinya.

## Frontend

- **templ** untuk semua HTML. File `*_templ.go` **di-commit**; CI mengecek hasil `templ generate` sudah sinkron.
- **htmx 2 + Alpine.js 3** di-vendor di `static/js` (bukan CDN). Upgrade = ganti file + catat di CHANGELOG.
- **Tailwind v4 standalone CLI** (`make tools`). Jangan tambahkan Node/Vite/Webpack.
- Aset direferensikan lewat `static.URL("css/app.css")` supaya dapat hash konten (`?v=...`) dan cache immutable di production.
- `<html data-theme="...">` diisi dari `layouts.Meta.Theme`; nilainya ditentukan theme registry (T08).

## Workflow Lokal

```bash
cp .env.example .env
make tools        # unduh tailwindcss & sqlc ke ./bin
make db-up        # Postgres lokal (docker compose, port 5433)
make migrate-up
make dev          # air (templ generate + rebuild) + tailwind --watch → http://localhost:8090
make test         # go test -race ./... (integration test DB di-skip)
make test-integration  # termasuk integration test Postgres
make lint         # gofmt, templ fmt, go vet, golangci-lint, lint-tenant
make build        # binary ke ./bin/lovoria
make migrate-new name=create_weddings
make migrate-down / make migrate-status / make seed
make sqlc         # generate query sqlc
```

## Definition of Done

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` lolos (`make lint test`)
- [ ] Migration `up` dan `down` jalan bersih
- [ ] Tidak melanggar 9 aturan wajib di atas
- [ ] Halaman baru dicek di viewport 375px
- [ ] README/CHANGELOG/CONTRIBUTING diupdate bila ada konvensi baru
