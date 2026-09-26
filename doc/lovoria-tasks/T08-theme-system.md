# T08 — Theme System & Registry

**Estimasi:** 2–3 hari · **Depends on:** T04 · **Modul:** `modules/theme`, `/src/templates`

## Konteks
Dua lapis kustomisasi: token visual (CSS vars) dan layout per tema (templ terpisah), dikontrol satu theme registry (Arsitektur §6).

## Scope (In)
- `theme.Registry`: map `theme_id → ThemeDef{ ID, Name, Preview, Layout components, DefaultTokens, Islands []string (kosong di MVP) }`
- 4 tema: `elegant`, `minimal`, `romantic`, `modern`
- Per tema, komponen templ: `Layout`, `Opening/Hero`, `Couple`, `LoveStory`, `Events`, `Gallery`, `Closing`
- Komponen shared di `/templates/shared`: `RSVPSection`, `GuestbookSection`, `GiftSection` (placeholder markup; logic diisi T10/T11)
- Token CSS via `:root[data-theme="..."]` + override per wedding: `primary_color`, `font_heading`, `font_body`, `background` (warna/gambar), `cover_image`
- Tabel `wedding_theme_settings` (`wedding_id` PK, `primary_color`, `font_heading`, `font_body`, `background_value`, `cover_image_url`) — hanya dari whitelist font
- Dashboard: pilih tema (kartu preview), atur warna/font/background, **preview live** di iframe memakai data dummy + data wedding sendiri
- Fungsi `Render(ctx, weddingView) templ.Component` — satu-satunya pintu masuk render wedding (dipakai T09)

## Out of Scope
- Drag & drop builder, premium themes, islands Svelte.

## Acceptance Criteria
- [ ] Menambah tema baru cukup: folder baru + satu entri registry (didokumentasikan di `docs/themes.md`)
- [ ] Tidak ada `switch themeID` di luar modul theme
- [ ] Warna input divalidasi hex; font di luar whitelist ditolak
- [ ] Keempat tema lolos cek visual di 375px dan 1280px
- [ ] Google Fonts dimuat hanya font yang dipakai

## Catatan untuk Agent
- `weddingView` adalah struct DTO gabungan (couple, events, stories, gallery, settings) yang disusun T09 — definisikan struct-nya di modul theme agar kontraknya jelas.
