import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

// Every section, in both themes: a light theme is only real once its contrast
// holds up, so the scan runs the whole cockpit in each. The section labels are
// the nav buttons' accessible names and the content heading's text.
const themes = ['dark', 'light'] as const
const sectionNames = ['Issues', 'Branch', 'Review', 'Messaging', 'Settings']

// Scan the resting state, not mid-animation frames: reduced motion collapses
// transitions to instant, so axe never samples a half-faded element (whose
// transient blended colors are a false contrast failure).
test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})

// scan returns the WCAG A/AA violations axe finds on whatever is on screen.
async function scan(page: Page) {
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  return violations
}

for (const theme of themes) {
  test(`no accessibility violations across the sections in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: pin the theme before the app paints, so the whole run is in it,
    // and answer the health read as a --dry-run server would, so the read-only
    // banner is on screen for every scan (the hermetic server has no API).
    await page.addInitScript((value) => {
      window.localStorage.setItem('workflow-theme', value)
    }, theme)
    await page.route('**/api/health', (route) =>
      route.fulfill({ json: { version: '1.2.3', dry_run: true } }),
    )
    await page.goto('/')
    await expect(page.getByText(/every write is held back/i)).toBeVisible()

    const nav = page.getByRole('navigation', { name: 'Sections' })

    for (const name of sectionNames) {
      // Act: open the section and let its heading settle.
      await nav.getByRole('button', { name, exact: true }).click()
      await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()

      // Assert: axe finds nothing on this section in this theme.
      const violations = await scan(page)
      const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
      expect(violations, `${theme} / ${name}: ${summary}`).toEqual([])
    }
  })
}

// A snapshot with more issues than its page carries, so the list, its view
// select, filter and "Load more" are all on screen for the scan.
const issuesSnapshot = {
  issues: {
    total: 3,
    start_at: 0,
    issues: [
      {
        key: 'PROJ-1',
        summary: 'Redact tokens before they reach the request log',
        status: 'In Progress',
        status_category: 'indeterminate',
        type: 'Bug',
        priority: 'High',
      },
      {
        key: 'PROJ-2',
        summary: 'Document the token flow',
        status: 'To Do',
        status_category: 'new',
        type: 'Task',
      },
    ],
  },
  branch: {
    name: '',
    detached: false,
    head: '',
    upstream: '',
    ahead: 0,
    behind: 0,
    base: '',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
}

const issueDetail = {
  ...issuesSnapshot.issues.issues[0],
  reporter: 'Ana Lopez',
  assignee: 'octocat',
  description: 'The request log records every header, so a bearer token lands in it.',
  comments: [{ author: 'Sam Ortiz', body: "Repro'd on main.", created: '2026-09-18T15:04:00Z' }],
  comment_total: 3,
  url: 'https://jira.example.com/browse/PROJ-1',
}

for (const theme of themes) {
  test(`no accessibility violations in the issue list and detail in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the hermetic server has no API, so the stream, the views and the
    // issue are answered here — enough for the list's controls and the detail.
    await page.addInitScript((value) => {
      window.localStorage.setItem('workflow-theme', value)
    }, theme)
    await page.route('**/api/events**', (route) =>
      route.fulfill({
        contentType: 'text/event-stream',
        body: `event: snapshot\ndata: ${JSON.stringify(issuesSnapshot)}\n\n`,
      }),
    )
    await page.route('**/api/views', (route) =>
      route.fulfill({
        json: {
          views: [
            { name: 'Assigned to me', jql: 'assignee = currentUser()' },
            { name: 'Team bugs', jql: 'type = Bug' },
          ],
        },
      }),
    )
    await page.route('**/api/issues/PROJ-1', (route) => route.fulfill({ json: issueDetail }))
    await page.goto('/')
    await expect(page.getByRole('button', { name: /load more/i })).toBeVisible()
    await expect(page.getByRole('combobox', { name: 'View' })).toBeVisible()

    // Act: open the first issue and let its detail land.
    await page.getByRole('button', { name: /redact tokens/i }).click()
    await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()

    // Assert: axe finds nothing on the list beside the open detail.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / issues: ${summary}`).toEqual([])
  })
}
