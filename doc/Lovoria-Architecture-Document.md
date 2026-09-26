# Lovoria — Software Architecture Document

**Versi:** 1.0 (MVP)
**Target:** 100 wedding aktif
**Prinsip:** Lean, budget minim, tidak overkill, jalur upgrade jelas

---

## 1. Ringkasan Keputusan

| Aspek | Keputusan |
|---|---|
| Pola arsitektur | Modular monolith, single deployable |
| Bahasa backend | Go |
| Rendering | Server-Side Rendering (SSR) |
| Interaktivitas frontend | htmx + Alpine.js |
| Styling | Tailwind CSS |
| Database | PostgreSQL |
| Storage foto/gallery | Cloudflare R2 |
| Hosting | Railway (Hobby plan) |
| CDN/DNS | Cloudflare (gratis) |
| Custom domain per wedding | Cloudflare for SaaS (Custom Hostnames) — **termasuk di MVP** |
| Estimasi biaya bulanan | ≈ $16–22/bulan |

---

## 2. High-Level Architecture

```text
┌───────────────────────────────────────────┐
│              Railway (Hobby plan)           │
│                                              │
│   ┌──────────────────────────────────────┐  │
│   │         Go binary (1 service)          │  │
│   │  - Fiber/Echo (router + API)           │  │
│   │  - templ / html-template (SSR page)    │  │
│   │  - htmx + Alpine.js (interaktivitas)   │  │
│   │  - Tailwind CSS                        │  │
│   └──────────────┬───────────────────────┘  │
│                  │                           │
│   ┌──────────────▼───────────────────────┐  │
│   │           PostgreSQL                    │  │
│   └───────────────────────────────────────┘  │
└──────────────────┬──────────────────────────┘
                    │
         ┌──────────▼───────────┐
         │   Cloudflare R2        │  ← foto/gallery
         │   (storage + CDN,      │
         │    egress gratis)      │
         └─────────────────────────┘
                    │
         ┌──────────▼───────────┐
         │   Cloudflare (DNS,     │
         │   SSL, CDN depan app)  │
         │   + Cloudflare for     │
         │   SaaS (Custom         │
         │   Hostnames)           │
         └─────────────────────────┘
                    ▲
        ┌───────────┴────────────┐
        │  Domain milik couple     │
        │  (mis. khamdansarah.com)│
        │  → CNAME ke Lovoria     │
        └──────────────────────────┘
```

Semua fitur (couple dashboard, public wedding website, admin panel) berjalan sebagai **satu codebase, satu proses deploy**. Tidak ada microservices, message queue, atau container orchestration di tahap ini.

**Custom domain per wedding** ditangani lewat Cloudflare for SaaS: couple arahkan domain mereka (CNAME) ke Lovoria, Cloudflare otomatis provision sertifikat TLS untuk domain tersebut, lalu meneruskan trafiknya ke aplikasi Go yang sama. Tidak perlu server/proxy tambahan, tidak perlu urus sertifikat SSL manual per domain.

---

## 3. Struktur Aplikasi (Modular Monolith)

```text
/src
  /modules
    auth/          → register, login couple & admin
    wedding/       → wedding, couple info, event, love story
    guest/         → guest list, invitation code, RSVP
    gallery/       → upload & serve foto (ke R2)
    guestbook/     → ucapan tamu
    gift/          → info rekening/e-wallet
    theme/         → theme registry, token warna/font
    domain/        → custom domain: verifikasi, status, lookup ke wedding
    admin/         → panel internal
  /templates
    /themes
      romantic/    → layout.templ, hero.templ, gallery.templ, love-story.templ
      minimal/
      elegant/
      modern/
    /shared
      rsvp-form.templ     ← komponen sama dipakai semua tema
      guestbook.templ
  /public-site     → routing & rendering wedding website (mobile-first)
  /dashboard       → UI couple
```

