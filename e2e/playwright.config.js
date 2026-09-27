// E2E smoke test (T17). Target: E2E_BASE_URL (default http://localhost:8080).
// Di CI aplikasi dijalankan lebih dulu oleh workflow; lokal: `make e2e`.
const { defineConfig, devices } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './tests',
  timeout: 60_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:8080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    // Lokal boleh memakai Chrome terpasang (PW_CHANNEL=chrome) tanpa unduh browser.
    channel: process.env.PW_CHANNEL || undefined,
  },
  projects: [{ name: 'mobile', use: { ...devices['Pixel 7'], channel: process.env.PW_CHANNEL || undefined } }],
});
