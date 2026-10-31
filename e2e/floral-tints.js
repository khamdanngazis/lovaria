// Aset floral Signature per preset warna (T33): memotong lembar aset watercolor
// lalu menggeser warna ungunya ke warna preset (mawar krem & daun tetap) →
// static/img/themes/signature/floral/<preset>/<bagian>.webp. Berkas di-commit.
//
//   cd e2e && node floral-tints.js
//
// Berkas hasil boleh diganti lukisan sungguhan tanpa mengubah kode.
const { chromium } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

const SRC = path.join(__dirname, '..', 'doc', 'lovoria-tasks', 'Gemini_Generated_Image_a5arsxa5arsxa5ar.png');
const OUT = path.join(__dirname, '..', 'static', 'img', 'themes', 'signature', 'floral');
const MAX_BYTES = 80 * 1024;

// Potongan lembar aset: [x, y, lebar, tinggi].
const PARTS = {
  cluster: [224, 44, 416, 332], // mawar + lilac, tidak terpotong
  corner: [432, 776, 411, 488], // buket + pita, terpotong di kanan & bawah
  'ribbon-left': [0, 1096, 416, 168], // pita, terpotong di kiri & bawah
  ribbon: [0, 476, 843, 308], // pita melintang + kelopak, terpotong di kiri & kanan
  petal: [176, 500, 68, 44],
};

// Serpihan bagian tetangga yang ikut terpotong, dihapus: [x, y, lebar, tinggi].
const CLEAR = {
  corner: [0, 0, 200, 10],
  ribbon: [600, 290, 243, 18],
};

// hue: warna tujuan (derajat); sat & light: pengali. null = warna asli lembar aset.
const PRESETS = {
  'signature-lavender': null,
  'sage-garden': { hue: 118, sat: 0.55, light: 0.98 },
  'blush-romance': { hue: 352, sat: 0.95, light: 1.04 },
  'midnight-elegance': { hue: 215, sat: 0.95, light: 0.78 },
  'terracotta-bloom': { hue: 18, sat: 1.2, light: 1.0 },
};

function render(pngB64, part, clear, preset, q) {
  return (async () => {
    const img = new Image();
    img.src = 'data:image/png;base64,' + pngB64;
    await img.decode();
    const [x, y, w, h] = part;
    const c = document.createElement('canvas');
    c.width = w;
    c.height = h;
    const ctx = c.getContext('2d');
    ctx.drawImage(img, -x, -y);
    if (clear) ctx.clearRect(...clear);
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
  const png = fs.readFileSync(SRC).toString('base64');
  for (const [id, preset] of Object.entries(PRESETS)) {
    const dir = path.join(OUT, id);
    fs.mkdirSync(dir, { recursive: true });
    let total = 0;
    for (const [name, part] of Object.entries(PARTS)) {
      let webp;
      for (let q = 0.9; q >= 0.4; q -= 0.05) {
        const data = await page.evaluate(`(${render.toString()})(${JSON.stringify(png)}, ${JSON.stringify(part)}, ${JSON.stringify(CLEAR[name] || null)}, ${JSON.stringify(preset)}, ${q})`);
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
