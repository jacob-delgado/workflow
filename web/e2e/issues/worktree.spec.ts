import { expect, test, type Page } from '@playwright/test'
import type { CreatedWorktree, Snapshot } from '../../src/api/generated/types.gen.ts'
import { mockRepositories } from '../../src/dev/mockRepositories.ts'
import { height, pinTheme, themes, widths } from '../cockpit.ts'
import { axeViolations, sidewaysScrollers, streams, walkTabOrder } from '../tabwalk.ts'

// Starting work on an issue in a new worktree from its detail, and the switch
// to the worktree offered once it is made.

const issue = {
  key: 'PROJ-7',
  tracker: 'jira',
  summary: 'Limit the rate of token refreshes',
  status: 'To Do',
  status_category: 'new',
  type: 'Story',
  priority: 'Medium',
} as const

const snapshot = {
  issues: { total: 1, start_at: 0, unavailable: [], issues: [issue] },
  branch: {
    name: 'main',
    issue_link: '',
    detached: false,
    head: '300a7be',
    upstream: 'origin/main',
    push_remote: 'origin',
    ahead: 0,
    behind: 0,
    base: 'origin/main',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: false, announced: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 0,
  tasks: { available: true, reason: '', linked: [] },
  here: '/home/ana/src/api',
} satisfies Snapshot

const made: CreatedWorktree = {
  dir: '/home/ana/src/api-feat-PROJ-7-rate-limits',
  shown: '~/src/api-feat-PROJ-7-rate-limits',
  branch: 'feat/PROJ-7-rate-limits',
}

// opensIssue serves PROJ-7, not yet started, answers a worktree made for it and
// a switch to it, and opens its detail. It answers each directory switched to.
async function opensIssue(page: Page): Promise<string[]> {
  const switched: string[] = []
  await streams(page, snapshot)
  await page.route('**/api/issues/PROJ-7', (route) =>
    route.fulfill({
      json: {
        ...issue,
        reporter: 'Ana Lopez',
        assignee: 'octocat',
        description: 'Refreshes stampede the token service.',
        comments: [],
        comment_total: 0,
        url: 'https://jira.example.com/browse/PROJ-7',
      },
    }),
  )
  await page.route('**/api/worktrees', (route) => route.fulfill({ json: made }))
  await page.route('**/api/repositories/here', (route) => {
    const asked = route.request().postDataJSON() as { dir: string }
    switched.push(asked.dir)
    const after = mockRepositories()

    return route.fulfill({
      json: { ...after, here: { ...after.here, dir: made.dir, shown: made.shown } },
    })
  })
  await page.goto('/')
  await page.getByRole('button', { name: /limit the rate/i }).click()
  await expect(page.getByRole('button', { name: 'Start work in a new worktree' })).toBeVisible()

  return switched
}

test('work started in a new worktree offers to switch to it', async ({ page }) => {
  // Arrange
  const switched = await opensIssue(page)
  await page.getByRole('button', { name: 'Start work in a new worktree' }).click()
  const offer = page.getByRole('region', { name: `Started PROJ-7 in ${made.shown}` })
  await expect(offer).toBeVisible()

  // Act: switch to it
  await offer.getByRole('button', { name: 'Switch to it' }).click()

  // Assert: it asks first, and nothing is switched yet
  const confirm = page.getByRole('region', { name: `Switch to ${made.shown}?` })
  await expect(confirm).toBeVisible()
  expect(switched).toEqual([])

  // Act: confirm the switch
  await confirm.getByRole('button', { name: 'Switch', exact: true }).click()

  // Assert: switched
  await expect(page.getByText(`Switched to ${made.shown}.`)).toBeVisible()
  expect(switched).toEqual([made.dir])
})

test('a fetch that fails offers to branch from what you have', async ({ page }) => {
  // Arrange: the first ask's fetch fails; the second, without one, makes it.
  await opensIssue(page)
  const asked: unknown[] = []
  await page.route('**/api/worktrees', (route) => {
    asked.push(route.request().postDataJSON())
    if (asked.length === 1) {
      return route.fulfill({
        status: 502,
        contentType: 'application/problem+json',
        json: {
          type: 'https://jacob-delgado.github.io/workflow/docs/errors/#fetch-failed',
          title: 'Fetch failed',
          status: 502,
          code: 'fetch_failed',
          detail: 'origin could not be fetched, so nothing was made for PROJ-7',
        },
      })
    }

    return route.fulfill({ json: made })
  })
  await page.getByRole('button', { name: 'Start work in a new worktree' }).click()
  await expect(page.getByRole('alert')).toContainText('could not be fetched')

  // Act
  await page.getByRole('button', { name: 'Branch from what you have' }).click()

  // Assert
  await expect(page.getByRole('region', { name: `Started PROJ-7 in ${made.shown}` })).toBeVisible()
  expect(asked).toEqual([
    { issue_key: 'PROJ-7', fetch: true },
    { issue_key: 'PROJ-7', fetch: false },
  ])
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the offer to switch to a new worktree fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: a worktree made, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await opensIssue(page)
      await page.getByRole('button', { name: 'Start work in a new worktree' }).click()
      await expect(page.getByRole('button', { name: 'Switch to it' })).toBeVisible()

      // Act: Tab once round the page.
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways; Tab reaches the switch, in view; and
      // axe finds nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(['Switch to it']))
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
