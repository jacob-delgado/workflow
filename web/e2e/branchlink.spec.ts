import { expect, test, type Page } from '@playwright/test'
import type { Branch, BranchIssuePreview, Snapshot } from '../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from './cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, streams, walkTabOrder } from './tabwalk.ts'

// The Branch section's Link an issue, for work begun outside workflow on a
// branch whose name names no issue: the form that asks which, the pull
// request's description shown before it changes, and a key that names none.

const branch = {
  name: 'spike/search-speed',
  issue_link: '',
  detached: false,
  head: 'abc1234',
  upstream: 'origin/spike/search-speed',
  push_remote: 'origin',
  ahead: 0,
  behind: 0,
  base: 'origin/main',
  commits: [],
} satisfies Branch

const snapshot = {
  issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
  branch,
  changes: { changes: [] },
  review: { found: false, announced: false },
  messaging: {
    kind: 'slack',
    service: 'Slack',
    configured: false,
    channel: '',
    channels: [],
    author: '',
  },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 0,
  tasks: { available: true, reason: '', linked: [] },
  here: '/home/ana/src/api',
} satisfies Snapshot

// The description linking PROJ-7 would leave on the branch's pull request.
const preview = {
  key: 'PROJ-7',
  pull: 12,
  body: 'Speeds up search.\n\nPROJ-7',
  changes: true,
} satisfies BranchIssuePreview

// noSuchIssue is the answer to a key that names no issue.
const noSuchIssue = {
  status: 404,
  contentType: 'application/problem+json',
  body: JSON.stringify({
    type: 'https://jacob-delgado.github.io/workflow/docs/errors/#not_found',
    title: 'Not found',
    status: 404,
    detail: 'PROJ-999 names no issue',
    code: 'not_found',
  }),
}

// opensLinkForm opens the Branch section on the unlinked branch, and its form.
async function opensLinkForm(page: Page): Promise<void> {
  await streams(page, snapshot)
  await page.goto('/')
  await openSection(page, 'Branch')
  await page.getByRole('button', { name: 'Link an issue' }).click()
  await expect(page.getByRole('textbox', { name: 'Issue' })).toBeFocused()
}

test('shows the description first, then links and updates the pull request', async ({ page }) => {
  // Arrange: the preview and the link answered here, the link's body kept.
  await page.route('**/api/branch/issue/preview**', (route) => route.fulfill({ json: preview }))
  const sent: unknown[] = []
  await page.route('**/api/branch/issue', (route) => {
    sent.push(route.request().postDataJSON())

    return route.fulfill({ json: { ...branch, issue_link: 'PROJ-7' } })
  })
  await opensLinkForm(page)
  await page.getByRole('textbox', { name: 'Issue' }).fill('PROJ-7')
  await page.getByRole('button', { name: 'Link', exact: true }).click()
  await expect(page.getByText("#12's description becomes:")).toBeVisible()

  // Act
  await page.getByRole('button', { name: 'Link and update #12' }).click()

  // Assert
  await expect(page.getByText('Linked spike/search-speed to PROJ-7.')).toBeVisible()
  expect(sent).toEqual([{ key: 'PROJ-7', update_pull: true }])
})

test('a key that names no issue says so, and Cancel closes the form', async ({ page }) => {
  // Arrange
  await page.route('**/api/branch/issue/preview**', (route) => route.fulfill(noSuchIssue))
  await opensLinkForm(page)
  await page.getByRole('textbox', { name: 'Issue' }).fill('PROJ-999')
  await page.getByRole('button', { name: 'Link', exact: true }).click()
  await expect(page.getByRole('alert')).toHaveText(/names no issue/)

  // Act
  await page.getByRole('button', { name: 'Cancel' }).click()

  // Assert
  await expect(page.getByRole('button', { name: 'Link an issue' })).toBeVisible()
  await expect(page.getByRole('textbox', { name: 'Issue' })).toBeHidden()
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the description to link fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: the form, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await page.route('**/api/branch/issue/preview**', (route) => route.fulfill({ json: preview }))
      await opensLinkForm(page)

      // Act: ask for PROJ-7, let its description land, and Tab once round.
      await page.getByRole('textbox', { name: 'Issue' }).fill('PROJ-7')
      await page.getByRole('button', { name: 'Link', exact: true }).click()
      await expect(page.getByRole('button', { name: 'Link and update #12' })).toBeVisible()
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways, nor the page down; Tab reaches the
      // form's controls and every other drawn one, each in view; and axe
      // finds nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
      expect(reached, 'reached by Tab').toEqual(
        expect.arrayContaining(['Issue', 'Link and update #12', 'Link only', 'Cancel']),
      )
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
