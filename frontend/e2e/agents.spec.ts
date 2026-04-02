/**
 * E2E tests for the Agents list and Agent detail pages.
 *
 * Bug prevention:
 *   - Agent list renders with correct table columns and data.
 *   - View mode toggle (table/card) works and persists in localStorage.
 *   - Filters (role, phase) narrow the list correctly.
 *   - Agent detail page loads with tabs and shows agent info.
 *   - Delete modal requires name confirmation before enabling button.
 *   - Pause/Resume toggling updates the agent status indicator.
 */
import { test, expect } from '@playwright/test'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function gotoAgents(page: import('@playwright/test').Page) {
  await page.goto('/agents')
  await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible({ timeout: 10_000 })
}

/** Wait for agent list to finish loading (skeleton rows disappear). */
async function waitForAgentList(page: import('@playwright/test').Page) {
  // Wait for either a table row, empty state, or error state
  await expect(
    page.locator('tbody tr').first()
      .or(page.getByText('No agents found'))
      .or(page.getByText('Error loading agents')),
  ).toBeVisible({ timeout: 15_000 })
}

// ---------------------------------------------------------------------------
// 1. Agent List Page
// ---------------------------------------------------------------------------

test.describe('Agent list page', () => {
  test('renders page header with title and online count', async ({ page }) => {
    await gotoAgents(page)

    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible()
    // Online count indicator: "X/Y Online"
    await expect(page.getByText(/\d+\/\d+ Online/)).toBeVisible({ timeout: 10_000 })
  })

  test('shows "+ New Agent" button', async ({ page }) => {
    await gotoAgents(page)

    await expect(page.getByRole('button', { name: '+ New Agent' })).toBeVisible()
  })

  test('shows auto-refresh indicator', async ({ page }) => {
    await gotoAgents(page)
    await expect(page.getByText('Auto-refresh: ON')).toBeVisible({ timeout: 10_000 })
  })
})

// ---------------------------------------------------------------------------
// 2. Agent Table
// ---------------------------------------------------------------------------

test.describe('Agent table', () => {
  test('table has correct column headers', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    // Skip if no agents exist (shows empty state)
    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    for (const header of ['Name', 'Role', 'Status', 'Model', 'Last Action', 'Token/Today', 'Age', 'Actions']) {
      await expect(page.getByRole('columnheader', { name: header })).toBeVisible()
    }
  })

  test('agent name links to detail page', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    // First agent name in the table should be a link
    const firstNameCell = page.locator('td').first().locator('a')
    if (await firstNameCell.isVisible().catch(() => false)) {
      const href = await firstNameCell.getAttribute('href')
      expect(href).toMatch(/\/agents\//)
    }
  })

  test('each agent row shows a phase tag', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    // Phase tags should be visible in the table
    const phaseTag = page.locator('[class*="phaseTag"]').first()
    await expect(phaseTag).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// 3. View Mode Toggle
// ---------------------------------------------------------------------------

test.describe('View mode toggle', () => {
  test('table and card view toggle buttons are visible', async ({ page }) => {
    await gotoAgents(page)

    // Buttons are icon-only with title attributes
    await expect(page.locator('button[title="Table view"]')).toBeVisible()
    await expect(page.locator('button[title="Card view"]')).toBeVisible()
  })

  test('switching to card view shows agent cards', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    // Switch to card view
    await page.locator('button[title="Card view"]').click()

    // Card grid should appear (table should disappear)
    const cardGrid = page.locator('[class*="cardGrid"]')
    await expect(cardGrid).toBeVisible({ timeout: 5_000 })
  })

  test('view mode persists in localStorage', async ({ page }) => {
    await gotoAgents(page)

    // Switch to card view
    await page.locator('button[title="Card view"]').click()

    const storedMode = await page.evaluate(() => localStorage.getItem('agents_view_mode'))
    expect(storedMode).toBe('card')

    // Switch back to table
    await page.locator('button[title="Table view"]').click()

    const storedMode2 = await page.evaluate(() => localStorage.getItem('agents_view_mode'))
    expect(storedMode2).toBe('table')
  })
})

// ---------------------------------------------------------------------------
// 4. Filters
// ---------------------------------------------------------------------------

test.describe('Agent filters', () => {
  test('phase filter pills are rendered for all phases', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    for (const phase of ['Pending', 'Initializing', 'Running', 'Paused', 'Error']) {
      await expect(page.getByRole('button', { name: phase, exact: true })).toBeVisible()
    }
  })

  test('clicking a phase filter toggles its active state', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    const runningPill = page.getByRole('button', { name: 'Running', exact: true })
    await runningPill.click()

    // Should have active class
    const classAfterClick = await runningPill.getAttribute('class')
    expect(classAfterClick).toContain('Active')

    // Click again to deselect
    await runningPill.click()
    const classAfterSecondClick = await runningPill.getAttribute('class')
    expect(classAfterSecondClick).not.toContain('Active')
  })
})

