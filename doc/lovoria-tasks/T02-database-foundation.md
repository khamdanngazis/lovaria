# T02 — Database Foundation & Migrations

**Estimasi:** 1–2 hari · **Depends on:** T01 · **Modul:** `/src/platform/db`, `/migrations`

## Konteks
Shared database multi-tenant dengan isolasi via `wedding_id` (Arsitektur §4). Task ini menyiapkan tooling dan konvensi, bukan seluruh tabel fitur — tabel fitur dibuat oleh task masing-masing.

## Scope (In)
- Koneksi `pgxpool` dengan config pool dari env, dipakai `/readyz`
- `goose` untuk migration SQL, embed ke binary, perintah `migrate up/down/status` via subcommand binary (`./lovoria migrate up`)
- `sqlc` config: satu package query per modul (`modules/<m>/db`)
- Konvensi tabel: `id UUID` (uuidv7 dari Go), `created_at`, `updated_at` (trigger), soft delete **tidak** dipakai kecuali disebut task
- Migration awal: extension (`citext`, `pgcrypto`), fungsi trigger `set_updated_at()`
- Helper transaksi `db.WithTx(ctx, fn)`
- Test harness: integration test memakai Postgres (testcontainers atau `DATABASE_URL_TEST`), reset schema per test package
- Seed script dev (`./lovoria seed`) — kerangka saja, diisi task lain

## Out of Scope
- Tabel fitur (users, weddings, guests, dst).

## Detail Teknis
- Aturan tenant: tiap query sqlc untuk tabel tenant wajib punya parameter `wedding_id`. Tambahkan lint sederhana (script `make lint-tenant`) yang grep file `.sql` query untuk tabel ber-`wedding_id` tanpa `wedding_id` di WHERE.
- Index default: setiap FK `wedding_id` diindex.

## Acceptance Criteria
- [ ] `./lovoria migrate up` jalan di Railway saat deploy (pre-deploy command)
- [ ] `/readyz` → 503 bila DB mati, 200 bila sehat
- [ ] Integration test contoh lolos di CI
- [ ] Dokumentasi konvensi schema di `docs/database.md`

## Catatan untuk Agent
- Jangan pakai ORM. sqlc + SQL eksplisit.
- Migration harus reversible; `down` wajib diisi.
