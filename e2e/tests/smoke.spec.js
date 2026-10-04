// Smoke test alur utama (T17): register → wizard → acara → foto → tamu →
// publikasi → undangan → RSVP → ucapan. Satu akun baru per jalan.
const path = require('path');
const { test, expect } = require('@playwright/test');

test('alur utama pasangan & tamu', async ({ page, browser }) => {
  const email = `e2e-${Date.now()}@example.test`;
  page.on('dialog', (d) => d.accept()); // konfirmasi (Publikasikan, dll.)

  // 1. Register
  await page.goto('/register');
  await page.locator('#register-name').fill('Tes E2E');
  await page.locator('#register-email').fill(email);
  await page.locator('#register-password').fill('password-e2e-123');
  await page.getByRole('button', { name: /daftar/i }).click();

  // 2. Wizard 3 langkah → beranda wedding
  await expect(page).toHaveURL(/\/dashboard\/weddings\/new/);
  await page.locator('#wedding-groom-name').fill('Budi');
  await page.locator('#wedding-bride-name').fill('Sari');
  await page.getByRole('button', { name: 'Lanjut' }).click();
  await page.locator('#wedding-title').fill('Pernikahan Budi & Sari');
  const nextYear = new Date().getFullYear() + 1;
  await page.locator('#wedding-date').fill(`${nextYear}-06-06`);
  await page.getByRole('button', { name: 'Lanjut' }).click();
  await page.getByRole('button', { name: /Selesai/ }).click();
  await expect(page).toHaveURL(/\/dashboard\/weddings\/[0-9a-f-]{36}(\?.*)?$/);
  const dash = new URL(page.url()).pathname;

  // 3. Acara
  await page.goto(dash + '/events');
  await page.getByRole('link', { name: /Tambah acara/ }).click();
  await page.locator('[name="name"]').first().fill('Resepsi');
  await page.locator('[name="type"]').first().selectOption('reception');
  await page.locator('[name="date"]').first().fill(`${nextYear}-06-06`);
  await page.locator('[name="start_time"]').first().fill('11:00');
  await page.locator('[name="venue"]').first().fill('Gedung Serbaguna');
  await page.getByRole('button', { name: 'Simpan' }).click();
  await expect(page.getByText('Gedung Serbaguna')).toBeVisible();

  // 4. Upload foto (pengunggah Alpine: input file tersembunyi)
  await page.goto(dash + '/gallery');
  await page.locator('input[type="file"][multiple]').setInputFiles(path.join(__dirname, '../fixtures/photo.jpg'));
  await expect(page.locator('#gallery img').first()).toBeVisible({ timeout: 20_000 });

  // 5. Tamu (tambah cepat)
  await page.goto(dash + '/guests');
  await page.locator('#guest-quick [name="name"]').fill('Tamu E2E');
  await page.locator('#guest-quick [name="name"]').press('Enter');
  const row = page.locator('#guests li', { hasText: 'Tamu E2E' });
  await expect(row).toBeVisible();
  const code = (await row.locator('.font-mono').innerText()).trim();
  expect(code).toMatch(/^[A-Z0-9]{7}$/);

  // 6. Publikasikan
  await page.goto(dash);
  await page.getByRole('button', { name: 'Publikasikan' }).click();
  await expect(page.getByText('Undangan dipublikasikan')).toBeVisible();

  // 7–9. Sebagai tamu (tanpa login): undangan → RSVP → ucapan
  // Layar pendek (360×640): sampul bisa lebih tinggi dari layar — tombol "Buka
  // Undangan" harus tetap terjangkau (regresi T21: tombol terpotong & terkunci).
  const guestCtx = await browser.newContext({ viewport: { width: 360, height: 640 } });
  const guest = await guestCtx.newPage();
  await guest.goto(`/i/${code}`);
  await expect(guest.getByText('Tamu E2E').first()).toBeVisible();
  // Isi terkunci di belakang sampul sampai tombol pembuka ditekan.
  await expect(guest.locator('#rsvp')).toBeHidden();
  await guest.getByRole('link', { name: 'Buka Undangan' }).click();
  await expect(guest.locator('#rsvp')).toBeVisible();

  await guest.locator('#rsvp label', { hasText: 'Hadir' }).first().click();
  await guest.locator('#rsvp-message').fill('Selamat ya! 🎉');
  await guest.getByRole('button', { name: 'Kirim konfirmasi' }).click();
  await expect(guest.getByText('Konfirmasi Anda tersimpan')).toBeVisible();

  await guest.locator('#gb-message').fill('Semoga bahagia selalu');
  await guest.getByRole('button', { name: 'Kirim ucapan' }).click();
  await expect(guest.getByText('Ucapan Anda terkirim')).toBeVisible();
  await expect(guest.locator('#guestbook-list').getByText('Semoga bahagia selalu')).toBeVisible();
  await guestCtx.close();

  // RSVP tercermin di dashboard pasangan.
  await page.goto(dash + '/rsvp');
  await expect(page.locator('#rsvp-list').getByText('Tamu E2E')).toBeVisible();
});
