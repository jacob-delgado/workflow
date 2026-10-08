import { expect, test, type Page } from '@playwright/test'
import type { HookSetup, RunEvent, Snapshot } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../support/cockpit.ts'
import { axeViolations, sidewaysScrollers, streams, walkTabOrder } from '../support/tabwalk.ts'

// The Branch section's git runs — pre-commit, a rebase, an amend, a fixup —
// streamed as they go, each that rewrites history behind a last look; a
// changed file's diff; and setting up lefthook.

const snapshot = {
  issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
  branch: {
    name: 'fix/PROJ-1-redact',
    issue_link: '',
    detached: false,
    head: 'b2b2b2b',
    upstream: '',
    push_remote: 'origin',
    ahead: 2,
    behind: 0,
    base: 'origin/main',
    commits: [
      { hash: 'a1a1a1a1', subject: 'fix: redact tokens', unpushed: true },
      { hash: 'b2b2b2b2', subject: 'test: prove it', unpushed: true },
    ],
  },
  changes: {
    changes: [
      { path: 'log.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
    ],
  },
  review: { found: false, announced: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
  commit_types: ['feat', 'fix'],
  subject_limit: 72,
  suggested_scope: '',
  hooks_unmanaged: 1,
  tasks: { available: true, reason: '', linked: [] },
  here: '/home/ana/src/api',
} satisfies Snapshot

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

// olderCommit is the fixup's other choice, as its radio is named.
const olderCommit = 'a1a1a1a fix: redact tokens'

// steps open each of the Branch section's new looks, named for the test.
const steps: Record<string, (page: Page) => Promise<void>> = {
  'the rebase look': async (page) => {
    await page.getByRole('button', { name: 'Rebase onto main' }).click()
  },
  'the amend look': async (page) => {
    await page.getByRole('button', { name: 'Amend last commit' }).click()
  },
  'the fixup look': async (page) => {
    await page.getByRole('button', { name: 'Fix up a commit' }).click()
  },
  'a run’s output': async (page) => {
    await page.getByRole('button', { name: 'Run pre-commit' }).click()
    await expect(page.getByRole('button', { name: 'Close' })).toBeVisible()
  },
  'a file’s diff': async (page) => {
    await page.getByRole('button', { name: 'Show diff of log.go' }).click()
    await expect(page.getByRole('region', { name: 'Diff of log.go' })).toBeVisible()
  },
  'the lefthook offer': async (page) => {
    await page.getByRole('button', { name: 'Set up lefthook' }).click()
    await expect(page.getByRole('button', { name: 'Write lefthook.yml' })).toBeVisible()
  },
}

for (const theme of themes) {
  for (const [name, opens] of Object.entries(steps)) {
    test(`${name} fits ${String(widths[0])} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width: widths[0], height })
      await opensBranch(page)
      await opens(page)

      // Act: Tab once round the page.
      const { missed, hidden } = await walkTabOrder(page)

      // Assert
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      // A radio group is one Tab stop, on the choice made; the arrow keys
      // reach the rest, so the fixup's older commit is the stop Tab passes by.
      expect(missed, 'never reached by Tab').toEqual(name === 'the fixup look' ? [olderCommit] : [])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