**Aturan modular yang wajib dijaga:**
1. Modul tidak boleh query langsung ke tabel modul lain — akses lewat service/fungsi milik modul tersebut.
2. Setiap tabel wajib punya `wedding_id` untuk isolasi multi-tenant, tanpa terkecuali.
3. Semua keputusan tema (layout mana untuk tema apa) hanya lewat satu **theme registry**, tidak tersebar di banyak tempat.
4. Endpoint didesain "API-shaped" sejak awal (nama route & payload masuk akal walau saat ini balikin HTML fragment), supaya gampang diekspos jadi JSON API di masa depan.
5. Resolusi wedding dari request dilakukan lewat satu middleware terpusat yang mengecek **Host header lebih dulu** (untuk custom domain), baru fallback ke path slug (`lovoria.com/i/kode`) — supaya logic ini tidak terduplikasi di tiap handler.

---

## 4. Data Model (Ringkasan)

Multi-tenancy: **shared database**, dibedakan lewat `wedding_id` di setiap tabel relevan — bukan database terpisah per tenant.

```text
weddings          (id, couple_id, slug, status, theme_id, ...)
couples           (id, groom_name, bride_name, ...)
events            (id, wedding_id, name, date, venue, ...)
love_stories      (id, wedding_id, date, title, photo_url)
guests            (id, wedding_id, name, invitation_code, rsvp_status, ...)
guestbook_entries (id, wedding_id, guest_name, message, created_at)
gallery_items     (id, wedding_id, url, type)
custom_domains    (id, wedding_id, domain, status, verified_at, created_at)
```

### Status Custom Domain

```text
pending_verification → active → (failed / removed)
```

`status` dicek oleh Cloudflare for SaaS API setelah couple memasukkan domain mereka dan mengarahkan CNAME. Setelah `active`, middleware routing di §3 memakai tabel ini untuk mencocokkan Host header ke `wedding_id`.

### Wedding Lifecycle

```text
Draft → Published → Wedding Day → Memory → Archived
```

---

## 5. Custom Domain per Wedding (MVP)

### Alur bagi couple

```text
Couple masukkan domain (mis. khamdansarah.com) di dashboard
       ↓
Lovoria daftarkan domain ke Cloudflare for SaaS (Custom Hostnames)
       ↓
Couple diberi instruksi CNAME (arahkan domain ke Lovoria)
       ↓
Cloudflare verifikasi + otomatis provision TLS certificate
       ↓
Status domain: pending_verification → active
       ↓
Domain langsung bisa diakses, trafik diteruskan ke aplikasi Go yang sama
```

### Kenapa ini tetap murah dan tidak menambah kompleksitas besar

- Cloudflare for SaaS menyediakan **100 custom hostname gratis** di plan Free/Pro/Business, baru dikenakan biaya tambahan kecil per hostname di atas kuota tersebut. Untuk target 100 wedding, ini **praktis $0 tambahan**.
- Sertifikat TLS di-provision otomatis oleh Cloudflare per domain — tidak perlu setup Let's Encrypt manual atau reverse proxy tambahan.
- Aplikasi Go tidak perlu tahu domain mana yang dipakai; cukup satu middleware yang mencocokkan Host header ke `wedding_id` lewat tabel `custom_domains` (lihat §4), lalu proses render berjalan sama seperti biasa.
- Link personalisasi tamu (`/i/ABCD123`) tetap jalan di atas custom domain (`khamdansarah.com/i/ABCD123`), tidak perlu redesign skema slug yang sudah ada.

### Yang perlu disiapkan di UI dashboard

- Form input domain + instruksi CNAME yang jelas untuk couple (kebanyakan bukan orang teknis).
- Indikator status verifikasi (pending/active/gagal) supaya couple tahu domainnya sudah aktif atau belum.
- Fallback: kalau couple tidak setup custom domain, wedding tetap bisa diakses lewat slug bawaan Lovoria (`lovoria.com/i/kode`) tanpa dependency ke domain eksternal.

### Batasan yang perlu disepakati untuk MVP

- Dukungan **subdomain/CNAME standar** (`www.khamdansarah.com` atau domain penuh yang di-CNAME-kan) — ini yang didukung penuh di plan Cloudflare biasa.
- Dukungan **apex/root domain** tanpa `www` untuk sebagian kasus DNS provider tertentu bisa lebih rumit tergantung dukungan CNAME flattening dari provider domain couple — perlu dicek saat implementasi, bukan diasumsikan selalu mulus di semua provider domain.

