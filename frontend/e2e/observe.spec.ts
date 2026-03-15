/**
 * E2E tests for the Observe (Agent Live Room) page.
 *
 * Test philosophy:
 *   - Test real user journeys end-to-end, not component internals.
 *   - Each test asserts on visible outcomes (rendered text, network success).
 *   - Assertions cover the "what bug does this prevent?" principle:
 *     every assert maps to a real bug we've had or an API contract we depend on.
 *
 * Prerequisites:
 *   - Agent "dev-test" must be Running and have >=1 completed session.
 *   - Observer sidecar must be reachable through the gateway proxy.
 */
import { test, expect, type Page } from '@playwright/test'

const AGENT_NAME = process.env.E2E_AGENT_NAME || 'dev-test'
const OBSERVE_URL = `/observe/${AGENT_NAME}`

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Navigate to observe page and wait for initial data load. */
async function gotoObserve(page: Page) {
  await page.goto(OBSERVE_URL)
  // Wait for the page header to render with the agent name
  await expect(page.getByRole('heading', { name: AGENT_NAME })).toBeVisible({ timeout: 10_000 })
}

/** Wait for API responses to settle (sessions + current loaded). */
async function waitForDataLoad(page: Page) {
  // Wait until "Loading session..." disappears — meaning current session loaded
  await expect(page.getByText('Loading session...')).not.toBeVisible({ timeout: 15_000 })
}

// ---------------------------------------------------------------------------
// 1. Page Layout & Navigation
// ---------------------------------------------------------------------------

test.describe('Observe page layout', () => {
  test('renders 3-panel layout with header, main area, and status footer', async ({ page }) => {
    await gotoObserve(page)

    // Page header
    await expect(page.getByRole('heading', { name: AGENT_NAME })).toBeVisible()
    await expect(page.getByRole('link', { name: 'Observe', exact: true })).toBeVisible()

    // Left panel: Explorer with Follow toggle
    await expect(page.getByText('Explorer')).toBeVisible()
    await expect(page.getByRole('button', { name: /Following|Browse/ })).toBeVisible()

    // Center panel: Editor tabs
    await expect(page.getByRole('button', { name: /file/ }).first()).toBeVisible()
    await expect(page.getByRole('button', { name: /Diff/ })).toBeVisible()

    // Status footer: should have token count and session count
    await expect(page.getByText(/Token:/)).toBeVisible()
    await expect(page.getByText(/\d+ session/)).toBeVisible()
  })

  test('breadcrumb navigates back to observe list', async ({ page }) => {
    await gotoObserve(page)
    const observeLink = page.getByRole('link', { name: 'Observe', exact: true })
    await expect(observeLink).toHaveAttribute('href', '/observe')
  })

  test('Manage button links to agent detail page', async ({ page }) => {
    await gotoObserve(page)
    // Wait for agent info to load (Manage button appears after agent data arrives)
    const manageBtn = page.getByRole('button', { name: 'Manage' })
    await expect(manageBtn).toBeVisible({ timeout: 10_000 })
    await manageBtn.click()
    await expect(page).toHaveURL(new RegExp(`/agents/${AGENT_NAME}`))
  })
})

// ---------------------------------------------------------------------------
// 2. Observe API Contract — verifies backend returns expected shapes
// ---------------------------------------------------------------------------

