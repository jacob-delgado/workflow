import type { Page } from '@playwright/test'
import type { BranchIssuePreview } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { branchWith, expect, problem, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// The Branch section's Link an issue, for work begun outside workflow on a
// branch whose name names no issue: the form that asks which, the pull
// request's description shown before it changes, and a key that names none.

const branch = branchWith({
  name: 'spike/search-speed',
  head: 'abc1234',
  upstream: 'origin/spike/search-speed',
  push_remote: 'origin',
  base: 'origin/main',
})

const snapshot = snapshotWith({ branch })

// The description linking PROJ-7 would leave on the branch's pull request.
const preview = {
  key: 'PROJ-7',
  pull: 12,
  body: 'Speeds up search.\n\nPROJ-7',
  changes: true,
} satisfies BranchIssuePreview

// noSuchIssue is the answer to a key that names no issue.
const noSuchIssue = problem('not_found', 'PROJ-999 names no issue')

// opensLinkForm opens the Branch section on the unlinked branch, and its form.
async function opensLinkForm(page: Page): Promise<void> {
  await streams(page, snapshot)
  await page.goto('/')
  await openSection(page, 'Branch')
  await page.getByRole('button', { name: 'Link an issue' }).click()
  await expect(page.getByRole('textbox', { name: 'Issue' })).toBeFocused()
}

test('shows the description first, then links and updates the pull request', async ({ page }) => {
  // Arrange
  // The preview and the link answered here, the link's body kept.
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
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await page.route('**/api/branch/issue/preview**', (route) => route.fulfill({ json: preview }))
      await opensLinkForm(page)

      // Act
      // Ask for PROJ-7, and let its description land.
      await page.getByRole('textbox', { name: 'Issue' }).fill('PROJ-7')
      await page.getByRole('button', { name: 'Link', exact: true }).click()
      await expect(page.getByRole('button', { name: 'Link and update #12' })).toBeVisible()

      // Assert
      // Tab reaches the form's controls, and it is clean.
      await expectReachableAndClean(page, {
        reaches: ['Issue', 'Link and update #12', 'Link only', 'Cancel'],
      })
    })
  }
}
