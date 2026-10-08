import { expect, test } from '@playwright/test'
import type { PullRequestDraft } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { snapshotWith, streams } from '../support/fixtures.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, walkTabOrder } from '../support/tabwalk.ts'

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

for (const theme of themes) {
  test(`no accessibility violations in the pull request form in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft answered here.
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await streams(page, noPullYet)
    await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
    await page.goto('/')
    await openSection(page, 'Review')

    // Act: compose the pull request, which opens as a form to edit.
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await expect(page.getByRole('form', { name: 'Open a pull request' })).toBeVisible()

    // Assert: axe finds nothing on the form and its seven fields.
    expect(await axeViolations(page), `${theme} / pull request form`).toBe('')
  })
}

for (const theme of themes) {
  test(`no accessibility violations in the offers after opening in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft, the open and
    // the link answered here, so the open's outcome, its offers and a done
    // offer's status line are all on screen for the scan.
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await streams(page, noPullYet)
    await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
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
      .getByRole('button', { name: 'Review', exact: true })
      .click()
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await page.getByRole('button', { name: 'Open pull request' }).click()

    // Act: link it, leaving the move offered beside what the link said.
    await page.getByRole('button', { name: 'Link it on PROJ-1' }).click()
    await expect(page.getByText('Linked #7 on PROJ-1.')).toBeVisible()
    await expect(page.getByRole('button', { name: 'Move PROJ-1 to In Review' })).toBeVisible()

    // Assert: axe finds nothing on the outcome and its offers.
    expect(await axeViolations(page), `${theme} / offers`).toBe('')
  })
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

for (const width of widths) {
  test(`the pull request form fits ${String(width)} px, each field reached in view`, async ({
    page,
  }) => {
    // Arrange: a stream with no pull request yet, and the draft answered here.
    await streams(page, noPullYet)
    await page.route('**/api/pull-request/draft', (route) => route.fulfill({ json: pullDraft }))
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width, height })
    await page.goto('/')
    await openSection(page, 'Review')

    // Act: compose the pull request, and Tab once round the page.
    await page.getByRole('button', { name: 'Open a pull request' }).click()
    await expect(page.getByRole('form', { name: 'Open a pull request' })).toBeVisible()
    const { reached, missed, hidden } = await walkTabOrder(page)

    // Assert: nothing scrolls sideways, nor the page down; and Tab reaches
    // each of the form's fields and every other drawn control, in view.
    expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
    expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(formControls))
    expect(missed, 'never reached by Tab').toEqual([])
    expect(hidden, 'out of view with focus').toEqual([])
  })
}
