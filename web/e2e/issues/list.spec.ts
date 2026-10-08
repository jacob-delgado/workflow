import type { Issue } from '../../src/api/generated/types.gen.ts'
import { height, openCockpit, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expect, issuesOf, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// The issue list: its controls and an open issue's detail beside it, a later
// page of the view loaded into it, and its rows, each summary on one line and
// every row at one right edge.

// redactTokens is the issue the list opens, in progress.
const redactTokens = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Redact tokens before they reach the request log',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
} satisfies Issue

// A snapshot with more issues than its page carries, so the list, its view
// select, filter, Where buttons and "Load more" are all on screen for the scan.
const issuesSnapshot = snapshotWith({
  issues: issuesOf(
    [
      redactTokens,
      {
        key: 'PROJ-2',
        tracker: 'jira',
        summary: 'Document the token flow',
        status: 'To Do',
        status_category: 'new',
        type: 'Task',
      },
    ],
    3,
  ),
})

const issueDetail = {
  ...redactTokens,
  reporter: 'Ana Lopez',
  assignee: 'octocat',
  description: 'The request log records every header, so a bearer token lands in it.',
  comments: [{ author: 'Sam Ortiz', body: "Repro'd on main.", created: '2026-09-18T15:04:00Z' }],
  comment_total: 3,
  url: 'https://jira.example.com/browse/PROJ-1',
}

for (const theme of themes) {
  for (const width of widths) {
    test(`the issue list and detail fit ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await streams(page, issuesSnapshot)
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

      // Act
      // A place is chosen, so a pressed Where button is in the check.
      const inProgress = page
        .getByRole('group', { name: 'Filter' })
        .getByRole('button', { name: /^In Progress/ })
      await inProgress.click()
      await expect(inProgress).toHaveAttribute('aria-pressed', 'true')
      await page.getByRole('button', { name: /redact tokens/i }).click()
      await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()

      // Assert
      await expectReachableAndClean(page, { reaches: ['View'] })
    })
  }
}

const issuesTotal = 12

// streamedIssue is one of the issues the stream carries, or a later page adds.
function streamedIssue(number: number): Issue {
  return {
    key: `PROJ-${String(number)}`,
    tracker: 'jira',
    summary: `Issue number ${String(number)} of the view`,
    status: 'To Do',
    status_category: 'new',
    type: 'Task',
  }
}

// A stream carrying the first eight of the view's twelve issues, so the list
// offers to load more.
const pagedSnapshot = snapshotWith({
  issues: issuesOf([1, 2, 3, 4, 5, 6, 7, 8].map(streamedIssue), issuesTotal),
})

test('a loaded page hands focus to its first issue, in view in the list, at 640 px', async ({
  page,
}) => {
  // Arrange
  await streams(page, pagedSnapshot)
  await page.route(/\/api\/issues\?/, (route) =>
    route.fulfill({
      json: {
        total: issuesTotal,
        start_at: 8,
        unavailable: [],
        issues: [9, 10, 11, 12].map(streamedIssue),
      },
    }),
  )
  // 700 px tall, so the list scrolls in its pane.
  await page.setViewportSize({ width: 640, height: 700 })
  await page.goto('/')

  // Act
  await page.getByRole('button', { name: 'Load more' }).click()

  // Assert
  const added = page.getByRole('button', { name: /^PROJ-9/ })
  await expect(added).toBeFocused()
  await expect(added).toBeInViewport({ ratio: 1 })
})

test(
  'a wide window sets each issue summary in the list on one line',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await openCockpit(page, { width: 1440, height }, 'dark')

    // Act
    const lines = await page
      .getByRole('list', { name: 'Issues' })
      .getByRole('button')
      .filter({ hasNotText: 'Switch branch' })
      .evaluateAll((rows) =>
        rows.map((row) => {
          const summary = row.lastElementChild ?? row

          return Math.round(
            summary.getBoundingClientRect().height /
              parseFloat(getComputedStyle(summary).lineHeight),
          )
        }),
      )

    // Assert
    expect(lines.length).toBeGreaterThan(1)
    expect(lines).toEqual(lines.map(() => 1))
  },
)

for (const width of [640, 1440]) {
  test(
    `every issue row ends at one right edge at ${String(width)} px, with or without Switch branch`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width, height }, 'dark')
      const list = page.getByRole('list', { name: 'Issues' })
      await expect(list.getByRole('button', { name: /^Switch branch/ }).first()).toBeVisible()

      // Act
      const edges = await list
        .getByRole('button')
        .filter({ hasNotText: 'Switch branch' })
        .evaluateAll((rows) => rows.map((row) => Math.round(row.getBoundingClientRect().right)))

      // Assert
      expect(new Set(edges).size, `right edges ${edges.join(', ')}`).toBe(1)
    },
  )
}
