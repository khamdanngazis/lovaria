# Lifecycle wedding — Dokumentasi Teknis

`Draft → Published → Wedding Day → Memory → Archived` (Produk §16). Kode: `src/modules/wedding/lifecycle.go`, migration `00008_create_wedding_status_history.sql`.

## Transisi

| Dari | Ke | Oleh |
|---|---|---|
| draft | published | pasangan, admin — **checklist wajib lengkap** |
| published | draft | pasangan, admin |
| published | wedding_day | sistem (hari H 00:00 zona waktu wedding) |
| wedding_day | memory | sistem (H+1) |
| memory | archived | sistem (H+1+`LIFECYCLE_ARCHIVE_DAYS`, default 365), admin |
| archived | memory | admin |

`Service.Transition(ctx, weddingID, to, actor)` menjalankan transisi dalam transaksi (`SELECT … FOR UPDATE`), menolak yang tidak ada di tabel dengan `*TransitionError` ("status tidak bisa diubah dari Terbit ke Kenangan" / "… tidak boleh dilakukan oleh user"), dan mencatat setiap perubahan di `wedding_status_history` (`from_status`, `to_status`, `actor` user/system/admin, `actor_user_id`, `at`).

**Checklist publikasi** (`Service.Checklist`): nama kedua mempelai, tanggal, minimal 1 acara. Jumlah acara berasal dari sub-modul event lewat interface `EventCounter` (`SetEventCounter`) — modul wedding tidak membaca tabel `events`. Kurang → `*ChecklistError{Missing}`.

## Scheduler

`Service.RunLifecycle` (goroutine di `lovoria serve`, tiap 10 menit, juga sekali saat start) → `RunLifecycleOnce` mengambil **advisory lock Postgres** (`pg_try_advisory_lock`) di koneksi tersendiri; instance lain yang tidak mendapat lock melewati putaran. `AdvanceDue` menghitung status target per wedding dengan **tanggal lokal di zona waktu wedding** (`dueStatus`) dan menjalankan transisi sistem langkah demi langkah (status yang tertinggal beberapa langkah dikejar sekaligus, tiap langkah tercatat). Jam bisa di-inject (`Service.now`) untuk test.

Setelah pasangan mempublikasikan wedding yang tanggalnya sudah tiba/lewat, `AdvanceNow` langsung memajukan statusnya tanpa menunggu scheduler.

## Guard (dipakai modul lain)

Modul lain **tidak boleh** membandingkan string status — pakai method `Wedding` (ditegakkan `TestStatusGuardsOnly`):

| Guard | Benar untuk |
|---|---|
| `IsPublic()` | semua kecuali draft |
| `AllowsRSVP()` | published, wedding_day (T10) |
| `AllowsGuestbook()` | published, wedding_day, memory (T11) |
| `InMemory()` | memory |
| `IsArchived()` / `IsDraft()` | archived / draft |

## Perilaku halaman publik

| Status | Tamu melihat |
|---|---|
| draft | 404 (pemilik: preview dengan banner) |
| published, wedding_day | undangan penuh |
| memory | undangan + banner "Terima kasih telah menjadi bagian dari hari kami", RSVP ditutup, buku tamu tetap terbuka |
| archived | halaman ringkas "Undangan ini telah diarsipkan" |

## Dashboard

Kartu "Status undangan" di ringkasan wedding: label status, penjelasan, checklist (✓/✗, tautan "Tambah acara"), tombol **Publikasikan** (nonaktif bila checklist belum lengkap) / **Tarik publikasi** dengan konfirmasi, dan riwayat status. Endpoint: `PATCH /dashboard/weddings/:id/status` (`status=published|draft`).
