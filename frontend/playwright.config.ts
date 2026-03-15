import { defineConfig } from '@playwright/test'

/**
 * Playwright E2E test configuration for EntropyGen Control Panel.
 *
 * Prerequisites:
 *   - Platform running (skaffold run / kind cluster)
 *   - At least one agent (dev-test) deployed and Running
 *   - Agent must have completed >= 1 cron cycle (sessions exist)
 *
 * Run:
 *   npx playwright test                    # all tests
 *   npx playwright test observe            # only observe tests
 *   npx playwright test --headed           # watch in browser
 *   npx playwright test --ui               # interactive UI mode
 */
export default defineConfig({
  testDir: './e2e',
  timeout: 60_000,
  retries: 1,
  workers: 1, // sequential — tests share auth state

  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://10.10.10.220:30083',
    screenshot: 'only-on-failure',
    trace: 'on-first-retry',
    video: 'on-first-retry',
  },

  projects: [
    {
      name: 'setup',
      testMatch: /global-setup\.ts/,
    },
    {
      name: 'chromium',
      use: {
        browserName: 'chromium',
        storageState: './e2e/.auth/storage-state.json',
      },
      dependencies: ['setup'],
    },
  ],

  outputDir: './e2e/test-results',
})
