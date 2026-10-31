// Aset floral Signature per preset warna (T33): memotong lukisan watercolor lalu
// menggeser warna ungunya ke warna preset (mawar krem & daun tetap) →
// static/img/themes/signature/floral/<preset>/<bagian>.webp. Berkas di-commit.
//
//   cd e2e && node floral-tints.js
//
// Berkas hasil boleh diganti lukisan sungguhan tanpa mengubah kode.
const { chromium } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

const SRC_DIR = path.join(__dirname, '..', 'doc', 'lovoria-tasks');
const OUT = path.join(__dirname, '..', 'static', 'img', 'themes', 'signature', 'floral');
const MAX_BYTES = 80 * 1024;

const SHEET = 'Gemini_Generated_Image_a5arsxa5arsxa5ar.png';
const BOUQUET = 'Large-lush.png';
const PETALS = 'Four separate loose.png';

// Potongan aset: src berkas, crop [x, y, lebar, tinggi], width lebar hasil
// (bawaan: tetap), clear serpihan bagian tetangga [x, y, lebar, tinggi] setelah
// dipotong, drop titik pada bagian tetangga yang dibuang sebelum dipotong.
const PARTS = {
  bouquet: { src: BOUQUET, crop: [0, 40, 700, 932], width: 600, drop: [800, 1120], clear: [130, 870, 130, 62] }, // terpotong di kiri
  cluster: { src: SHEET, crop: [224, 44, 416, 332] }, // mawar + lilac
  corner: { src: SHEET, crop: [432, 776, 411, 488], clear: [0, 0, 200, 10] }, // terpotong di kanan & bawah
  'ribbon-left': { src: SHEET, crop: [0, 1096, 416, 168] }, // terpotong di kiri & bawah
  ribbon: { src: SHEET, crop: [0, 476, 843, 308], clear: [600, 290, 243, 18] }, // terpotong di kiri & kanan
  ribbon2: { src: 'Thin satin lilac ribbon.png', crop: [0, 436, 843, 340] }, // terpotong di kiri & kanan
  sprig: { src: 'Long-thin-horizontal.png', crop: [16, 470, 824, 286] },
  rose: { src: 'Single-ivory-rose.png', crop: [224, 360, 444, 530] },
  'petal-a': { src: PETALS, crop: [92, 140, 260, 456], width: 90 },
  'petal-b': { src: PETALS, crop: [444, 200, 320, 432], width: 110 },
  'petal-c': { src: PETALS, crop: [456, 744, 340, 344], width: 110 },
};

// hue: warna tujuan (derajat); sat & light: pengali. null = warna asli lembar aset.
const PRESETS = {
  'signature-lavender': null,
  'sage-garden': { hue: 118, sat: 0.55, light: 0.98 },
  'blush-romance': { hue: 352, sat: 0.95, light: 1.04 },
  'midnight-elegance': { hue: 215, sat: 0.95, light: 0.78 },
  'terracotta-bloom': { hue: 18, sat: 1.2, light: 1.0 },
};

function render(pngB64, part, preset, q) {
  return (async () => {
    const img = new Image();
    img.src = 'data:image/png;base64,' + pngB64;
    await img.decode();
    const src = document.createElement('canvas');
    src.width = img.width;
    src.height = img.height;
    const sctx = src.getContext('2d');
    sctx.drawImage(img, 0, 0);
    if (part.drop) {
      // Buang bagian yang tersambung dengan titik drop.
      const full = sctx.getImageData(0, 0, src.width, src.height);
      const W = src.width, stack = [part.drop[1] * W + part.drop[0]];
      while (stack.length) {
        const k = stack.pop();
        if (k < 0 || k >= W * src.height || full.data[k * 4 + 3] <= 10) continue;
        full.data[k * 4 + 3] = 0;
        stack.push(k + 1, k - 1, k + W, k - W);
      }
      sctx.putImageData(full, 0, 0);
    }
    const [x, y, cw, ch] = part.crop;
    const crop = document.createElement('canvas');
    crop.width = cw;
    crop.height = ch;
    const cctx = crop.getContext('2d');
    cctx.drawImage(src, -x, -y);
    if (part.clear) cctx.clearRect(...part.clear);
    const w = part.width || cw;
    const h = Math.round((ch * w) / cw);
    const c = document.createElement('canvas');
    c.width = w;
    c.height = h;
    const ctx = c.getContext('2d');
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(crop, 0, 0, w, h);
    if (preset) {
      const data = ctx.getImageData(0, 0, w, h);
      const px = data.data;
      const toHsl = (r, g, b) => {
        const max = Math.max(r, g, b), min = Math.min(r, g, b), d = max - min, l = (max + min) / 2;
        if (d === 0) return [0, 0, l];
        const s = d / (1 - Math.abs(2 * l - 1));
        const hue = max === r ? ((g - b) / d + 6) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
        return [hue * 60, s, l];
      };
      const toRgb = (hue, s, l) => {
        const a = s * Math.min(l, 1 - l);
        const f = (n) => {
          const k = (n + hue / 30) % 12;
          return l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
        };
        return [f(0), f(8), f(4)];
      };
      for (let i = 0; i < px.length; i += 4) {
        if (px[i + 3] === 0) continue;
        const [hue, s, l] = toHsl(px[i] / 255, px[i + 1] / 255, px[i + 2] / 255);
        // Seberapa "ungu" piksel ini (termasuk kilap pita yang condong merah
        // muda): halus di tepi rentang supaya tanpa garis.
        const h = hue < 20 ? hue + 360 : hue;
        const inHue = Math.min(1, Math.max(0, Math.min(h - 240, 375 - h) / 15));
        const weight = inHue * Math.min(1, s / 0.08);
        if (weight === 0) continue;
        const rgb = toRgb(preset.hue, Math.min(1, s * preset.sat), Math.min(1, l * preset.light));
        for (let k = 0; k < 3; k++) px[i + k] = Math.round(px[i + k] * (1 - weight) + rgb[k] * 255 * weight);
      }
      ctx.putImageData(data, 0, 0);
    }
    return c.toDataURL('image/webp', q).split(',')[1];
  })();
}

(async () => {
  const browser = await chromium.launch({ channel: process.env.PW_CHANNEL || undefined });
  const page = await browser.newPage();
  for (const [id, preset] of Object.entries(PRESETS)) {
    const dir = path.join(OUT, id);
    fs.mkdirSync(dir, { recursive: true });
    let total = 0;
    for (const [name, part] of Object.entries(PARTS)) {
      let webp;
      for (let q = 0.9; q >= 0.4; q -= 0.05) {
        const png = fs.readFileSync(path.join(SRC_DIR, part.src)).toString('base64');
        const data = await page.evaluate(`(${render.toString()})(${JSON.stringify(png)}, ${JSON.stringify(part)}, ${JSON.stringify(preset)}, ${q})`);
        webp = Buffer.from(data, 'base64');
        if (webp.length <= MAX_BYTES) break;
      }
      fs.writeFileSync(path.join(dir, `${name}.webp`), webp);
      total += webp.length;
    }
    console.log(`${id}  ${(total / 1024).toFixed(0)} KB`);
  }
  await browser.close();
})().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