---

## 6. Sistem Tema/Template

Tema terdiri dari 2 lapis kustomisasi:

**Level 1 — Token visual (ringan):** warna, font, background — via CSS custom properties, tanpa file terpisah.

```css
:root[data-theme="romantic"] {
  --color-primary: #B76E79;
  --font-heading: 'Playfair Display', serif;
}
:root[data-theme="minimal"] {
  --color-primary: #2C2C2C;
  --font-heading: 'Inter', sans-serif;
}
```

**Level 2 — Struktur layout (berbeda per tema):** hero section, susunan gallery, gaya love story — file `.templ` terpisah per tema di `/templates/themes/`.

Komponen fungsional yang sama di semua tema (form RSVP, guestbook, digital gift) dibuat sekali di `/templates/shared/` dan dipakai lintas tema.

Menambah tema baru = tambah folder di `/templates/themes/` + daftarkan di theme registry. Tidak mengubah routing, database, atau proses deploy.

**Untuk tema premium dengan animasi/interaktivitas kompleks**, lihat §7 (Islands Architecture) — pendekatan ini yang dipakai, bukan konversi seluruh halaman jadi SPA.

---

## 7. Strategi Animasi & Tema Premium (Islands Architecture)

### Latar belakang keputusan

Kekhawatiran awal: kalau nanti ada tema premium dengan animasi kompleks (parallax, particle effect, gallery interaktif dengan state rumit), apakah arsitektur SSR+htmx masih cukup, atau harus pindah ke SPA penuh?

**Keputusan: tetap SSR+htmx sebagai default, animasi kompleks masuk lewat "islands"** — bukan konversi seluruh platform jadi SPA. Alasan penolakan opsi full SPA (Go API + Svelte/Preact di Cloudflare Pages) dijelaskan di bawah.

### Kenapa full SPA ditolak untuk kasus Lovoria

| Masalah | Penjelasan |
|---|---|
| Meta tag OG dinamis untuk WhatsApp preview | Fitur "Invitation Sharing via WhatsApp" (lihat dokumen produk) butuh preview link personal per wedding (nama pasangan, foto cover). WhatsApp crawler tidak menjalankan JavaScript, jadi SPA murni tidak bisa menyajikan OG tag dinamis tanpa trik tambahan (edge function/prerendering) |
| Kompleksitas tidak sepadan | Fitur RSVP, guestbook, digital gift — mayoritas platform — adalah CRUD/form sederhana yang tidak butuh state management SPA |
| CORS & split deployment | Memisah frontend (Cloudflare Pages) dan backend (Railway) menambah permukaan yang perlu dikelola (CORS, dua tempat deploy) tanpa manfaat yang sepadan di skala ini |

### Pola yang dipakai: Islands

```text
┌─────────────────────────────────────────┐
│   Halaman wedding (SSR dari Go, tetap)   │
│                                           │
│  <section class="hero">...</section>     │
│  <section class="love-story">...</section>│
│                                           │
│  ┌─────────────────────────────────┐    │
│  │  <div id="premium-gallery-fx">   │    │ ← "island": bundle JS
│  │  (komponen Svelte/library        │    │   kecil, self-contained,
│  │   animasi, di-compile jadi       │    │   cuma aktif di section
│  │   1 file JS statis)              │    │   ini
│  └─────────────────────────────────┘    │
│                                           │
│  <section class="rsvp">...</section>     │
└─────────────────────────────────────────┘
```

### Aturan implementasi

1. Halaman inti (hero, love story, event detail, RSVP, guestbook) **tetap SSR + htmx/Alpine** — meta tag OG selalu benar karena di-render server-side per wedding.
2. Animasi ringan (fade-in scroll, parallax, countdown, gallery lightbox) cukup **CSS + vanilla JS/library kecil** (Intersection Observer, GSAP/Motion One) — tidak butuh framework, jalan di tema manapun.
3. Untuk tema premium yang benar-benar butuh state kompleks (live customizer, efek reaktif berat), komponen tersebut ditulis di **Svelte**, di-compile jadi satu file JS statis saat build, lalu di-mount ke elemen spesifik di halaman via `<script>` tag — bukan mengambil alih seluruh halaman.
4. File JS hasil compile disimpan sebagai aset statis (bisa lewat R2/Cloudflare CDN yang sudah ada), **tidak butuh service/hosting tambahan**.
5. Theme registry (§6) menentukan tema mana yang memuat island tertentu — logic ini tetap terpusat, konsisten dengan aturan modular di §3.

