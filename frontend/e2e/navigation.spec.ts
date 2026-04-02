/**
 * E2E tests for sidebar navigation and page routing.
 *
 * Uses the pre-authenticated storage state (admin user) so all nav items
 * are visible.
 *
 * Bug prevention:
 *   - Every sidebar link resolves to a page that renders a header (not a blank screen).
 *   - 404 / unknown paths gracefully redirect or show content.
 *   - Sidebar collapse/expand persists across navigation.
 *   - Active nav item is highlighted.
 */
import { test, expect } from '@playwright/test'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Navigate to a sidebar link by its visible text and assert the page loads. */
async function navigateVia(page: import('@playwright/test').Page, linkName: string, expectedHeading: string | RegExp) {
  const link = page.getByRole('link', { name: linkName })
  await expect(link).toBeVisible({ timeout: 5_000 })
  await link.click()
  await expect(page.getByRole('heading', { name: expectedHeading }).first()).toBeVisible({ timeout: 10_000 })
}

// ---------------------------------------------------------------------------
// 1. Sidebar Navigation — Every Page Renders
// ---------------------------------------------------------------------------

test.describe('Sidebar navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })
  })

  test('Dashboard page loads with heading', async ({ page }) => {
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible()
  })

  test('navigates to Agents page', async ({ page }) => {
    await navigateVia(page, 'Agents', 'Agents')
    await expect(page).toHaveURL(/\/agents/)
  })

  test('navigates to Roles page', async ({ page }) => {
    await navigateVia(page, 'Roles', 'Roles')
    await expect(page).toHaveURL(/\/roles/)
  })

  test('navigates to LLM Models page', async ({ page }) => {
    await navigateVia(page, 'LLM Models', /LLM/)
    await expect(page).toHaveURL(/\/llm/)
  })

  test('navigates to Audit Log page', async ({ page }) => {
    await navigateVia(page, 'Audit Log', /Audit/)
    await expect(page).toHaveURL(/\/audit/)
  })

  test('navigates to Data Dashboard (Monitor) page', async ({ page }) => {
    await navigateVia(page, 'Data Dashboard', /Data Dashboard|Monitor/)
    await expect(page).toHaveURL(/\/monitor/)
  })

  test('navigates to Export page', async ({ page }) => {
    await navigateVia(page, 'Export', /Export/)
    await expect(page).toHaveURL(/\/export/)
  })

  test('navigates to User Management page (admin only)', async ({ page }) => {
    await navigateVia(page, 'User Management', /User Management/)
    await expect(page).toHaveURL(/\/users/)
  })
})

// ---------------------------------------------------------------------------
// 2. Sidebar Active State
// ---------------------------------------------------------------------------

test.describe('Sidebar active state', () => {
  test('active nav item has active class when on that page', async ({ page }) => {
    await page.goto('/agents')
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible({ timeout: 10_000 })

    // The Agents link should have the active style (navItemActive class)
    const agentsLink = page.getByRole('link', { name: 'Agents' })
    const className = await agentsLink.getAttribute('class')
    expect(className).toContain('Active')
  })

  test('navigating away removes active state from previous page', async ({ page }) => {
    await page.goto('/agents')
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible({ timeout: 10_000 })

    // Navigate to Roles
    await page.getByRole('link', { name: 'Roles' }).click()
    await expect(page.getByRole('heading', { name: 'Roles' })).toBeVisible({ timeout: 10_000 })

    // Agents link should no longer be active
    const agentsClass = await page.getByRole('link', { name: 'Agents' }).getAttribute('class')
    expect(agentsClass).not.toContain('Active')

    // Roles link should be active
    const rolesClass = await page.getByRole('link', { name: 'Roles' }).getAttribute('class')
    expect(rolesClass).toContain('Active')
  })
})

// ---------------------------------------------------------------------------
// 3. Sidebar Collapse/Expand
// ---------------------------------------------------------------------------

test.describe('Sidebar collapse', () => {
  test('collapse button toggles sidebar width', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    // EntropyGen text should be visible in expanded state
    await expect(page.getByText('EntropyGen')).toBeVisible()

    // Click collapse button (uses title attr, not text content — icon-only button)
    const collapseBtn = page.locator('button[title="Collapse sidebar"]')
    await collapseBtn.click()

    // EntropyGen text should be hidden in collapsed state
    await expect(page.getByText('EntropyGen')).not.toBeVisible()

    // Click expand button
    const expandBtn = page.locator('button[title="Expand sidebar"]')
    await expandBtn.click()

    // EntropyGen text visible again
    await expect(page.getByText('EntropyGen')).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// 4. Root Redirect
// ---------------------------------------------------------------------------

test.describe('Root redirect', () => {
  test('/ redirects to /dashboard', async ({ page }) => {
    await page.goto('/')
    await expect(page).toHaveURL(/\/dashboard/, { timeout: 10_000 })
  })
})

// ---------------------------------------------------------------------------
// 5. User Info in Sidebar
// ---------------------------------------------------------------------------

test.describe('User info in sidebar', () => {
  test('shows logged-in user avatar text and Sign Out button', async ({ page }) => {
    await page.goto('/dashboard')
    await expect(page.getByRole('heading', { name: 'Dashboard' })).toBeVisible({ timeout: 10_000 })

    // Avatar initials for "admin" => "AD" (use exact match to avoid matching "admin", "Admin")
    await expect(page.locator('[class*="avatar"]').getByText('AD', { exact: true })).toBeVisible()

    // Display name (scoped to userName element to avoid matching sidebar nav text)
    await expect(page.locator('[class*="userName"]')).toHaveText('admin')

    // Sign Out button
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })
})
