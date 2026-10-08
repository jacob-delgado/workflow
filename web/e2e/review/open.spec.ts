import type { Page } from '@playwright/test'
import type { PullRequestDraft } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expect, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// Opening the branch's pull request from the Review section: the form it is
// composed in, and the offers once it is open.

// noPullYet is a branch with no pull request yet.
const noPullYet = snapshotWith()

const openedPull = {
  number: 7,
  url: 'https://forge.example.com/pull/7',
  title: 'fix: redact tokens before they reach the request log',
  state: 'open',
  draft: false,
  approvals: 0,
  changes_requested: false,
  mergeable: 'unknown',
}

// The pull request the branch's work composes, as the draft read answers it.
const pullDraft = {
  title: openedPull.title,
  body: 'Redacts the Authorization header.',
  base: 'main',
  head: 'fix/PROJ-1',
  draft: false,
  needs_push: false,
  reviewers: ['ana', 'acme/control-plane'],
  templates: [],
  template: '',
} satisfies PullRequestDraft

// opensReview opens the Review section in a theme, in a window of a width, on a
// branch with no pull request yet and the draft answered here.
async function opensReview(page: Page, theme: string, width: number): Promise<void> {
  await pinTheme(page, theme)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.setViewportSize({ width, height })
  await streams(page, noPullYet)
  await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
  await page.goto('/')
  await openSection(page, 'Review')
}

// The pull request form's fields and buttons, named as a walk names them.
const formControls = [
  'Title',
  'Base branch',
  'Reviewers',
  'Assignees',
  'Labels',
  'Description',
  'Open as a draft',
  'Cancel',
  'Open pull request',
]

for (const theme of themes) {
  for (const width of widths) {
    test(`the pull request form fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await opensReview(page, theme, width)

      // Act: compose the pull request, which opens as a form to edit.
      await page.getByRole('button', { name: 'Open a pull request' }).click()
      await expect(page.getByRole('form', { name: 'Open a pull request' })).toBeVisible()

      // Assert: Tab reaches each of the form's fields, and it is clean.
      await expectReachableAndClean(page, { reaches: formControls })
    })
  }
}

for (const theme of themes) {
  for (const width of widths) {
    test(`the offers after opening fit ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: the open and the link answered here, so the open's outcome,
      // its offers and a done offer's status line are all on screen.
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
      await opensReview(page, theme, width)
      await page.getByRole('button', { name: 'Open a pull request' }).click()
      await page.getByRole('button', { name: 'Open pull request' }).click()

      // Act: link it, leaving the move offered beside what the link said.
      await page.getByRole('button', { name: 'Link it on PROJ-1' }).click()
      await expect(page.getByText('Linked #7 on PROJ-1.')).toBeVisible()

      // Assert: Tab reaches the move still offered, and it is clean.
      await expectReachableAndClean(page, { reaches: ['Move PROJ-1 to In Review'] })
    })
  }
}
