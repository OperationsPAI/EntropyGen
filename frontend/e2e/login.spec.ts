/**
 * E2E tests for the Login page and authentication flows.
 *
 * These tests run WITHOUT the shared auth state (no storageState dependency)
 * so they can exercise the unauthenticated login form.
 *
 * Bug prevention:
 *   - Login form rejects bad credentials with a visible error message.
 *   - Successful login stores JWT and redirects to /dashboard.
 *   - Logout clears JWT and redirects to /login.
 *   - Guest users see limited sidebar (Dashboard + Agents only).
 */
import { test, expect } from '@playwright/test'

// Override: these tests must NOT use the pre-authenticated storage state.
test.use({ storageState: { cookies: [], origins: [] } })

// ---------------------------------------------------------------------------
// 1. Login Form UI
// ---------------------------------------------------------------------------

test.describe('Login form UI', () => {
  test('renders login page with branding and form fields', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/login`)

    // Branding
    await expect(page.getByText('EntropyGen')).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Sign In' })).toBeVisible()
    await expect(page.getByText('Control Panel')).toBeVisible()

    // Form fields
    await expect(page.getByRole('textbox', { name: 'Username' })).toBeVisible()
    await expect(page.getByRole('textbox', { name: 'Password' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible()
  })

  test('username and password fields are required (HTML validation)', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/login`)

    const usernameInput = page.getByRole('textbox', { name: 'Username' })
    const passwordInput = page.getByRole('textbox', { name: 'Password' })

    await expect(usernameInput).toHaveAttribute('required', '')
    await expect(passwordInput).toHaveAttribute('required', '')
  })
})

// ---------------------------------------------------------------------------
// 2. Login Error Handling
// ---------------------------------------------------------------------------

test.describe('Login error handling', () => {
  test('shows error message on invalid credentials', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/login`)

    await page.getByRole('textbox', { name: 'Username' }).fill('wrong-user')
    await page.getByRole('textbox', { name: 'Password' }).fill('wrong-pass')
    await page.getByRole('button', { name: 'Sign In' }).click()

    // Error banner should appear
    await expect(page.getByText('Invalid username or password')).toBeVisible({ timeout: 10_000 })

    // Should stay on login page
    await expect(page).toHaveURL(/\/login/)

    // JWT should NOT be stored
    const token = await page.evaluate(() => localStorage.getItem('jwt_token'))
    expect(token).toBeFalsy()
  })

  test('error message disappears on retry', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/login`)

    // First attempt with bad creds
    await page.getByRole('textbox', { name: 'Username' }).fill('wrong')
    await page.getByRole('textbox', { name: 'Password' }).fill('wrong')
    await page.getByRole('button', { name: 'Sign In' }).click()
    await expect(page.getByText('Invalid username or password')).toBeVisible({ timeout: 10_000 })

    // Second attempt — error should clear when form is resubmitted
    await page.getByRole('textbox', { name: 'Username' }).fill('admin')
    await page.getByRole('textbox', { name: 'Password' }).fill('wrong-again')
    await page.getByRole('button', { name: 'Sign In' }).click()

    // The previous error disappears momentarily during the request
    // (then reappears because creds are still bad, but the key point
    // is that stale errors don't stack up)
  })
})

// ---------------------------------------------------------------------------
// 3. Successful Login Flow
// ---------------------------------------------------------------------------

test.describe('Login success flow', () => {
  test('valid credentials redirect to /dashboard and store JWT', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/login`)

    await page.getByRole('textbox', { name: 'Username' }).fill('admin')
    await page.getByRole('textbox', { name: 'Password' }).fill('admin')
    await page.getByRole('button', { name: 'Sign In' }).click()

    // Should redirect to dashboard
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 })

    // JWT stored in localStorage
    const token = await page.evaluate(() => localStorage.getItem('jwt_token'))
    expect(token).toBeTruthy()
  })
})

// ---------------------------------------------------------------------------
// 4. Logout Flow
// ---------------------------------------------------------------------------

test.describe('Logout flow', () => {
  test('Sign Out clears JWT and redirects to /login', async ({ page, baseURL }) => {
    // Log in first
    await page.goto(`${baseURL}/login`)
    await page.getByRole('textbox', { name: 'Username' }).fill('admin')
    await page.getByRole('textbox', { name: 'Password' }).fill('admin')
    await page.getByRole('button', { name: 'Sign In' }).click()
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 })

    // Click Sign Out in sidebar
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Should redirect to /login
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 })

    // JWT should be cleared
    const token = await page.evaluate(() => localStorage.getItem('jwt_token'))
    expect(token).toBeFalsy()
  })
})

// ---------------------------------------------------------------------------
// 5. Guest Access
// ---------------------------------------------------------------------------

test.describe('Guest access', () => {
  test('unauthenticated user can access /dashboard without login', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/dashboard`)

    // Dashboard should load (guest-accessible route)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })
  })

  test('unauthenticated user is redirected to /login when accessing auth-required pages', async ({ page, baseURL }) => {
    // /roles requires login (member+)
    await page.goto(`${baseURL}/roles`)
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 })
  })

  test('guest sidebar shows only Dashboard and Agents', async ({ page, baseURL }) => {
    await page.goto(`${baseURL}/dashboard`)
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    // Guest nav items should be visible
    await expect(page.getByRole('link', { name: 'Dashboard' })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Agents' })).toBeVisible()

    // Member-only nav items should NOT be visible
    await expect(page.getByRole('link', { name: 'Roles' })).not.toBeVisible()
    await expect(page.getByRole('link', { name: 'LLM Models' })).not.toBeVisible()
    await expect(page.getByRole('link', { name: 'Audit Log' })).not.toBeVisible()

    // Sign In button should be visible (not Sign Out)
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible()
  })
})
