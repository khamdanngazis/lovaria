# T03 — Authentication & Session

**Estimasi:** 2 hari · **Depends on:** T02 · **Modul:** `modules/auth`

## Konteks
Couple dan admin butuh login (Produk §2, §18 "User registration/login"). Guest **tidak** punya akun — akses via invitation code (T09).

## Scope (In)
- Tabel `users` (`id`, `email citext unique`, `password_hash`, `name`, `role` enum `couple|admin`, `email_verified_at`, timestamps)
- Tabel `sessions` (server-side, `id` random 32 byte, `user_id`, `expires_at`, `ip`, `user_agent`)
- Register, login, logout, halaman form (templ + htmx validation inline)
- Password hashing argon2id
- Cookie session: `HttpOnly`, `Secure`, `SameSite=Lax`, rolling expiry 30 hari
- Middleware: `RequireAuth`, `RequireRole(admin)`, `CurrentUser(ctx)`
- CSRF protection untuk semua form POST dashboard (Echo CSRF middleware; htmx kirim header token)
- Forgot/reset password via token sekali pakai (email dikirim lewat interface `Mailer`; implementasi awal: log ke console + adapter SMTP/Resend via env)
- Rate limit login & register per IP (in-memory, cukup untuk single instance)
- CLI `./lovoria create-admin --email --password`

## Out of Scope
- Social login, 2FA, email verification wajib (field disiapkan, flow opsional).

## Acceptance Criteria
- [ ] Register → otomatis login → redirect ke dashboard
- [ ] Route `/dashboard/*` redirect ke login bila belum auth; `/admin/*` 403 untuk role couple
- [ ] Session hilang setelah logout dan tidak bisa dipakai ulang
- [ ] Reset password token kedaluwarsa 1 jam dan tidak bisa dipakai 2 kali
- [ ] Test: hash/verify, session lifecycle, CSRF ditolak tanpa token

## Catatan untuk Agent
- Pesan error login generik ("email atau password salah").
- Jangan simpan session di JWT.
