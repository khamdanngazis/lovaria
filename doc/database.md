# Konvensi Database Lovoria

PostgreSQL 16 · driver `pgx/v5` · query `sqlc` · migration `goose` (SQL, di-embed ke binary).
Tidak memakai ORM — semua query adalah SQL eksplisit.

## Multi-tenancy

Shared database, isolasi per wedding lewat kolom `wedding_id` (Arsitektur §4).

- **Setiap tabel milik wedding wajib punya `wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE`.**
- **Setiap `wedding_id` wajib ter-index** — lewat `CREATE INDEX ... (wedding_id)` atau `UNIQUE`/`PRIMARY KEY` yang *diawali* `wedding_id`.
- **Setiap query sqlc ke tabel tenant wajib memfilter `wedding_id` di `WHERE`** (INSERT wajib mengisi `wedding_id`). Parameter `wedding_id` datang dari konteks request yang sudah diotorisasi, bukan dari input bebas user.
- Pengecualian yang disengaja (mis. lookup publik by kode undangan yang unik global) wajib diberi komentar di blok query:

  ```sql
  -- name: GetGuestByInvitationCode :one
  -- tenant:ignore kode undangan unik global; wedding di-resolve dari hasilnya
  SELECT * FROM guests WHERE invitation_code = $1;
  ```

Kedua aturan di atas dicek otomatis oleh `make lint-tenant` (jalan di CI).

## Konvensi Tabel

```sql
CREATE TABLE guests (
    id         uuid PRIMARY KEY,                -- UUIDv7, dibuat di Go: db.NewID()
    wedding_id uuid NOT NULL REFERENCES weddings (id) ON DELETE CASCADE,
    name       text NOT NULL,
    email      citext,                          -- citext untuk email/slug/domain (case-insensitive)
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX guests_wedding_id_idx ON guests (wedding_id);
CREATE TRIGGER set_updated_at BEFORE UPDATE ON guests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

| Aturan | Keterangan |
|---|---|
| Primary key | `id uuid`, **tanpa default di DB**. Selalu `db.NewID()` (UUIDv7, terurut waktu → index B-tree tetap rapat). |
| Waktu | `timestamptz` (bukan `timestamp`). `created_at`/`updated_at` `NOT NULL DEFAULT now()`. |
| `updated_at` | Diisi trigger `set_updated_at()` (dibuat di migration `00001_init.sql`). Jangan set manual dari Go. |
| Soft delete | **Tidak dipakai**, kecuali task secara eksplisit memintanya. |
| Teks case-insensitive | `citext` (email, slug, domain). |
| Enum/status | `text` + `CHECK (status IN (...))` — lebih mudah diubah daripada tipe `ENUM`. |
| Nama | `snake_case`, tabel jamak (`guests`), index `<tabel>_<kolom>_idx`, unique `<tabel>_<kolom>_key`. |
| FK | Selalu eksplisit dengan `ON DELETE` yang disengaja. |
| Extension | `citext`, `pgcrypto` (di schema `public`). |

## Migration

- File di `migrations/NNNNN_nama.sql`, nomor urut 5 digit. Buat dengan `make migrate-new name=create_guests`.
- **Setiap migration wajib reversible**: bagian `-- +goose Down` wajib diisi dan membalik `Up` sepenuhnya. Integration test menjalankan down sampai versi 0 lalu up lagi.
- Fungsi/trigger PL/pgSQL dibungkus `-- +goose StatementBegin` / `-- +goose StatementEnd`.
- Satu migration = satu perubahan logis. Migration yang sudah di-merge ke `main` **tidak boleh diedit** — buat migration baru.
- Tabel fitur dibuat oleh task pemilik modulnya.

Perintah (binary yang sama dengan server):

```bash
lovoria migrate up        # jalankan semua migration baru
lovoria migrate down      # rollback 1 versi
lovoria migrate redo      # down + up versi terakhir
lovoria migrate status
lovoria migrate version
```

Lokal: `make migrate-up`, `make migrate-down`, `make migrate-status`.
Railway: `lovoria migrate up` dijalankan sebagai **pre-deploy command** (`railway.toml`); bila gagal, deploy dibatalkan dan versi lama tetap jalan.

## sqlc

Satu package query per modul:

```text
src/modules/<m>/db/queries/*.sql   ← query (ditulis tangan)
src/modules/<m>/db/*.go            ← hasil generate, package <m>db (mis. guestdb), di-commit
```

Daftarkan modul di `sqlc.yaml` (salin template entry di file itu), lalu `make sqlc`. Schema dibaca langsung dari `migrations/`. Override tipe global: `uuid` → `uuid.UUID`, `timestamptz` → `time.Time`, `citext` → `string`; kolom nullable → pointer.

Hanya `repository.go` modul pemilik yang boleh mengimpor package `<m>db`. Modul lain memanggil service-nya (aturan wajib #1).

## Transaksi

```go
err := db.WithTx(ctx, pool, func(tx pgx.Tx) error {
    q := guestdb.New(tx)
    ...
    return nil // commit; error atau panic → rollback
})
```

`WithTx` menerima `*pgxpool.Pool` atau `pgx.Tx` (nested → savepoint).

## Integration Test

```go
func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

func TestCreateGuest(t *testing.T) {
    pool := dbtest.Pool(t)   // skip bila DATABASE_URL_TEST kosong
    dbtest.Reset(t, pool)    // TRUNCATE semua tabel
    ...
}
```

- Setiap test package mendapat **database baru** (`lovoria_test_<acak>`) yang sudah di-migrate, di-drop setelah selesai — package bisa jalan paralel tanpa saling ganggu.
- Lokal: `make test-integration` (menyalakan Postgres docker di port 5433).
- CI: service Postgres + `LOVORIA_REQUIRE_DB_TESTS=true` → test DB gagal (bukan skip) bila DB tidak tersedia.

## Seed

`lovoria seed` (atau `make seed`) menjalankan seeder yang didaftarkan di `seeders()` pada `cmd/server/main.go`. Seeder memanggil service modul, harus idempoten, dan ditolak di `APP_ENV=production`.
