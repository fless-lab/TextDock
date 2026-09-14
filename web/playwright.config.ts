import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: 'list',
  use: { baseURL: 'http://127.0.0.1:18258', trace: 'retain-on-failure' },
  webServer: {
    command: '../bin/textdock --listen 127.0.0.1:18258 --db :memory:',
    url: 'http://127.0.0.1:18258/healthz',
    env: { TEXTDOCK_TOKEN: 'textdock-e2e-token-only', TEXTDOCK_WEBHOOK_SECRET: 'textdock-e2e-webhook-secret', TEXTDOCK_RELAY_DRIVER: 'android', TEXTDOCK_ADB_ENABLED: 'true', TEXTDOCK_ADB_PATH: '../bin/adb-fixture' },
    reuseExistingServer: false,
  },
  projects: [
    { name: 'desktop', use: { ...devices['Desktop Chrome'] } },
    { name: 'mobile', use: { ...devices['Pixel 7'] } },
  ],
});
