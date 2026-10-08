import type { Page } from '@playwright/test'
import type { Snapshot } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { branchWith, expect, problem, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean, sidewaysScrollers } from '../support/reachable.ts'

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

// opensBranch opens the Branch section in a theme, in a window of a width, on
// the working tree streamed.
async function opensBranch(
  page: Page,
  { theme, width, snapshot }: { theme: string; width: number; snapshot: Snapshot },
): Promise<void> {
  await pinTheme(page, theme)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.setViewportSize({ width, height })
  await streams(page, snapshot)
  await page.goto('/')
  await openSection(page, 'Branch')
}

for (const theme of themes) {
  for (const width of widths) {
    test(`a staged file fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      // The stage answered here, so a file's outcome line is on screen beside
      // the buttons and the commit form.
      await page.route('**/api/stage', (route) =>
        route.fulfill({ json: workingTreeSnapshot.changes }),
      )
      await opensBranch(page, { theme, width, snapshot: workingTreeSnapshot })

      // Act
      await page.getByRole('button', { name: 'Stage notes.txt' }).click()
      await expect(page.getByText('Staged notes.txt.')).toBeVisible()

      // Assert
      await expectReachableAndClean(page, {
        reaches: ['Stage all', 'Commit staged changes'],
      })
    })
  }
}

// refused is what a write gets when the server cannot complete it.
const refused = problem(
  'internal',
  'the request could not be completed; try again, and run workflow doctor if it keeps failing',
)

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
  // Arrange
  // 320 px is the narrowest width a page must reflow to, here with a source
  // build's version in the header.
  await streams(page, unbrokenSnapshot)
  await page.route('**/api/health', (route) => route.fulfill({ json: sourceBuild }))
  await page.setViewportSize({ width: 320, height })
  await page.goto('/')

  // Act
  await openSection(page, 'Branch')

  // Assert
  const path = page.getByText(/^internal\/tui\/testdata/)
  const row = page.getByRole('listitem').filter({ has: path })
  await expect(path).toBeVisible()
  expect(await page.evaluate(sidewaysScrollers)).toEqual([])
  expect(await path.evaluate(widthDrawn), 'the path takes the row').toBeGreaterThanOrEqual(
    (await row.evaluate(widthDrawn)) - 1,
  )
  expect(await page.getByRole('term').evaluateAll(termsSlack), 'the terms column').toBeLessThan(1)
})

for (const theme of themes) {
  for (const width of widths) {
    test(`a refused write fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      // The refused file's name is wider than the content.
      await page.route('**/api/stage', (route) => route.fulfill(refused))
      await opensBranch(page, { theme, width, snapshot: unbrokenSnapshot })

      // Act
      await page.getByRole('button', { name: /^Stage internal\/tui/ }).click()
      await expect(page.getByRole('alert')).toHaveText(/could not be completed/)

      // Assert
      await expectReachableAndClean(page)
    })
  }
}
