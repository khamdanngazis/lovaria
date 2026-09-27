# T19 — Remember: Halaman Kenangan, Arsip, & Keepsake

**Estimasi:** 2–3 hari · **Depends on:** T06, T11, T12, T16 · **Modul:** `modules/wedding` (lifecycle), `public-site`, `modules/guestbook`, `modules/gallery`, `templates/themes`

## Konteks
Brand positioning menjanjikan website yang **"tidak berakhir ketika acara selesai"** (prinsip *Long-lasting*, tahap **Remember**). Kondisi sekarang (T12):
- **Kenangan** (H+1 s.d. ±1 tahun): undangan tetap tampil + banner terima kasih, RSVP ditutup, ucapan tetap terbuka — sudah sejalan, tetapi tampilannya masih "undangan", belum "kenangan".
- **Diarsipkan** (setelah `archive_days`): website diganti **satu kalimat** "Undangan ini telah diarsipkan" — cerita, galeri, dan ucapan hilang dari publik. **Ini bertentangan** dengan positioning.

Task ini mengubah Kenangan & Arsip menjadi pengalaman "Remember" dan memberi pasangan kenang-kenangan yang bisa disimpan.

## Scope (In)
**1. Mode Kenangan (status `memory`)**
- Tata letak undangan berganti fokus: pembuka "Terima kasih telah menjadi bagian dari hari kami", lalu **foto hari-H** (galeri kategori `wedding`) di posisi atas, **sorotan ucapan** (ucapan terbaru/terpilih), cerita cinta, dan informasi acara diringkas ("Telah dilangsungkan pada …").
- Form RSVP & hitung mundur tidak tampil; ucapan tetap bisa dikirim (perilaku T12).
- Dashboard: saat status Kenangan, beranda menyarankan "Unggah foto hari bahagia" (kategori Pernikahan) dan "Pilih ucapan favorit".

**2. Arsip yang tetap bermakna (status `archived`)**
- Ganti halaman satu kalimat dengan **halaman arsip read-only**: nama pasangan, tanggal, cerita cinta, galeri (hingga batas di bawah), dan ucapan. Semua form (RSVP, ucapan) ditutup; link tamu `/i/KODE` menampilkan halaman arsip yang sama (tanpa data RSVP pribadi).
- Pasangan dapat memilih **visibilitas arsip** di dashboard: *Publik* (default) atau *Privat* (hanya pemilik yang login; publik mendapat halaman ringkas seperti sekarang).
- Perilaku `archive_days` & transisi T12 tidak berubah; hanya tampilan & aksesnya.

**3. Ucapan favorit**
- Pasangan menandai ucapan sebagai **favorit** (bintang) di dashboard Ucapan; ucapan favorit ditampilkan paling atas di mode Kenangan & arsip.

**4. Keepsake (unduhan kenang-kenangan)**
- Dashboard → "Unduh kenang-kenangan": file **PDF** berisi sampul (nama pasangan, tanggal, foto utama), cerita cinta, dan **semua ucapan tamu** (tanpa yang disembunyikan), plus ringkasan RSVP (jumlah hadir).
- Tersedia sejak status Terbit; paling berguna setelah acara. Dibuat on-demand, tidak disimpan permanen.

## Out of Scope
- Foto kiriman tamu, galeri bersama, video/voice guestbook, anniversary reminder (Future Experience).
- Unduhan semua foto dalam ZIP (bisa task terpisah; perhatikan kuota bandwidth R2).
- Mengubah lama arsip default atau paket (sudah ada di T16).

## Detail Teknis
- Guard baru di modul wedding (aturan status hanya lewat guard, `TestStatusGuardsOnly`): `ShowsMemoryLayout()`, `ArchivePublic()` (butuh kolom `weddings.archive_visibility text NOT NULL DEFAULT 'public' CHECK (IN ('public','private'))`).
- `public-site`: `Handler.archived` merender `theme` versi arsip (read-only) bila `ArchivePublic()`, selain itu halaman ringkas lama. `View` mendapat `Archived bool`, `Favorites []GuestbookEntry`; tema menampilkan bagian sesuai mode — logika tampilan tetap lewat theme registry.
- Guestbook: kolom `guestbook_entries.is_favorite boolean NOT NULL DEFAULT false`; `Service.SetFavorite`, `Service.Favorites(ctx, weddingID, n)`; route `PATCH /dashboard/weddings/:id/guestbook/:entryID/favorite`.
- Gallery: `Service.ByCategory(ctx, weddingID, category, limit)` untuk foto hari-H.
- Keepsake PDF: generator Go murni (mis. `github.com/go-pdf/fpdf` atau `github.com/johnfercher/maroto`), font yang mendukung karakter Indonesia & emoji dasar (emoji boleh diganti/dihilangkan bila font tidak mendukung — dokumentasikan). Route `GET /dashboard/weddings/:id/keepsake.pdf`, owner-only, `Cache-Control: private, no-store`. Foto diambil dari URL R2 (thumbnail) dengan batas ukuran & timeout.
- Cache publik: `BuildPublic` (T17) — pastikan kunci cache ikut berubah saat favorit / visibilitas berubah (invalidate).

## Acceptance Criteria
- [ ] Wedding berstatus Kenangan menampilkan tata letak kenangan (foto hari-H, ucapan favorit di atas) di semua tema; RSVP tidak tampil
- [ ] Wedding Diarsipkan + visibilitas Publik menampilkan cerita, galeri, dan ucapan read-only; tidak ada form yang bisa dikirim (POST → 403)
- [ ] Visibilitas Privat: publik mendapat halaman ringkas, pemilik yang login melihat arsip lengkap
- [ ] Favorit: tandai/lepas di dashboard, urutan di halaman publik ikut berubah dalam ≤ 1 request (cache di-invalidate)
- [ ] Keepsake PDF terunduh untuk wedding dengan 500 ucapan dalam < 5 detik, isi sesuai, ucapan tersembunyi tidak ikut
- [ ] Isolasi tenant: route favorit & keepsake 404 untuk user lain (otomatis tercakup `TestTenantIsolationAllDashboardRoutes`)
- [ ] Dicek di 375px untuk ketiga mode (Terbit, Kenangan, Arsip)

## Catatan untuk Agent
- Jangan membuat pengecekan status langsung (`w.Status == "memory"`) di luar modul wedding — tambah guard.
- Halaman arsip publik tetap `noindex`.
- "Forever" adalah positioning emosional (brand-positioning §5): jangan menulis janji penyimpanan selamanya di UI; sebut lama arsip sesuai paket.
