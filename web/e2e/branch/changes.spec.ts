import { expect, test } from '@playwright/test'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { branchWith, snapshotWith, streams } from '../support/fixtures.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, walkTabOrder } from '../support/tabwalk.ts'

// The Branch section's working tree: a file staged, a stage the server
// refuses, and names wider than the narrowest window.

// A working tree with a file wholly staged, one partly staged and one the
// index does not hold, on a branch with nothing to push, so each file's stage
// or unstage button, Stage all and the commit form are all on screen.
const workingTreeSnapshot = snapshotWith({
  branch: branchWith({
    name: 'fix/PROJ-1',
    head: 'abc1234',
    upstream: 'origin/fix/PROJ-1',
    push_remote: 'origin',
    base: 'origin/main',
  }),
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
})

for (const theme of themes) {
  test(`no accessibility violations in the working tree in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and the stage answered here, so a
    // file's outcome line is on screen beside the buttons and the form.
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await streams(page, workingTreeSnapshot)
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
    expect(await axeViolations(page), `${theme} / working tree`).toBe('')
  })
}

// refused is what a write gets when the server cannot complete it.
const refused = {
  status: 500,
  contentType: 'application/problem+json',
  body: JSON.stringify({
    type: 'https://jacob-delgado.github.io/workflow/docs/errors/#internal',
    title: 'Internal error',
    status: 500,
    detail:
      'the request could not be completed; try again, and run workflow doctor if it keeps failing',
    code: 'internal',
  }),
}

for (const theme of themes) {
  test(`no accessibility violations beside a refused write in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and a stage the server refuses.
    await pinTheme(page, theme)
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await streams(page, workingTreeSnapshot)
    await page.route('**/api/stage', (route) => route.fulfill(refused))
    await page.goto('/')
    await openSection(page, 'Branch')

    // Act: stage the untracked file, and let the refusal land.
    await page.getByRole('button', { name: 'Stage notes.txt' }).click()
    await expect(page.getByRole('alert')).toHaveText(/could not be completed/)

    // Assert: axe finds nothing on the refusal and the working tree around it.
    expect(await axeViolations(page), `${theme} / refused write`).toBe('')
  })
}

// A branch and a file whose names each hold a word wider than the content.
const unbrokenSnapshot = snapshotWith({
  branch: branchWith({
    name: 'fix/PROJ-1',
    head: 'abc1234',
    upstream: 'origin/redact_every_authorization_header_before_the_request_log_writes_it',
    base: 'origin/main',
  }),
  changes: {
    changes: [
      {
        path: 'internal/tui/testdata/TestScreenDrawsEveryPaneAtTheNarrowestWidthItAllows.golden',
        kind: 'modified',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
    ],
  },
})

// A source build's health: its version is a commit marked dirty, the widest
// the header draws.
const sourceBuild = {
  version: 'ddbb935d6c04-dirty',
  dry_run: false,
  forge_noun: 'pull request',
  forge_sigil: '#',
}

// widthDrawn is how wide an element is drawn.
function widthDrawn(element: Element): number {
  return element.getBoundingClientRect().width
}

// termsSlack is how much wider the terms' column is than the widest term in it.
function termsSlack(terms: Element[]): number {
  const column = Math.max(...terms.map((term) => term.getBoundingClientRect().width))
  const words = terms.map((term) => {
    const range = document.createRange()
    range.selectNodeContents(term)

    return range.getBoundingClientRect().width
  })

  return column - Math.max(...words)
}

test('the branch and the header fit 320 px, wrapping a name wider than the content', async ({
  page,
}) => {
  // Arrange: the stream's branch and working tree, and a source build's
  // version in the header, at the narrowest width a page must reflow to.
  await streams(page, unbrokenSnapshot)
  await page.route('**/api/health', (route) => route.fulfill({ json: sourceBuild }))
  await page.setViewportSize({ width: 320, height })
  await page.goto('/')

  // Act
  await openSection(page, 'Branch')

  // Assert: nothing scrolls sideways; the file's path takes a line of its
  // own; and the terms' column is as wide as its widest term.
  const path = page.getByText(/^internal\/tui\/testdata/)
  const row = page.getByRole('listitem').filter({ has: path })
  await expect(path).toBeVisible()
  expect(await page.evaluate(sidewaysScrollers)).toEqual([])
  expect(await path.evaluate(widthDrawn), 'the path takes the row').toBeGreaterThanOrEqual(
    (await row.evaluate(widthDrawn)) - 1,
  )
  expect(await page.getByRole('term').evaluateAll(termsSlack), 'the terms column').toBeLessThan(1)
})

for (const width of widths) {
  test(`a refused write fits ${String(width)} px, every control reached in view`, async ({
    page,
  }) => {
    // Arrange: the stream's working tree, and a stage the server refuses.
    await streams(page, unbrokenSnapshot)
    await page.route('**/api/stage', (route) => route.fulfill({ status: 500 }))
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.setViewportSize({ width, height })
    await page.goto('/')
    await openSection(page, 'Branch')

    // Act: stage the file, let the refusal land, and Tab once round the page.
    await page.getByRole('button', { name: /^Stage internal\/tui/ }).click()
    await expect(page.getByRole('alert')).toBeVisible()
    const { missed, hidden } = await walkTabOrder(page)

    // Assert: nothing scrolls sideways, nor the page down; and Tab reaches
    // every drawn control beside the refusal, each in view.
    expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
    expect(missed, 'never reached by Tab').toEqual([])
    expect(hidden, 'out of view with focus').toEqual([])
  })
}