### Dampak ke biaya dan kompleksitas

Tidak ada biaya infrastruktur tambahan — semua tetap berjalan di stack yang sudah ada (Railway + R2 + Cloudflare). Kompleksitas dev tambahan cuma muncul saat benar-benar ada tema yang butuh island, terisolasi per komponen, tidak menyentuh bagian platform yang lain.

---

## 8. Estimasi Biaya Bulanan (MVP)

| Komponen | Layanan | Estimasi |
|---|---|---|
| App (Go) + Database | Railway Hobby plan | $15–20 |
| Storage foto/gallery | Cloudflare R2 | $0–1 |
| CDN/DNS | Cloudflare | $0 |
| Custom domain per wedding (hingga 100 domain) | Cloudflare for SaaS | $0 (termasuk di kuota gratis) |
| Domain utama Lovoria sendiri | — | ~$1/bulan (dari $10–15/tahun) |
| **Total** | | **≈ $16–22/bulan** |

> Catatan: biaya di atas untuk domain **milik Lovoria sendiri** (mis. `lovoria.com`). Domain custom per wedding (mis. `khamdansarah.com`) dibeli dan dibayar sendiri oleh masing-masing couple di registrar pilihan mereka — Lovoria hanya menyediakan mekanisme koneksi/verifikasinya.

---

## 9. Yang Sengaja Tidak Dibangun di MVP

- Microservices / event-driven architecture
- Message queue (RabbitMQ/Kafka)
- Kubernetes / container orchestration
- **Full SPA (frontend terpisah 100%, mis. Cloudflare Pages + API JSON)** — ditolak untuk arsitektur inti, lihat §7 untuk alasan dan alternatif (Islands)
- S3 + CloudFront (kompleksitas setup tidak sepadan di skala ini)
- Payment gateway sendiri
- Drag & drop page builder
- Redis/caching layer
- Dukungan DNS provider eksotis/edge-case untuk apex domain (ditangani case-by-case, bukan dijamin universal di MVP)

---

## 10. Rencana & Catatan Upgrade (Roadmap Teknis)

### 10.1 Prinsip Umum

Arsitektur ini didesain supaya **99% pertumbuhan bisa ditangani dengan menambah**, bukan merombak. Upgrade dilakukan bertahap, dipicu oleh sinyal nyata (bottleneck terukur), bukan diantisipasi berlebihan di awal.

### 10.2 Jalur Upgrade Berdasarkan Skala Traffic

| Tahap | Trigger | Tindakan |
|---|---|---|
| 100 → 500 wedding | Resource Railway mulai mepet | Scale vertikal (naikkan RAM/CPU), tetap 1 service |
| 500 → 2.000 wedding | Query database mulai lambat | Tambah index, connection pooling (pgbouncer), pertimbangkan read replica |
| Traffic gallery naik | — | Tidak perlu tindakan, R2 sudah linear cost tanpa egress fee |
| >10.000 wedding / traffic viral | Satu service Go mulai kepepet menangani load RSVP bersamaan | Ekstrak modul RSVP jadi service terpisah (baru di titik ini worth dipisah) |

### 10.3 Jalur Upgrade Berdasarkan Fitur (dari daftar V2 dokumen produk)

> Catatan: custom domain per wedding sudah dipindah masuk **scope MVP** (lihat §5), bukan lagi item V2.

| Fitur V2 | Dampak ke arsitektur | Catatan |
|---|---|---|
| Guest photo upload | Rendah | Tambah endpoint upload ke R2, tidak ada perubahan struktural |
| QR check-in | Rendah | Tambah tabel + endpoint scan |
| Video/voice guestbook | Menengah | R2 tetap bisa handle storage; kalau butuh transcode video, tambahkan worker terpisah saat dibutuhkan |
| AI wedding assistant | Netral | Panggil API eksternal (LLM) dari Go, tidak mengganggu arsitektur inti |
| **Drag & drop page builder** | ⚠️ **Paling besar dampaknya** | Kemungkinan besar butuh pindah dari SSR file-per-tema ke sistem block-based (lebih dekat arsitektur SPA/component). Ini satu-satunya item yang berpotensi memicu refactor signifikan. Sudah eksplisit di luar scope MVP — evaluasi ulang saat benar-benar masuk roadmap dekat |

