import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e', fullyParallel: false, workers: 1, retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: { baseURL: process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:8080', trace: 'on-first-retry', screenshot: 'only-on-failure' },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
