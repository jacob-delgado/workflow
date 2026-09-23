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
      route.fulfill({
        json: { version: '1.2.3', dry_run: true, forge_noun: 'pull request', forge_sigil: '#' },
      }),
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

const openedPull = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'fix: redact tokens before they reach the request log',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown',
}

for (const theme of themes) {
  test(`no accessibility violations in the offers after opening in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft, the open and
    // the link answered here, so the open's outcome, its offers and a done
    // offer's status line are all on screen for the scan.
    await page.addInitScript((value) => {
      window.localStorage.setItem('workflow-theme', value)
    }, theme)
    await page.route('**/api/events**', (route) =>
      route.fulfill({
        contentType: 'text/event-stream',
        body: `event: snapshot\ndata: ${JSON.stringify(issuesSnapshot)}\n\n`,
      }),
    )
    await page.route('**/api/pull-request/draft', (route) =>
      route.fulfill({
        json: {
          title: openedPull.title,
          body: 'Redacts the Authorization header.',
          base: 'main',
          head: 'fix/PROJ-1',
          draft: false,
          needs_push: false,
        },
      }),
    )
    await page.route('**/api/pull-request', (route) =>
      route.fulfill({
        json: {
          pull: openedPull,
          follow_ups: [
            { action: 'link', issue_key: 'PROJ-1' },
            { action: 'transition', issue_key: 'PROJ-1', status: 'In Review' },
          ],
        },
      }),
    )
    await page.route('**/api/issues/PROJ-1/link', (route) => route.fulfill({ json: openedPull }))
    await page.goto('/')
    await page
      .getByRole('navigation', { name: 'Sections' })
      .getByRole('button', { name: 'Review' })
      .click()
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await page.getByRole('button', { name: 'Open pull request' }).click()

    // Act: link it, leaving the move offered beside what the link said.
    await page.getByRole('button', { name: 'Link it on PROJ-1' }).click()
    await expect(page.getByText('Linked #7 on PROJ-1.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Move PROJ-1 to In Review' })).toBeVisible()

    // Assert: axe finds nothing on the outcome and its offers.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / offers: ${summary}`).toEqual([])
  })
}

// A working tree with a file wholly staged, one partly staged and one the
// index does not hold, on a branch with nothing to push, so each file's stage
// or unstage button, Stage all and the commit form are all on screen.
const workingTreeSnapshot = {
  ...issuesSnapshot,
  branch: {
    name: 'fix/PROJ-1',
    detached: false,
    head: 'abc1234',
    upstream: 'origin/fix/PROJ-1',
    ahead: 0,
    behind: 0,
    base: 'origin/main',
    commits: [],
  },
  changes: {
    changes: [
      {
        path: 'internal/wiring/reqlog.go',
        kind: 'modified',
        staged: true,
        has_unstaged: false,
        conflicted: false,
      },
      {
        path: 'internal/config/redact.go',
        kind: 'modified',
        staged: true,
        has_unstaged: true,
        conflicted: false,
      },
      {
        path: 'notes.txt',
        kind: 'untracked',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
    ],
  },
}

for (const theme of themes) {
  test(`no accessibility violations in the working tree in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and the stage answered here, so a
    // file's outcome line is on screen beside the buttons and the form.
    await page.addInitScript((value) => {
      window.localStorage.setItem('workflow-theme', value)
    }, theme)
    await page.route('**/api/events**', (route) =>
      route.fulfill({
        contentType: 'text/event-stream',
        body: `event: snapshot\ndata: ${JSON.stringify(workingTreeSnapshot)}\n\n`,
      }),
    )
    await page.route('**/api/stage', (route) =>
      route.fulfill({ json: workingTreeSnapshot.changes }),
    )
    await page.goto('/')
    await page
      .getByRole('navigation', { name: 'Sections' })
      .getByRole('button', { name: 'Branch' })
      .click()

    // Act: stage the untracked file.
    await page.getByRole('button', { name: 'Stage notes.txt' }).click()
    await expect(page.getByText('Staged notes.txt.')).toBeVisible()

    // Assert: axe finds nothing on the working tree and its commit form.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / working tree: ${summary}`).toEqual([])
  })
}