test.describe('Observe API contract', () => {
  test('GET /sessions returns array with expected fields', async ({ page }) => {
    await gotoObserve(page)
    const resp = await page.evaluate(async (agent) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/sessions`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      return { status: r.status, body: await r.json() }
    }, AGENT_NAME)

    expect(resp.status).toBe(200)
    expect(Array.isArray(resp.body)).toBe(true)
    expect(resp.body.length).toBeGreaterThan(0)

    const session = resp.body[0]
    expect(session).toHaveProperty('started_at')
    expect(session).toHaveProperty('message_count')
    expect(session).toHaveProperty('is_current')
    expect(session).toHaveProperty('filename')
  })

  test('GET /sessions/current returns NDJSON with valid message types', async ({ page }) => {
    await gotoObserve(page)
    const result = await page.evaluate(async (agent) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/sessions/current`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const raw = await r.text()
      const lines = raw.trim().split('\n')
      const parsed = lines
        .map((l: string) => { try { return JSON.parse(l) } catch { return null } })
        .filter(Boolean)
      return {
        status: r.status,
        contentType: r.headers.get('content-type'),
        lineCount: lines.length,
        parsedCount: parsed.length,
        types: parsed.map((m: Record<string, unknown>) => m.type),
        // Verify message envelope structure
        firstMessage: parsed.find((m: Record<string, unknown>) => m.type === 'message'),
      }
    }, AGENT_NAME)

    expect(result.status).toBe(200)
    expect(result.contentType).toContain('ndjson')
    expect(result.parsedCount).toBeGreaterThan(0)
    expect(result.parsedCount).toBe(result.lineCount) // no parse failures

    // Must have a session line
    expect(result.types).toContain('session')

    // Message envelope: type="message" with nested .message.role
    if (result.firstMessage) {
      expect(result.firstMessage).toHaveProperty('message')
      expect(result.firstMessage.message).toHaveProperty('role')
      expect(result.firstMessage.message).toHaveProperty('content')
      expect(['user', 'assistant', 'toolResult']).toContain(result.firstMessage.message.role)
    }
  })

  test('GET /workspace/tree returns file tree with root node', async ({ page }) => {
    await gotoObserve(page)
    const result = await page.evaluate(async (agent) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/workspace/tree`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      return { status: r.status, body: await r.json() }
    }, AGENT_NAME)

    expect(result.status).toBe(200)
    expect(result.body).toHaveProperty('name')
    expect(result.body).toHaveProperty('type', 'dir')
  })

  test('GET /workspace/file returns FileContentResponse with content+language', async ({ page }) => {
    await gotoObserve(page)

    // First get a file path from the tree
    const filePath = await page.evaluate(async (agent) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/workspace/tree`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const tree = await r.json()
      const firstFile = (tree.children || []).find((n: Record<string, string>) => n.type === 'file')
      return firstFile?.name
    }, AGENT_NAME)

    if (!filePath) {
      test.skip(true, 'No files in workspace yet')
      return
    }

    const result = await page.evaluate(async ({ agent, path }) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/workspace/file?path=${path}`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      return { status: r.status, body: await r.json() }
    }, { agent: AGENT_NAME, path: filePath })

    expect(result.status).toBe(200)
    expect(result.body).toHaveProperty('path')
    expect(result.body).toHaveProperty('content')
    expect(result.body).toHaveProperty('language')
    expect(typeof result.body.content).toBe('string')
    expect(result.body.content.length).toBeGreaterThan(0)
  })

  test('GET /workspace/diff returns DiffResultResponse', async ({ page }) => {
    await gotoObserve(page)
    const result = await page.evaluate(async (agent) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/workspace/diff`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      return { status: r.status, body: await r.json() }
    }, AGENT_NAME)

    expect(result.status).toBe(200)
    expect(result.body).toHaveProperty('diff')
    expect(result.body).toHaveProperty('files_changed')
    expect(result.body).toHaveProperty('insertions')
    expect(result.body).toHaveProperty('deletions')
    expect(typeof result.body.files_changed).toBe('number')
  })
})

// ---------------------------------------------------------------------------
// 3. File Explorer Panel
// ---------------------------------------------------------------------------

test.describe('File Explorer', () => {
  test('displays file tree after data loads', async ({ page }) => {
    await gotoObserve(page)
    // Wait for tree to populate (polling every 15s, but initial load is immediate)
    // At minimum the workspace root should have files after agent ran
    const fileItems = page.locator('[class*="fileTreeItem"]')
    await expect(fileItems.first()).toBeVisible({ timeout: 20_000 })
  })

  test('displays session list with at least one session', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Sessions section header should show count > 0
    const sessionsHeader = page.getByText('Sessions')
    await expect(sessionsHeader).toBeVisible()

    // At least one session item should be visible
    const sessionItems = page.locator('[class*="explorerSessionItem"]')
    await expect(sessionItems.first()).toBeVisible({ timeout: 15_000 })
  })

  test('sessions section is collapsible', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    const sessionItems = page.locator('[class*="explorerSessionItem"]')
    await expect(sessionItems.first()).toBeVisible({ timeout: 15_000 })

    // Click Sessions header to collapse
    await page.getByText('Sessions').click()
    await expect(sessionItems.first()).not.toBeVisible()

    // Click again to expand
    await page.getByText('Sessions').click()
    await expect(sessionItems.first()).toBeVisible()
  })

  test('Follow toggle switches between Following and Browse', async ({ page }) => {
    await gotoObserve(page)

    const btn = page.getByRole('button', { name: /Following|Browse/ })
    await expect(btn).toBeVisible()

    // Initial state should be "Following"
    await expect(btn).toHaveText(/Following/)

    // Click to switch to Browse
    await btn.click()
    await expect(btn).toHaveText(/Browse/)

    // Click again to switch back
    await btn.click()
    await expect(btn).toHaveText(/Following/)
  })
})

// ---------------------------------------------------------------------------
// 4. Editor Panel — file content and diff
// ---------------------------------------------------------------------------

