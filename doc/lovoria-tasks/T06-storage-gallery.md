# T06 — R2 Storage & Gallery

**Estimasi:** 2–3 hari · **Depends on:** T04 · **Modul:** `modules/gallery`, `/src/platform/storage`

## Konteks
Semua foto harus di Cloudflare R2 (Arsitektur §2, §10.5 poin 5). Gallery MVP hanya upload oleh couple/admin (Produk §11).

## Scope (In)
- Package `platform/storage` dengan interface `Storage { Put, Delete, PublicURL }`, implementasi R2 (aws-sdk-go-v2, endpoint R2) + implementasi lokal untuk dev/test (disk di `/tmp`, **hanya dev**)
- Upload pipeline: validasi MIME dari magic bytes (jpeg/png/webp/heic→tolak dulu kalau tidak bisa diproses), max 10 MB, strip EXIF, resize ke `max 2048px` + thumbnail `480px`, encode WebP/JPEG
- Key pattern: `weddings/{wedding_id}/{category}/{uuid}.{ext}`
- Tabel `gallery_items` (`id`, `wedding_id`, `category` enum `cover|couple|prewedding|wedding`, `url`, `thumb_url`, `width`, `height`, `size_bytes`, `sort_order`, `caption`)
- Dashboard gallery: multi-upload dengan progress (htmx/Alpine), grid, hapus, reorder, pilih kategori, set cover
- Komponen templ reusable `ImageUploadField` untuk dipakai T04/T05 (foto couple, main photo, love story)
- Kuota storage per wedding (kolom `storage_used_bytes` di `weddings`, limit dari env default 500 MB), ditolak bila lewat
- Public bucket via custom domain R2 (mis. `media.lovoria.com`) di balik Cloudflare CDN

## Out of Scope
- Guest upload, video (V2).

## Acceptance Criteria
- [ ] Upload 10 foto sekaligus berhasil di mobile
- [ ] Hapus item juga menghapus objek di R2 dan mengurangi `storage_used_bytes`
- [ ] File non-gambar yang di-rename `.jpg` ditolak
- [ ] Tidak ada file tersimpan di filesystem Railway
- [ ] Service expose `ListGallery(weddingID)`, `StorageUsage(weddingID)` (dipakai T09, T16)

## Catatan untuk Agent
- Proses resize di-request (sinkron) cukup untuk MVP; batasi concurrency upload per request.
- Jangan pernah pakai URL R2 presigned yang kedaluwarsa untuk konten public.
