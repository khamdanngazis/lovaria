// Bunga Signature per preset warna (T33): mewarnai ulang satu lukisan watercolor
// → static/img/themes/signature/floral/<preset>.webp (≤ 80 KB). Berkas di-commit.
//
//   cd e2e && node floral-tints.js
//
// Kelopak krem digeser ke warna preset; daun hanya sedikit mengikuti; alpha
// tidak berubah. Berkas hasil boleh diganti lukisan sungguhan tanpa ubah kode.
const { chromium } = require('@playwright/test');
const fs = require('node:fs');
const path = require('node:path');

const SRC = path.join(__dirname, '..', 'doc', 'lovoria-tasks', 'Gemini_Generated_Image_266ol6266ol6266o.png');
const OUT = path.join(__dirname, '..', 'static', 'img', 'themes', 'signature', 'floral');
const SIZE = 840; // tampil maks. 420px, tajam di layar 2x
const MAX_BYTES = 80 * 1024;

// petal: warna tujuan kelopak; strength: seberapa kuat digeser (0–1);
// leaf: warna tujuan daun; leafStrength: pergeseran daun.
const PRESETS = {
  'signature-lavender': { petal: '#a987a8', strength: 0.8, leaf: '#8f9a86', leafStrength: 0.25 },
  'sage-garden': { petal: '#c7d2bb', strength: 0.55, leaf: '#6f8a6c', leafStrength: 0.4 },
  'blush-romance': { petal: '#d9a5a5', strength: 0.75, leaf: '#94a085', leafStrength: 0.2 },
  'midnight-elegance': { petal: '#d8c7ac', strength: 0.5, leaf: '#63758d', leafStrength: 0.45 },
  'terracotta-bloom': { petal: '#d39a78', strength: 0.75, leaf: '#8f9468', leafStrength: 0.25 },
};

function tint(pngB64, size, preset, q) {
  return (async () => {
    const hex = (h) => [1, 3, 5].map((i) => parseInt(h.slice(i, i + 2), 16));
    const img = new Image();
    img.src = 'data:image/png;base64,' + pngB64;
    await img.decode();
    const c = document.createElement('canvas');
    c.width = c.height = size;
    const ctx = c.getContext('2d');
    ctx.drawImage(img, 0, 0, size, size);
    const data = ctx.getImageData(0, 0, size, size);
    const px = data.data;
    const petal = hex(preset.petal);
    const leaf = hex(preset.leaf);
    for (let i = 0; i < px.length; i += 4) {
      if (px[i + 3] === 0) continue;
      const [r, g, b] = [px[i], px[i + 1], px[i + 2]];
      const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
      // Hijau: g lebih tinggi dari r → daun; selain itu kelopak, batang, bayangan.
      const green = Math.min(1, Math.max(0, (g - r) / 24));
      // Kelopak digelapkan sedikit supaya tetap terlihat di atas latar ivory.
      const shade = Math.pow(lum, 1.25);
      const target = green > 0.5 ? leaf : petal;
      const strength = green > 0.5 ? preset.leafStrength : preset.strength;
      for (let k = 0; k < 3; k++) {
        const t = target[k] / 255;
        // Overlay: terang tetap terang, bayangan mengikuti warna tujuan.
        const o = shade < 0.5 ? 2 * shade * t : 1 - 2 * (1 - shade) * (1 - t);
        px[i + k] = Math.round(px[i + k] * (1 - strength) + o * 255 * strength);
      }
    }
    ctx.putImageData(data, 0, 0);
    return c.toDataURL('image/webp', q).split(',')[1];
  })();
}

(async () => {
  const browser = await chromium.launch({ channel: process.env.PW_CHANNEL || undefined });
  const page = await browser.newPage();
  const png = fs.readFileSync(SRC).toString('base64');
  fs.mkdirSync(OUT, { recursive: true });
  for (const [id, preset] of Object.entries(PRESETS)) {
    let webp;
    for (let q = 0.85; q >= 0.4; q -= 0.05) {
      const data = await page.evaluate(`(${tint.toString()})(${JSON.stringify(png)}, ${SIZE}, ${JSON.stringify(preset)}, ${q})`);
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
