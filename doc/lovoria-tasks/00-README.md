# Lunovia — Task Breakdown untuk Agent

Sumber: *Product Scope & Feature Overview* + *Software Architecture Document v1.0 (MVP)*.
Setiap task ukurannya sedang (±1–3 hari kerja agent), punya scope jelas, dan bisa di-review sebagai 1 PR.

## Tech Stack (dikunci, jangan diganti agent)

| Area | Pilihan |
|---|---|
| Bahasa | Go 1.23+ |
| Router | Echo v4 |
| Templating | templ |
| Interaktivitas | htmx 2 + Alpine.js 3 |
| Styling | Tailwind CSS (standalone CLI, tanpa Node di runtime) |
| DB | PostgreSQL 16, driver `pgx/v5`, query via `sqlc` |
| Migration | `goose` (SQL migrations) |
| Storage | Cloudflare R2 (S3-compatible, `aws-sdk-go-v2`) |
| Hosting | Railway (1 service + Postgres) |
| CDN/DNS/Custom domain | Cloudflare + Cloudflare for SaaS |

## Aturan Wajib untuk Semua Task (dari Arsitektur §3 & §10.5)

1. **Tidak ada query lintas modul.** Modul A memanggil service modul B, bukan tabel B.
2. **Setiap tabel tenant wajib punya `wedding_id`** dan setiap query tenant wajib memfilter `wedding_id`.
3. **Logic tema hanya lewat theme registry** (`modules/theme`).
4. **Endpoint "API-shaped"**: route & payload masuk akal walau saat ini balikin HTML fragment.
5. **Foto selalu ke R2**, tidak pernah ke disk/volume Railway.
6. **Resolusi wedding hanya lewat 1 middleware** (Host header dulu → fallback slug/kode).
7. Island (Svelte/animasi berat) di luar MVP; jangan ambil alih routing SSR.
8. Mobile-first untuk semua halaman public.
9. Setiap task menyertakan test (unit untuk service, handler test untuk endpoint penting) dan migration yang reversible.

## Daftar Task

| # | Task | Depends on |
|---|---|---|
| T01 | Project bootstrap & deploy pipeline | — |
| T02 | Database foundation & migrations | T01 |
| T03 | Authentication & session | T02 |
| T04 | Wedding core & setup wizard | T03 |
| T05 | Events & love story | T04 |
| T06 | R2 storage & gallery | T04 |
| T07 | Guest management | T04 |
| T08 | Theme system & registry | T04 |
| T09 | Public site: resolver, rendering, personalized invitation | T05, T06, T07, T08 |
| T10 | RSVP | T07, T09 |
| T11 | Guestbook & digital gift | T09 |
| T12 | Publish & wedding lifecycle | T09 |
| T13 | Dashboard home & RSVP summary | T10, T11, T12 |
| T14 | Invitation sharing & custom slug | T07, T12 |
| T15 | Custom domain (Cloudflare for SaaS) | T09, T12 |
| T16 | Admin panel | T12 |
| T17 | Hardening & launch readiness | semua |

### Pasca-MVP (dari `doc/brand-positioning.md`)

| # | Task | Depends on |
|---|---|---|
| T18 | Landing page & brand | T08, T09, T16, T17 |
| T19 | Remember: halaman kenangan, arsip, & keepsake | T06, T11, T12, T16 |
| T20 | Personalisasi undangan: musik, hitung mundur, kutipan, susunan bagian | T06, T08, T09 |
| T21 | Redesign template undangan: lebih menjual & selaras brand | T08, T09, T18, T20 |
| T22 | Redesign dashboard, login & register: selaras warna brand | T03, T13, T16, T18, T21 |

## Urutan & Paralelisasi

```text
T01 → T02 → T03 → T04
                    ├── T05 ┐
                    ├── T06 ├──→ T09 ──┬── T10 ┐
                    ├── T07 ┤          ├── T11 ├──→ T13
                    └── T08 ┘          └── T12 ┘
                                         ├── T14
                                         ├── T15
                                         └── T16
                                                  → T17
```

Setelah T04 selesai, T05–T08 bisa dikerjakan paralel oleh agent berbeda. Setelah T09, T10/T11/T12 bisa paralel.

## Format Setiap File Task

- **Konteks** — kenapa task ini ada
- **Scope (In)** / **Out of Scope**
- **Detail Teknis** — tabel, route, file yang disentuh
- **Acceptance Criteria** — checklist yang harus lolos sebelum PR di-merge
- **Catatan untuk Agent** — jebakan yang perlu dihindari

## Definition of Done (berlaku untuk semua task)

- [ ] `go build ./...`, `go vet ./...`, `go test ./...` lolos
- [ ] Migration `up` dan `down` jalan bersih
- [ ] Tidak melanggar 9 aturan wajib di atas
- [ ] Halaman baru dicek di viewport 375px
- [ ] README/CHANGELOG modul diupdate bila ada konvensi baru
