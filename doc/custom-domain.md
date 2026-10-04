# Custom domain — setup satu kali (Cloudflare for SaaS)

Pasangan bisa memakai domain sendiri (`www.samuelsarah.com`) untuk undangannya. Lunovia mendaftarkan domain itu sebagai **custom hostname** di Cloudflare for SaaS; Cloudflare menerbitkan sertifikat TLS otomatis dan meneruskan trafik ke aplikasi. Kode: `src/modules/domain`, dokumentasi modul di bawah.

## 1. Prasyarat

- Domain Lunovia sendiri (mis. `lovoria.com`) sudah memakai DNS Cloudflare (zona aktif).
- Cloudflare for SaaS aktif di zona itu: **SSL/TLS → Custom Hostnames → Enable** (gratis sampai 100 hostname).

## 2. Fallback origin & target CNAME

1. Buat record DNS **proxied** (awan oranye) untuk fallback origin, mis. `origin.lovoria.com`. Dengan Worker (opsi A di bawah) cukup record *originless* `AAAA origin 100::` — Worker yang meneruskan ke Railway. Tanpa Worker (opsi B): CNAME ke server aplikasi.
2. **SSL/TLS → Custom Hostnames → Fallback Origin** = `origin.lovoria.com`. Tunggu sampai status *Active*.
3. Buat target CNAME untuk pasangan, mis. `domains.lovoria.com` → CNAME ke `origin.lovoria.com` (proxied). Inilah nilai `CUSTOM_DOMAIN_CNAME_TARGET`.
4. **SSL/TLS → Overview**: mode **Full** (Railway melayani HTTPS). *Edge Certificates → Always Use HTTPS*: on.

## 3. Host header (penting untuk Railway)

Cloudflare meneruskan request custom hostname ke origin dengan `Host: www.samuelsarah.com`. Railway (dan banyak PaaS) merutekan berdasarkan Host, sehingga domain yang tidak terdaftar di Railway ditolak sebelum sampai ke aplikasi. Pilih salah satu:

**A. Cloudflare Worker (disarankan untuk Railway).** Worker menulis ulang Host ke domain Railway dan mengirim host asli lewat header yang dibaca aplikasi (`CUSTOM_DOMAIN_HOST_HEADER`):

```js
// Worker "lovoria-custom-domains", route: */* di zona lovoria.com — menurut dokumentasi
// Cloudflare, route */* juga menangkap trafik custom hostname.
// Variabel Worker: ORIGIN_HOST (domain Railway), OWN_ZONE (mis. lovoria.com).
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const original = url.hostname;
    // Domain Lunovia sendiri (lovoria.com, www., …) diteruskan apa adanya.
    if (original === env.OWN_ZONE || original.endsWith('.' + env.OWN_ZONE)) {
      return fetch(request);
    }
    url.hostname = env.ORIGIN_HOST; // mis. lovaria-production.up.railway.app
    const req = new Request(url, request);
    req.headers.set('X-Lovoria-Host', original);
    // IP pengunjung asli untuk rate limit (bukan IP Worker).
    req.headers.set('X-Forwarded-For', request.headers.get('CF-Connecting-IP') || '');
    return fetch(req, { redirect: 'manual' });
  },
};
```

Lalu isi `CUSTOM_DOMAIN_HOST_HEADER=X-Lovoria-Host`. Trafik domain Lunovia sendiri diteruskan Worker tanpa diubah.

**Domain lama tetap hidup.** Setelah domain sendiri didaftarkan di Railway, `RAILWAY_PUBLIC_DOMAIN` ikut berganti ke domain itu. Isi `EXTRA_HOSTS=<nama>.up.railway.app` supaya domain Railway lama tetap dikenali sebagai host Lunovia — tanpa ini link undangan yang sudah terkirim lewat domain lama menjadi 404 (aturan "host tak dikenal → 404"). Nilai ini juga dipakai sebagai `ORIGIN_HOST` Worker.

**B. Origin yang menerima Host apa pun** (VPS / reverse proxy sendiri): tidak perlu Worker, biarkan `CUSTOM_DOMAIN_HOST_HEADER` kosong.

