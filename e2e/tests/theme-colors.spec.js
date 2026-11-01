// Tema warna di halaman Tema (T37): pilihan warna mengikuti tema yang dipilih
// tanpa reload, hanya preset tema terpilih yang terkirim, dan tersimpan.
const { test, expect } = require('@playwright/test');

test('tema warna mengikuti tema yang dipilih', async ({ page }) => {
  const email = `e2e-warna-${Date.now()}@example.test`;
  await page.goto('/register');
  await page.locator('#register-name').fill('Tes Warna');
  await page.locator('#register-email').fill(email);
  await page.locator('#register-password').fill('password-e2e-123');
  await page.locator('#register-password_confirmation').fill('password-e2e-123');
  await page.getByRole('button', { name: /daftar/i }).click();

  await expect(page).toHaveURL(/\/dashboard\/weddings\/new/);
  await page.locator('label:has(input[value="elegant"])').click();
  await page.getByRole('button', { name: 'Lanjut' }).click();
  await page.locator('#wedding-groom-name').fill('Budi');
  await page.locator('#wedding-bride-name').fill('Sari');
  await page.locator('#wedding-date').fill(`${new Date().getFullYear() + 1}-06-06`);
  await page.getByRole('button', { name: /Buat & lihat undangan/ }).click();
  await expect(page).toHaveURL(/\/dashboard\/weddings\/[0-9a-f-]{36}\/start$/);
  const theme = new URL(page.url()).pathname.replace(/\/start$/, '/theme');

  await page.goto(theme);
  const group = (id) => page.locator(`[data-theme-presets="${id}"]`);
  const preview = page.locator('#theme-preview');

  // Grup tema tersimpan (Elegan) tampil dan aktif; grup lain tersembunyi.
  await expect(group('elegant')).toBeVisible();
  await expect(group('modern')).toBeHidden();
  await expect(group('elegant').locator('input[type=radio]:not([disabled])')).toHaveCount(5);
  await group('elegant').locator('label:has(input[value="navy-gold"])').click();
  await expect(preview).toHaveAttribute('src', /theme_id=elegant.*color_preset=navy-gold|color_preset=navy-gold.*theme_id=elegant/);

  // Ganti tema tanpa reload: grup ikut berganti, preset tema lama tidak terkirim.
  await page.getByText('Ganti tema').click();
  await page.locator('label:has(input[name="theme_id"][value="modern"])').click();
  await expect(group('modern')).toBeVisible();
  await expect(group('elegant')).toBeHidden();
  await expect(group('elegant').locator('input[type=radio][disabled]')).toHaveCount(5);
  await expect(preview).toHaveAttribute('src', /color_preset=teal/);
  await group('modern').locator('label:has(input[value="plum"])').click();
  await expect(preview).toHaveAttribute('src', /color_preset=plum/);
  await expect(preview).not.toHaveAttribute('src', /navy-gold/);

  // Warna sendiri menimpa tema warna: catatannya muncul saat dicentang.
  await page.getByText('Warna utama sendiri').click();
  await expect(page.getByText('Warna ini menimpa warna utama dari tema warna.')).toBeVisible();
  await page.getByText('Warna utama sendiri').click();

  // Simpan lalu muat ulang: tema & tema warna tersimpan.
  await page.getByRole('button', { name: /Simpan/ }).last().click();
  await expect(page).toHaveURL(/saved=1/);
  await expect(group('modern')).toBeVisible();
  await expect(group('modern').locator('input[value="plum"]')).toBeChecked();
  await expect(preview).toHaveAttribute('src', /preview/);
  const html = await (await page.request.get(theme + '/preview')).text();
  expect(html).toContain('--lv-primary:#6b3a6e;');
});
