// Thumbnail etalase tema (T21): tangkapan layar sampul undangan contoh tiap
// tema → static/img/themes/<id>.webp (potret 4:5, ≤ 60 KB). Berkas di-commit.
//
//   make theme-thumbs            (server harus jalan & `lovoria demo seed` sudah dibuat)
//
// Target: E2E_BASE_URL (default http://localhost:8080). Daftar tema diambil
// dari kartu "Lihat contoh" di landing, jadi tema baru otomatis ikut.
const { chromium } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

const BASE = process.env.E2E_BASE_URL || 'http://localhost:8080';
const OUT = path.join(__dirname, '..', 'static', 'img', 'themes');
const VIEW = { width: 390, height: 660 }; // ponsel pendek: sampul memadat, nama & foto berdekatan
const CLIP_H = 488; // 4:5 dari lebar 390
const SIZE = { width: 600, height: 750 };
const MAX_BYTES = 60 * 1024;

(async () => {
  const browser = await chromium.launch({ channel: process.env.PW_CHANNEL || undefined });
  const page = await browser.newPage({ viewport: VIEW, deviceScaleFactor: 2, reducedMotion: 'reduce' });
  await page.goto(BASE + '/', { waitUntil: 'networkidle' });
  const ids = await page.$$eval('a[href^="/w/contoh-"]', (as) => [...new Set(as.map((a) => a.getAttribute('href').replace('/w/contoh-', '')))]);
  if (!ids.length) throw new Error('Tidak ada undangan contoh di landing — jalankan `lovoria demo seed` dulu.');
  fs.mkdirSync(OUT, { recursive: true });

  for (const id of ids) {
    await page.goto(`${BASE}/w/contoh-${id}`, { waitUntil: 'networkidle' });
    await page.evaluate(() => document.fonts.ready);
    // Jendela 4:5 digeser supaya nama pasangan (h1) berada di tengahnya.
    const y = await page.evaluate(({ clipH, viewH }) => {
      const r = document.querySelector('h1').getBoundingClientRect();
      return Math.round(Math.max(0, Math.min(viewH - clipH, r.top + r.height / 2 - clipH / 2)));
    }, { clipH: CLIP_H, viewH: VIEW.height });
    const png = (await page.screenshot({ clip: { x: 0, y, width: VIEW.width, height: CLIP_H } })).toString('base64');
    // Kecilkan & ubah ke WebP lewat canvas browser (tanpa dependensi tambahan);
    // kualitas diturunkan sampai muat anggaran.
    let webp;
    for (let q = 0.82; q >= 0.4; q -= 0.06) {
      const data = await page.evaluate(async ({ png, w, h, q }) => {
        const img = new Image();
        img.src = 'data:image/png;base64,' + png;
        await img.decode();
        const c = document.createElement('canvas');
        c.width = w;
        c.height = h;
        c.getContext('2d').drawImage(img, 0, 0, w, h);
        return c.toDataURL('image/webp', q).split(',')[1];
      }, { png, w: SIZE.width, h: SIZE.height, q });
      webp = Buffer.from(data, 'base64');
      if (webp.length <= MAX_BYTES) break;
    }
    fs.writeFileSync(path.join(OUT, `${id}.webp`), webp);
    console.log(`${id}.webp  ${(webp.length / 1024).toFixed(1)} KB`);
  }
  await browser.close();
})().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