> Header host hanya dipercaya untuk memilih undangan publik (halaman yang memang publik). Jangan pakai header ini untuk keputusan keamanan lain.

## 4. API token & environment

Buat API token (**My Profile → API Tokens → Create Token → Custom**): permission **Zone → SSL and Certificates → Edit**, zone resource = `lovoria.com`.

| Variable | Contoh |
|---|---|
| `CLOUDFLARE_API_TOKEN` | token di atas |
| `CLOUDFLARE_ZONE_ID` | Zone ID (Overview zona, kolom kanan) |
| `CUSTOM_DOMAIN_CNAME_TARGET` | `domains.lovoria.com` |
| `CUSTOM_DOMAIN_HOST_HEADER` | `X-Lovoria-Host` (opsi A) atau kosong |
| `EXTRA_HOSTS` | `lovaria-production.up.railway.app` (domain Railway lama, dipisah koma bila lebih dari satu) |

Ketiga variabel pertama wajib diisi bersamaan; kosong semua → menu Domain menampilkan "belum tersedia".

## 5. Uji end-to-end (staging)

1. Di dashboard wedding → **Domain** → daftarkan `www.<domain-uji>`; status *Menunggu verifikasi*, instruksi CNAME tampil.
2. Di DNS domain uji: `CNAME www → domains.lovoria.com` (DNS only bila domain uji juga di Cloudflare).
3. Tunggu / klik **Cek ulang** sampai *Aktif* (hostname & sertifikat aktif). Cloudflare dashboard → Custom Hostnames menampilkan hostname yang sama.
4. `https://www.<domain-uji>` menampilkan undangan; `https://www.<domain-uji>/i/<KODE>` menampilkan nama tamu; `BASE_URL/w/<slug>` → 301 ke domain uji; `BASE_URL/i/<KODE>` tetap jalan.
5. Host asing (mis. `curl -H 'X-Lovoria-Host: asal.example' …`) → 404.
6. **Hapus domain** di dashboard → hostname hilang dari Cloudflare; `BASE_URL/w/<slug>` kembali tanpa redirect.

## Cara kerja di aplikasi

| Bagian | Perilaku |
|---|---|
| Tabel `custom_domains` (migration `00011`) | `wedding_id` unik, `domain` unik selama status bukan `removed`; `verification_errors` (jsonb) dari Cloudflare. |
| Validasi | huruf kecil, titik akhir dibuang; skema/path/port/non-ASCII ditolak; domain Lunovia (`BASE_URL`, domain Railway, target CNAME, dan domain induknya) ditolak; domain yang dipakai wedding lain ditolak. Domain utama (tanpa www) diberi peringatan CNAME flattening. |
| Status | `pending_verification` → `active` (hostname **dan** SSL aktif) / `failed` (lewat 72 jam) / `removed` (hilang di Cloudflare). **Cek ulang** bisa mengaktifkan domain `failed` bila CNAME sudah benar. |
| Scheduler | `RunPolling` tiap 5 menit (dan saat start), advisory lock `0x107E0002` — hanya satu instance yang memeriksa. |
| Hapus | hapus custom hostname di Cloudflare **dulu**; bila gagal, baris tetap ada (tidak ada hostname yatim). |
| Resolver (`public-site`) | Host (atau `CUSTOM_DOMAIN_HOST_HEADER`) = custom domain aktif → wedding itu; host Lunovia → path `/i/:code`, `/w/:slug`; host lain → **404 generik**. Lookup di-cache 60 detik per instance, dikosongkan saat status berubah. |
| Redirect | domain aktif + wedding publik: `GET/HEAD /w/:slug…` → **301** `https://<domain>…` (query ikut). `/i/:code` di domain Lunovia tidak dialihkan. |
| URL kanonik | `wedding.Service.CanonicalBaseURL` = `https://<domain>` bila aktif, selain itu `BASE_URL/w/<slug>` (beranda dashboard, T14). |
| Kuota | log `WARN` "mendekati kuota Cloudflare for SaaS" saat domain aktif ≥ 90; `ActiveCount` untuk panel admin (T16). |