test.describe('Editor Panel', () => {
  test('clicking a file loads its content in Monaco editor', async ({ page }) => {
    await gotoObserve(page)
    // Wait for file tree
    const fileItems = page.locator('[class*="fileTreeItem"]')
    await expect(fileItems.first()).toBeVisible({ timeout: 20_000 })

    // Find a .md file to click (likely to exist)
    const mdFile = fileItems.filter({ hasText: /\.md$/ }).first()
    if (!(await mdFile.isVisible())) {
      test.skip(true, 'No .md files in workspace')
      return
    }

    const fileName = await mdFile.textContent()
    await mdFile.click()

    // Tab should update to show the file name
    await expect(page.getByRole('button', { name: new RegExp(fileName!.trim()) })).toBeVisible()

    // Monaco editor should render (textbox with editor content)
    const editor = page.getByRole('textbox', { name: 'Editor content' })
    await expect(editor).toBeVisible({ timeout: 10_000 })
  })

  test('Diff tab shows diff view', async ({ page }) => {
    await gotoObserve(page)

    // Click Diff tab
    const diffTab = page.getByRole('button', { name: /Diff/ })
    await diffTab.click()

    // Should show either diff content or "No diff available"
    const diffView = page.locator('[class*="diffView"]')
    const noDiff = page.getByText('No diff available')
    await expect(diffView.or(noDiff)).toBeVisible({ timeout: 5_000 })
  })

  test('switching between Code and Diff tabs preserves state', async ({ page }) => {
    await gotoObserve(page)
    const fileItems = page.locator('[class*="fileTreeItem"]')
    await expect(fileItems.first()).toBeVisible({ timeout: 20_000 })

    // Click a file
    const firstFile = fileItems.filter({ hasText: /\.md$/ }).first()
    if (!(await firstFile.isVisible())) {
      test.skip(true, 'No files in workspace')
      return
    }
    await firstFile.click()

    // Switch to Diff
    await page.getByRole('button', { name: /Diff/ }).click()
    const diffView = page.locator('[class*="diffView"]')
    const noDiff = page.getByText('No diff available')
    await expect(diffView.or(noDiff)).toBeVisible()

    // Switch back to Code — editor should still have content
    await page.getByRole('button', { name: /file/ }).first().click()
    await expect(page.getByRole('textbox', { name: 'Editor content' })).toBeVisible({ timeout: 5_000 })
  })
})

// ---------------------------------------------------------------------------
// 5. Conversation Panel — messages render correctly
// ---------------------------------------------------------------------------

test.describe('Conversation Panel', () => {
  test('displays conversation messages from current session', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Should have at least a session divider
    await expect(page.getByText(/Session started/)).toBeVisible({ timeout: 10_000 })
  })

  test('renders model badge', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Model change badge should show the model name
    await expect(page.getByText(/Model:/).first()).toBeVisible({ timeout: 10_000 })
  })

  test('renders user message bubble', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // There should be at least one "User" label
    await expect(page.getByText('User').first()).toBeVisible({ timeout: 10_000 })
  })

  test('renders assistant message with tool calls', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // There should be at least one "Assistant" label
    await expect(page.getByText('Assistant').first()).toBeVisible({ timeout: 10_000 })

    // Tool calls should render with wrench icon
    const toolCards = page.locator('[class*="toolCallCard"]')
    // Agent always calls at least one tool
    await expect(toolCards.first()).toBeVisible({ timeout: 10_000 })
  })

  test('renders tool result with expand/collapse', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Tool result cards should exist
    const toolResults = page.locator('[class*="toolResult"]')
    await expect(toolResults.first()).toBeVisible({ timeout: 10_000 })

    // Click expand
    const expandBtn = page.getByRole('button', { name: /Expand/ }).first()
    if (await expandBtn.isVisible()) {
      await expandBtn.click()
      // Content should now be visible
      const resultContent = page.locator('[class*="toolResultContent"]')
      await expect(resultContent.first()).toBeVisible()

      // Click collapse
      await page.getByRole('button', { name: /Collapse/ }).first().click()
      await expect(resultContent.first()).not.toBeVisible()
    }
  })

  test('thinking block is collapsible', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    const thinkingBtn = page.getByRole('button', { name: /Thinking/ }).first()
    if (!(await thinkingBtn.isVisible({ timeout: 5_000 }).catch(() => false))) {
      test.skip(true, 'No thinking blocks in current session')
      return
    }

    // Initially collapsed — click to expand
    await thinkingBtn.click()
    const thinkingContent = page.locator('[class*="thinkingContent"]')
    await expect(thinkingContent.first()).toBeVisible()

    // Click again to collapse
    await thinkingBtn.click()
    await expect(thinkingContent.first()).not.toBeVisible()
  })

  test('clicking tool call navigates to the file in editor', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Find a tool call that has a file path (read/write tool)
    const toolCards = page.locator('[class*="toolCallCard"]')
    await expect(toolCards.first()).toBeVisible({ timeout: 10_000 })

    // Find one with a file path in args
    const toolWithPath = toolCards.filter({ hasText: /\.md|\.go|\.ts|\.json/ }).first()
    if (!(await toolWithPath.isVisible().catch(() => false))) {
      test.skip(true, 'No tool calls with file paths')
      return
    }

    await toolWithPath.click()

    // The editor tab should update to show a file name
    // (we just check the tab text changed from "No file")
    await expect(page.getByRole('button', { name: /file/ }).first()).not.toHaveText(/No file/, { timeout: 5_000 })
  })
})

