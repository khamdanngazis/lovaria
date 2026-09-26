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
  logger/              slog JSON
  server/              Echo + middleware global + graceful shutdown
  health/              /healthz (proses hidup) & /readyz (dependency, mis. DB)
  web/                 helper HTTP (Render templ, IsHTMX)
src/modules/<nama>/    modul bisnis: auth, wedding, guest, gallery, guestbook, gift, theme, domain, admin
src/templates/
  layouts/             layout dasar Public & Dashboard (<html data-theme="...">)
  themes/<tema>/       layout .templ per tema (T08)
  shared/              komponen lintas tema: RSVP form, guestbook, gift
src/public-site/       routing & rendering website wedding (package publicsite)
src/dashboard/         UI couple
src/styles/app.css     input Tailwind
static/                aset statis (di-embed ke binary); css/app.css hasil build, js/ vendored
migrations/            SQL migration goose
```

## Pola Modul

Contoh lengkap ada di [`src/modules/example`](src/modules/example) (hanya dipasang saat `APP_ENV=development` di `/_example`). Setiap modul mengikuti pola yang sama:

| File | Isi |
|---|---|
| `repository.go` | Akses data. Mulai T02 berisi query **sqlc**. Setiap method tenant menerima `weddingID` dan memfilternya. Tidak diekspor ke modul lain. |
| `service.go` | Business logic + validasi. **Satu-satunya API publik modul** — modul lain hanya boleh memanggil `Service`. Error domain sebagai `var ErrXxx = errors.New(...)`. |
| `handler.go` | HTTP handler Echo. Parse input → panggil service → render templ fragment. Map error domain ke status HTTP (422 validasi, 404 tidak ditemukan). |
| `routes.go` | `type Deps struct{...}` dan `func Register(g *echo.Group, deps Deps)`. |
| `views.templ` | Komponen templ milik modul. |
| `*_test.go` | Unit test service + handler test (`httptest`). |

Wiring di `cmd/server/main.go`:

```go
repo := wedding.NewRepository(db)
svc := wedding.NewService(repo)
wedding.Register(e.Group("/dashboard/wedding"), wedding.Deps{Service: svc})
```

Bila modul butuh modul lain, suntikkan **service**-nya lewat `Deps` (bukan repository/pool DB-nya).

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
make tools        # unduh tailwindcss ke ./bin
make dev          # air (templ generate + rebuild) + tailwind --watch → http://localhost:8090
make test         # go test -race ./...
make lint         # gofmt, templ fmt, go vet, golangci-lint
make build        # binary ke ./bin/server
make migrate-new name=create_weddings
make migrate-up / make migrate-down
```

## Definition of Done

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` lolos (`make lint test`)
- [ ] Migration `up` dan `down` jalan bersih
- [ ] Tidak melanggar 9 aturan wajib di atas
- [ ] Halaman baru dicek di viewport 375px
- [ ] README/CHANGELOG/CONTRIBUTING diupdate bila ada konvensi baru