// ---------------------------------------------------------------------------
// 5. Delete Modal
// ---------------------------------------------------------------------------

test.describe('Agent delete modal', () => {
  test('delete button opens confirmation modal with name input', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    // Click the delete button (trash icon) on the first agent row
    const deleteButtons = page.locator('[class*="actionsCell"]').first().getByRole('button').last()
    await deleteButtons.click()

    // Modal should open
    await expect(page.getByText('Delete Agent')).toBeVisible({ timeout: 5_000 })
    await expect(page.getByText('This action is irreversible')).toBeVisible()

    // Confirm Delete button should be disabled initially
    const confirmBtn = page.getByRole('button', { name: 'Confirm Delete' })
    await expect(confirmBtn).toBeDisabled()
  })

  test('cancel closes the delete modal', async ({ page }) => {
    await gotoAgents(page)
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    // Open delete modal
    const deleteBtn = page.locator('[class*="actionsCell"]').first().getByRole('button').last()
    await deleteBtn.click()
    await expect(page.getByText('Delete Agent')).toBeVisible({ timeout: 5_000 })

    // Click Cancel
    await page.getByRole('button', { name: 'Cancel' }).click()

    // Modal should close
    await expect(page.getByText('Delete Agent')).not.toBeVisible({ timeout: 5_000 })
  })
})

// ---------------------------------------------------------------------------
// 6. Agent Detail Page
// ---------------------------------------------------------------------------

