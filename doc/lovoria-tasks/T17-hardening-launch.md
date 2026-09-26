# T17 — Hardening & Launch Readiness

**Estimasi:** 2–3 hari · **Depends on:** semua task · **Modul:** lintas

## Konteks
Menyiapkan platform untuk 100 wedding aktif di produksi dengan biaya ≈ $16–22/bulan (Arsitektur §8, §10.6).

## Scope (In)
- **Security**: security headers (CSP ketat yang mengizinkan htmx/Alpine lokal & Google Fonts, HSTS, X-Frame-Options kecuali iframe preview tema), review ulang CSRF & rate limit, audit isolasi tenant (test otomatis yang mencoba akses silang untuk tiap route dashboard)
- **Reliability**: backup Postgres terjadwal (Railway backup + dump harian ke R2 bucket privat, retensi 14 hari), dokumentasi restore yang sudah diuji
- **Observability**: log terstruktur dengan request ID & wedding_id, error tracking (Sentry free tier atau setara), uptime check untuk `/healthz` dan satu halaman public contoh
- **Performance**: load test (k6) skenario H-1 acara — 50 RSVP/detik ke beberapa wedding + 200 page view/detik; catat baseline; tambah index yang kurang
- **Cloudflare**: cache rule untuk `/static/*` dan media R2, pastikan halaman dashboard tidak di-cache
- **E2E smoke test** (Playwright): register → wizard → tambah event → upload foto → tambah tamu → publish → buka invitation → RSVP → guestbook
- **Legal/UX minimum**: halaman Privacy Policy & Terms placeholder, halaman 404/500 bergaya
- **Runbook** `docs/runbook.md`: deploy, rollback, restore DB, rotasi secret, apa yang dicek saat sinyal di Arsitektur §10.6 muncul

## Out of Scope
- Scale-out, Redis, read replica (ditunggu sampai ada sinyal nyata).

## Acceptance Criteria
- [ ] Load test: p95 RSVP < 300 ms, p95 halaman public < 500 ms di plan Railway target
- [ ] Restore backup berhasil diuji ke database baru
- [ ] E2E smoke test hijau di CI terhadap staging
- [ ] Tidak ada temuan high dari `govulncheck`
- [ ] Checklist launch di runbook terisi semua

## Catatan untuk Agent
- Jangan tambah infrastruktur baru yang menaikkan biaya bulanan tanpa catatan di PR.
