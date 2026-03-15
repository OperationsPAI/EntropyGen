import { test as setup, expect } from '@playwright/test'

/**
 * Global setup: login and save auth state for all subsequent tests.
 * Runs once before any test suite.
 */
setup('authenticate', async ({ page, baseURL }) => {
  await page.goto(`${baseURL}/login`)
  await page.getByRole('textbox', { name: 'Username' }).fill('admin')
  await page.getByRole('textbox', { name: 'Password' }).fill('admin')
  await page.getByRole('button', { name: 'Sign In' }).click()

  // Wait for redirect to dashboard
  await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 })

  // Verify JWT is stored
  const token = await page.evaluate(() => localStorage.getItem('jwt_token'))
  expect(token).toBeTruthy()

  // Save auth state for reuse
  await page.context().storageState({ path: './e2e/.auth/storage-state.json' })
})