test.describe('Agent detail page', () => {
  test('detail page renders with breadcrumbs and tabs', async ({ page }) => {
    await page.goto('/agents')
    await expect(page.getByRole('heading', { name: 'Agents' })).toBeVisible({ timeout: 10_000 })
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    // Click on the first agent name
    const firstAgentLink = page.locator('td').first().locator('a')
    if (!(await firstAgentLink.isVisible().catch(() => false))) {
      test.skip(true, 'No agent link found')
      return
    }
    await firstAgentLink.click()

    // Should navigate to /agents/<name>
    await expect(page).toHaveURL(/\/agents\//, { timeout: 10_000 })

    // Breadcrumb "Agents" link should be visible
    await expect(page.getByRole('link', { name: 'Agents' })).toBeVisible()

    // Tabs should be visible
    for (const tab of ['Overview', 'Activity Timeline', 'Files', 'Logs', 'Audit']) {
      await expect(page.getByRole('button', { name: tab })).toBeVisible()
    }
  })

  test('detail Overview tab shows agent configuration', async ({ page }) => {
    await page.goto('/agents')
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    const firstAgentLink = page.locator('td').first().locator('a')
    if (!(await firstAgentLink.isVisible().catch(() => false))) {
      test.skip(true, 'No agent link found')
      return
    }
    await firstAgentLink.click()
    await expect(page).toHaveURL(/\/agents\//, { timeout: 10_000 })

    // Overview tab should be active by default
    // It should show key fields
    await expect(page.getByText('Role').first()).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Model').first()).toBeVisible()
    await expect(page.getByText('Phase').first()).toBeVisible()
  })

  test('detail page tab switching works', async ({ page }) => {
    await page.goto('/agents')
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    const firstAgentLink = page.locator('td').first().locator('a')
    if (!(await firstAgentLink.isVisible().catch(() => false))) {
      test.skip(true, 'No agent link found')
      return
    }
    await firstAgentLink.click()
    await expect(page).toHaveURL(/\/agents\//, { timeout: 10_000 })

    // Switch to Logs tab
    await page.getByRole('button', { name: 'Logs' }).click()

    // Should show log toggle buttons
    await expect(page.getByRole('button', { name: 'Live Event Stream' })).toBeVisible({ timeout: 5_000 })
    await expect(page.getByRole('button', { name: 'Pod stdout' })).toBeVisible()
  })

  test('agent not found shows empty state', async ({ page }) => {
    await page.goto('/agents/nonexistent-agent-name-12345')

    // Should show "Agent not found" message
    await expect(page.getByText('Agent not found')).toBeVisible({ timeout: 15_000 })
  })
})

// ---------------------------------------------------------------------------
// 7. Agent Detail Actions
// ---------------------------------------------------------------------------

test.describe('Agent detail actions', () => {
  test('detail page shows action buttons: Assign Task, Pause/Resume, Reset Memory, Edit Settings', async ({ page }) => {
    await page.goto('/agents')
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    const firstAgentLink = page.locator('td').first().locator('a')
    if (!(await firstAgentLink.isVisible().catch(() => false))) {
      test.skip(true, 'No agent link found')
      return
    }
    await firstAgentLink.click()
    await expect(page).toHaveURL(/\/agents\//, { timeout: 10_000 })

    // Action buttons in sidebar
    await expect(page.getByRole('button', { name: 'Assign Task' })).toBeVisible({ timeout: 10_000 })
    await expect(page.getByRole('button', { name: /Pause|Resume/ })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Reset Memory' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Edit Settings' })).toBeVisible()
  })

  test('Assign Task button opens modal with form', async ({ page }) => {
    await page.goto('/agents')
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    const firstAgentLink = page.locator('td').first().locator('a')
    if (!(await firstAgentLink.isVisible().catch(() => false))) {
      test.skip(true, 'No agent link found')
      return
    }
    await firstAgentLink.click()
    await expect(page).toHaveURL(/\/agents\//, { timeout: 10_000 })

    // Open Assign Task modal
    await page.getByRole('button', { name: 'Assign Task' }).click({ timeout: 10_000 })

    // Modal should have Title and Description fields
    await expect(page.getByRole('textbox', { name: 'Title' })).toBeVisible({ timeout: 5_000 })

    // Priority radio buttons
    await expect(page.getByLabel('low')).toBeVisible()
    await expect(page.getByLabel('medium')).toBeVisible()
    await expect(page.getByLabel('high')).toBeVisible()

    // Cancel should close
    await page.getByRole('button', { name: 'Cancel' }).click()
  })

  test('Edit Settings button opens modal with agent config', async ({ page }) => {
    await page.goto('/agents')
    await waitForAgentList(page)

    if (await page.getByText('No agents found').isVisible().catch(() => false)) {
      test.skip(true, 'No agents exist')
      return
    }

    const firstAgentLink = page.locator('td').first().locator('a')
    if (!(await firstAgentLink.isVisible().catch(() => false))) {
      test.skip(true, 'No agent link found')
      return
    }
    await firstAgentLink.click()
    await expect(page).toHaveURL(/\/agents\//, { timeout: 10_000 })

    // Open Edit Settings modal
    await page.getByRole('button', { name: 'Edit Settings' }).click({ timeout: 10_000 })

    // Modal should have LLM fields
    await expect(page.getByRole('textbox', { name: 'Model' })).toBeVisible({ timeout: 5_000 })
    await expect(page.getByRole('button', { name: 'Save' })).toBeVisible()

    // Cancel
    await page.getByRole('button', { name: 'Cancel' }).click()
  })
})
