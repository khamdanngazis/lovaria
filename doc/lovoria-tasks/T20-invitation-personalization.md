# T20 — Personalisasi Undangan: Musik, Hitung Mundur, Kutipan, & Susunan Bagian

**Estimasi:** 2–3 hari · **Depends on:** T06, T08, T09 · **Modul:** `modules/theme` (pengaturan tampilan), `modules/wedding`, `templates/themes`, `templates/shared`, `static/js/invitation.js`

## Konteks
Prinsip **Personal** di brand positioning: *"Setiap wedding harus terasa seperti milik pasangan, bukan template generik."* Saat ini personalisasi terbatas pada pilihan 4 tema + warna, font, latar, dan foto sampul (T08). Fitur yang lazim diharapkan pasangan di undangan digital Indonesia belum ada: musik latar, hitung mundur, kutipan/ayat pembuka, serta kendali atas bagian mana yang tampil dan urutannya.

## Scope (In)
**1. Musik latar**
- Pasangan memilih musik dari **pustaka lagu bebas royalti** bawaan (±8–10 lagu instrumental, disimpan di R2 bucket foto dengan prefix `music/`) **atau** mengunggah MP3 sendiri (maks. 8 MB, masuk kuota storage paket).
- Di undangan: tombol putar/jeda melayang (pojok bawah). Musik **tidak diputar otomatis** saat halaman dibuka (browser memblokir autoplay); mulai diputar saat tamu menekan **"Buka Undangan"** di bagian pembuka, bisa dimatikan kapan saja.
- Pasangan bisa menonaktifkan musik.

**2. Hitung mundur**
- Bagian hitung mundur (hari/jam/menit/detik) menuju **acara pertama** (tanggal + jam mulai, zona waktu wedding).
- Pada hari H: "Hari ini!"; setelah lewat: bagian tidak tampil. Tanpa JS: tampil sisa hari yang dihitung server.
- Tombol "Simpan ke kalender" (sudah ada per acara, T09) ditautkan dari bagian ini.

**3. Kutipan / ayat pembuka & teks sapaan**
- Bagian **kutipan** setelah pembuka: teks bebas (≤ 500 karakter) + sumber opsional (mis. "QS. Ar-Rum: 21"). Disediakan beberapa **contoh siap pakai** (ayat/kutipan umum lintas agama & netral) yang bisa dipilih lalu diedit.
- **Teks sapaan** di pembuka (default "Kepada Yth. Bapak/Ibu/Saudara/i") dan **kalimat penutup** dapat diubah.

**4. Susunan & visibilitas bagian**
- Pasangan dapat **menyembunyikan** bagian: cerita, galeri, hitung mundur, kutipan, ucapan, hadiah (RSVP & acara selalu tampil saat status mengizinkan).
- Pasangan dapat **mengubah urutan** bagian tengah (antara pembuka dan penutup) dengan tombol naik/turun (tanpa drag & drop).
- Preview tema di dashboard (T08) langsung mencerminkan pengaturan.

## Out of Scope
- Drag & drop page builder (Arsitektur §10.4 — sinyal evaluasi ulang).
- Video latar, efek animasi baru, tema baru.
- Pencarian/pemutaran lagu dari layanan streaming (hak cipta).

## Detail Teknis
- **Penyimpanan**: perluas `wedding_theme_settings` (modul theme) dengan kolom: `music_url text`, `music_enabled boolean`, `quote_text text`, `quote_source text`, `greeting_text text`, `closing_text text`, `hidden_sections text[]`, `section_order text[]`. Validasi di `theme.ValidateSettings` (whitelist nama bagian; URL musik harus dari R2 publik/prefix `music/` atau upload milik wedding).
- **Registry bagian**: urutan & nama bagian didefinisikan sekali di `modules/theme` (mis. `SectionIDs = []string{"couple","countdown","quote","events","story","gallery","rsvp","guestbook","gift"}`); `theme.Render` menyusun bagian sesuai `section_order` + `hidden_sections` — logika tetap di registry (`TestNoThemeLogicOutsideModule`).
- **View** (`theme/view`): tambah `Music{URL, Enabled}`, `Countdown{Target time.Time, DaysLeft int}`, `Quote{Text, Source}`, `Greeting`, `Closing`.
- **Komponen**: `shared.CountdownSection`, `shared.QuoteSection`, `shared.MusicButton` (dipakai semua tema; tema boleh override gaya lewat token CSS).
- **JS**: tambahkan ke `static/js/invitation.js` (tanpa Alpine, sesuai T09): pemutar audio (mulai setelah klik "Buka Undangan", simpan pilihan mute di `sessionStorage`), hitung mundur (update per detik, hormati `prefers-reduced-motion` untuk animasi). Tetap tanpa script inline (CSP T17).
- **Upload musik**: route `POST /dashboard/weddings/:id/theme/music` (multipart, `audio/mpeg`, cek magic bytes, ≤ 8 MB), simpan ke R2 `weddings/<id>/music/<uuid>.mp3`, kuota lewat `gallery`/storage yang sama (reserve/release). Pustaka bawaan: manifest di `modules/theme` (judul, durasi, URL, lisensi).
- **Cache** publik (`BuildPublic`, T17): kunci cache ikut `wedding_theme_settings.updated_at` atau di-invalidate saat pengaturan disimpan.
- **Performa**: audio `preload="none"`; tidak menambah JS > 3 KB gzip ke halaman undangan.

## Acceptance Criteria
- [ ] Musik: bawaan & unggahan bisa dipilih; di undangan tidak berbunyi sebelum "Buka Undangan", tombol jeda berfungsi, bisa dinonaktifkan; unggahan non-MP3 / > 8 MB ditolak; kuota storage bertambah/berkurang benar
- [ ] Hitung mundur benar di zona waktu wedding (uji WIB & WIT seperti T12), menampilkan "Hari ini!" pada hari H, hilang setelahnya; tanpa JS tampil sisa hari
- [ ] Kutipan & teks sapaan/penutup tampil di keempat tema; teks di-escape (uji XSS)
- [ ] Menyembunyikan & mengurutkan bagian berlaku di undangan publik dan preview dashboard
- [ ] Lighthouse mobile undangan tetap Performance ≥ 90 (baseline T09: 100) dan tidak ada pelanggaran CSP
- [ ] Test: validasi pengaturan (whitelist bagian, panjang teks, URL musik), render urutan bagian, upload musik (tipe, ukuran, kuota), isolasi tenant

## Catatan untuk Agent
- **Hak cipta**: pustaka musik bawaan hanya lagu bebas royalti dengan lisensi tercatat di manifest; unggahan pengguna menjadi tanggung jawab pasangan (tambahkan kalimat di Syarat & Ketentuan).
- Contoh kutipan/ayat: sediakan pilihan lintas agama dan netral; jangan menampilkan ayat bawaan secara default tanpa dipilih pasangan.
- Jangan autoplay audio; jangan menambah Alpine ke halaman undangan.
