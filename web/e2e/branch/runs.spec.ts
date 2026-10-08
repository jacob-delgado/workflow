import type { Page } from '@playwright/test'
import type { HookSetup, RunEvent } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { branchWith, expect, snapshotWith, streams, test } from '../support/fixtures.ts'
import { expectReachableAndClean } from '../support/reachable.ts'

// The Branch section's git runs — pre-commit, a rebase, an amend, a fixup —
// streamed as they go, each that rewrites history behind a last look; a
// changed file's diff; and setting up lefthook.

const snapshot = snapshotWith({
  branch: branchWith({
    name: 'fix/PROJ-1-redact',
    head: 'b2b2b2b',
    push_remote: 'origin',
    ahead: 2,
    base: 'origin/main',
    commits: [
      { hash: 'a1a1a1a1', subject: 'fix: redact tokens', unpushed: true },
      { hash: 'b2b2b2b2', subject: 'test: prove it', unpushed: true },
    ],
  }),
  changes: {
    changes: [
      { path: 'log.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
    ],
  },
  hooks_unmanaged: 1,
})

const offer: HookSetup = {
  offered: true,
  hooks: [{ name: 'pre-commit', lines: 3 }],
  config: 'pre-commit:\n  jobs:\n    - name: go-vet\n      run: go vet ./...\n',
  scripts: 0,
}

// ndjson is a run's stream: it starts, writes two lines and passes.
function ndjson(kind: string, title: string, outcome: string): string {
  const events: RunEvent[] = [
    { run: { kind, title, state: 'in_progress', outcome: '', lines: [] } },
    { line: 'lefthook v1.11.0  hook: pre-commit' },
    { line: 'go vet ./... passed' },
    {
      run: {
        kind,
        title,
        state: 'succeeded',
        outcome,
        lines: ['lefthook v1.11.0  hook: pre-commit', 'go vet ./... passed'],
      },
    },
  ] as RunEvent[]

  return events.map((event) => JSON.stringify(event)).join('\n') + '\n'
}

// opensBranch serves the branch, its runs, its diff and the lefthook offer,
// and opens the Branch section. It answers each run asked for.
async function opensBranch(page: Page): Promise<unknown[]> {
  const asked: unknown[] = []
  await streams(page, snapshot)
  await page.route('**/api/runs', (route) => {
    const request = route.request().postDataJSON() as { kind: string }
    asked.push(request)

    return route.fulfill({
      contentType: 'application/x-ndjson',
      body: ndjson(
        request.kind,
        request.kind === 'pre_commit' ? 'pre-commit' : 'git rebase',
        'It went through.',
      ),
    })
  })
  await page.route('**/api/changes/diff**', (route) =>
    route.fulfill({
      json: { path: 'log.go', lines: ['--- a/log.go', '+++ b/log.go', '-old', '+new'] },
    }),
  )
  await page.route('**/api/hooks/setup', (route) => route.fulfill({ json: offer }))
  await page.goto('/')
  await openSection(page, 'Branch')

  return asked
}

test('pre-commit runs at once and shows what it writes', async ({ page }) => {
  // Arrange
  const asked = await opensBranch(page)

  // Act
  await page.getByRole('button', { name: 'Run pre-commit' }).click()

  // Assert
  await expect(page.getByRole('region', { name: 'Output of pre-commit' })).toContainText(
    'go vet ./... passed',
  )
  await expect(page.getByText('It went through.')).toBeVisible()
  expect(asked).toEqual([{ kind: 'pre_commit' }])
})

test('a rebase waits on its last look', async ({ page }) => {
  // Arrange
  const asked = await opensBranch(page)
  await page.getByRole('button', { name: 'Rebase onto main' }).click()
  const look = page.getByRole('form', { name: 'Rebase fix/PROJ-1-redact onto main' })
  await expect(look).toBeVisible()
  expect(asked).toEqual([])

  // Act
  await look.getByRole('button', { name: 'Rebase' }).click()

  // Assert
  await expect(page.getByRole('region', { name: 'Output of git rebase' })).toBeVisible()
  expect(asked).toEqual([{ kind: 'rebase' }])
})

// steps open each of the Branch section's new looks, named for the test. A
// radio group is one Tab stop, on the choice made; the arrow keys reach the
// rest, so the fixup's older commit is the stop Tab passes by.
const steps: { name: string; opens: (page: Page) => Promise<void>; passedBy?: string[] }[] = [
  {
    name: 'the rebase look',
    opens: async (page) => {
      await page.getByRole('button', { name: 'Rebase onto main' }).click()
    },
  },
  {
    name: 'the amend look',
    opens: async (page) => {
      await page.getByRole('button', { name: 'Amend last commit' }).click()
    },
  },
  {
    name: 'the fixup look',
    opens: async (page) => {
      await page.getByRole('button', { name: 'Fix up a commit' }).click()
    },
    passedBy: ['a1a1a1a fix: redact tokens'],
  },
  {
    name: 'a run’s output',
    opens: async (page) => {
      await page.getByRole('button', { name: 'Run pre-commit' }).click()
      await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()
    },
  },
  {
    name: 'a file’s diff',
    opens: async (page) => {
      await page.getByRole('button', { name: 'Show diff of log.go' }).click()
      await expect(page.getByRole('region', { name: 'Diff of log.go' })).toBeVisible()
    },
  },
  {
    name: 'the lefthook offer',
    opens: async (page) => {
      await page.getByRole('button', { name: 'Set up lefthook' }).click()
      await expect(page.getByRole('button', { name: 'Write lefthook.yml' })).toBeVisible()
    },
  },
]

for (const theme of themes) {
  for (const { name, opens, passedBy } of steps) {
    test(`${name} fits ${String(widths[0])} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width: widths[0], height })
      await opensBranch(page)
      await opens(page)

      // Act & Assert: Tab once round the page.
      await expectReachableAndClean(page, { passedBy })
    })
  }
}