// ---------------------------------------------------------------------------
// 6. Status Footer
// ---------------------------------------------------------------------------

test.describe('Status Footer', () => {
  test('shows status icon and text (not emoji)', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // The status footer text should use semi-icon SVGs, not emoji characters
    const footerText = await page.getByText(/\d+ session/).textContent()
    // Should not contain any emoji codepoints in the footer area
    expect(footerText).not.toMatch(/[\u{1F300}-\u{1F9FF}]/u)
  })

  test('shows session count matching session list', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Get session count from footer text like "1 session" or "5 sessions"
    const sessionText = page.getByText(/\d+ session/)
    await expect(sessionText).toBeVisible()
    const text = await sessionText.textContent()
    const footerCount = parseInt(text!.match(/(\d+)/)![1])

    // Get actual session count from API
    const apiCount = await page.evaluate(async (agent) => {
      const token = localStorage.getItem('jwt_token')
      const r = await fetch(`/api/agents/${agent}/observe/sessions`, {
        headers: { Authorization: `Bearer ${token}` },
      })
      const data = await r.json()
      return Array.isArray(data) ? data.length : 0
    }, AGENT_NAME)

    expect(footerCount).toBe(apiCount)
  })

  test('shows token count', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    await expect(page.getByText(/Token:/)).toBeVisible()
  })
})

// ---------------------------------------------------------------------------
// 7. Session History Navigation
// ---------------------------------------------------------------------------

test.describe('Session History', () => {
  test('clicking a historical session loads its messages', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    // Need at least 2 sessions to test history navigation
    const sessionItems = page.locator('[class*="explorerSessionItem"]')
    const count = await sessionItems.count()
    if (count < 2) {
      test.skip(true, 'Need at least 2 sessions to test history')
      return
    }

    // Click the second session (first non-current)
    const historicalSession = sessionItems.nth(1)
    await historicalSession.click()

    // Should show "Viewing session" banner with "Return to live" button
    await expect(page.getByText(/Viewing session/)).toBeVisible({ timeout: 5_000 })
    await expect(page.getByText('Return to live')).toBeVisible()
  })

  test('Return to live button restores live view', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    const sessionItems = page.locator('[class*="explorerSessionItem"]')
    const count = await sessionItems.count()
    if (count < 2) {
      test.skip(true, 'Need at least 2 sessions')
      return
    }

    // Click historical session
    await sessionItems.nth(1).click()
    await expect(page.getByText(/Viewing session/)).toBeVisible({ timeout: 5_000 })

    // Click "Return to live"
    await page.getByText('Return to live').click()

    // Banner should disappear
    await expect(page.getByText(/Viewing session/)).not.toBeVisible({ timeout: 5_000 })
  })
})

// ---------------------------------------------------------------------------
// 8. Icons — verify semi-icons, not emoji
// ---------------------------------------------------------------------------

test.describe('Icons are SVG (semi-icons), not emoji', () => {
  test('tool call cards use wrench SVG icon', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    const toolCard = page.locator('[class*="toolCallCard"]').first()
    await expect(toolCard).toBeVisible({ timeout: 10_000 })

    // Should contain an SVG (semi-icon renders as <svg> inside <span>)
    const svg = toolCard.locator('svg')
    await expect(svg.first()).toBeVisible()
  })

  test('tool result cards use tick/minus SVG icons', async ({ page }) => {
    await gotoObserve(page)
    await waitForDataLoad(page)

    const toolResult = page.locator('[class*="toolResultInner"]').first()
    await expect(toolResult).toBeVisible({ timeout: 10_000 })

    const svg = toolResult.locator('svg')
    await expect(svg.first()).toBeVisible()
  })

  test('file tree items use file/folder SVG icons', async ({ page }) => {
    await gotoObserve(page)
    const fileItems = page.locator('[class*="fileTreeItem"]')
    await expect(fileItems.first()).toBeVisible({ timeout: 20_000 })

    // First item should have an SVG icon
    const svg = fileItems.first().locator('svg')
    await expect(svg.first()).toBeVisible()
  })
})