### 10.4 Jalur Upgrade Interaktivitas Frontend (Islands, bukan Full SPA)

**Keputusan final:** untuk kebutuhan animasi/interaktivitas kompleks (tema premium), jalur upgrade yang dipakai adalah **Islands Architecture** (lihat §7 untuk detail lengkap), bukan migrasi ke full SPA.

```text
Sekarang:  Go (SSR + htmx + Alpine)
     ↓ (kalau ada tema premium butuh animasi kompleks)
Nanti:     Go (SSR + htmx + Alpine) tetap jadi shell utama,
           + komponen Svelte di-compile jadi island JS statis,
           di-mount ke elemen spesifik per tema yang membutuhkan
```

**Kenapa opsi full SPA (Go API + SPA terpisah di Cloudflare Pages) tidak dipilih** meski secara biaya infrastruktur juga $0: full SPA mengorbankan meta tag OG dinamis per wedding yang dibutuhkan untuk WhatsApp link preview (lihat §7), dan menambah kompleksitas split deployment (CORS, dua tempat deploy) tanpa manfaat yang sepadan untuk mayoritas fitur Lovoria yang sifatnya CRUD/form.

**Catatan penting:** kalau suatu saat *benar-benar* ada kebutuhan yang memaksa full SPA (misalnya drag & drop page builder di §10.3, yang memang sudah diprediksi butuh arsitektur berbeda), keputusan ini perlu dievaluasi ulang secara khusus — bukan dari kekhawatiran soal animasi saja, yang sudah tertangani lewat Islands.

### 10.5 Titik Rawan yang Harus Dijaga Disiplinnya Sejak Sekarang

1. Tidak ada query lintas modul secara langsung — selalu lewat service layer.
2. `wedding_id` konsisten di semua tabel tanpa terkecuali.
3. Logic tema terpusat di satu theme registry.
4. Endpoint didesain masuk akal seandainya harus balikin JSON, bukan cuma HTML fragment.
5. Storage foto selalu lewat R2 (atau storage setara zero-egress), jangan pernah taruh di volume Railway.
6. Resolusi wedding dari Host header (custom domain) dan dari path slug harus lewat satu middleware yang sama — jangan ada handler yang mengasumsikan salah satu cara akses secara hardcode.
7. Island (komponen Svelte/animasi kompleks) harus self-contained dan di-mount ke elemen spesifik — jangan sampai island mengambil alih routing/rendering halaman yang seharusnya tetap SSR.

### 10.6 Sinyal Kapan Harus Evaluasi Ulang Arsitektur

- Traffic RSVP bersamaan mulai menyebabkan latency terukur pada jam sibuk (H-1 acara banyak wedding).
- Query database > beberapa ratus ms pada endpoint utama meski sudah di-index.
- Tim development bertambah signifikan dan modular monolith mulai terasa menghambat kerja paralel.
- Drag & drop page builder resmi masuk roadmap dekat (3–6 bulan ke depan).
- Jumlah custom domain aktif mendekati/melebihi 100 (kuota gratis Cloudflare for SaaS) — saat itu tinggal evaluasi biaya tambahan per hostname (nominal kecil), bukan ubah arsitektur.

Selama sinyal-sinyal di atas belum muncul, arsitektur ini **tidak perlu diubah** — cukup ditambah sesuai kebutuhan bertahap.

---

## 11. Ringkasan Prinsip Desain

> Setiap keputusan arsitektur di dokumen ini dipilih berdasarkan: (1) cukup untuk skala 100 wedding, (2) biaya minim, (3) tidak menutup jalur pertumbuhan. Kompleksitas ditambahkan hanya saat ada bukti kebutuhan nyata, bukan diasumsikan di depan.
