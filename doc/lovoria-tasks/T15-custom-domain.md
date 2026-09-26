# T15 — Custom Domain (Cloudflare for SaaS)

**Estimasi:** 2–3 hari · **Depends on:** T09, T12 · **Modul:** `modules/domain`

## Konteks
Custom domain per wedding termasuk MVP (Arsitektur §5), memakai Cloudflare for SaaS Custom Hostnames.

## Scope (In)
- Tabel `custom_domains` (`id`, `wedding_id unique`, `domain unique`, `cf_hostname_id`, `status` enum `pending_verification|active|failed|removed`, `verification_errors jsonb`, `verified_at`, `last_checked_at`, `created_at`)
- Client Cloudflare API (token dari env): create custom hostname, get status, delete
- Dashboard: form input domain, instruksi CNAME yang jelas untuk orang awam (host `www` → `domains.lovoria.com`), status badge, tombol "Cek ulang", tombol hapus
- Validasi domain: format valid, bukan subdomain lovoria, belum dipakai wedding lain
- Polling status: scheduler (reuse pola T12) cek domain `pending_verification` tiap 5 menit selama 72 jam lalu `failed`
- Implementasi `DomainLookup` untuk middleware T09 dengan cache in-memory TTL 60 detik (invalidasi saat status berubah)
- Isi `wedding.CanonicalBaseURL()` (dipakai T14 & OG tag)
- Redirect: saat domain `active`, akses `/w/:slug` redirect 301 ke custom domain (link `/i/:code` di domain Lovoria tetap jalan, tidak di-redirect, supaya link yang sudah terkirim aman)
- Dokumentasi setup satu kali di `docs/custom-domain.md`: fallback origin, CNAME target, SSL settings

## Out of Scope
- Jaminan apex domain di semua DNS provider (tangani case-by-case, Arsitektur §5).

## Acceptance Criteria
- [ ] Alur end-to-end teruji dengan satu domain sungguhan di staging
- [ ] Request dengan Host tak dikenal → 404 generik (bukan wedding lain)
- [ ] Menghapus domain juga menghapus custom hostname di Cloudflare
- [ ] Client Cloudflare di-mock pada unit test
- [ ] Peringatan admin/log bila jumlah hostname aktif ≥ 90 (mendekati kuota gratis 100)

## Catatan untuk Agent
- Normalisasi domain: lowercase, hapus trailing dot, tolak skema/path.
