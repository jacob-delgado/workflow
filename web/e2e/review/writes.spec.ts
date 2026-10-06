import { expect, test, type Page } from '@playwright/test'
import type { PullRequest, Snapshot } from '../../src/api/generated/types.gen.ts'
import { height, openSection, pinTheme, themes, widths } from '../cockpit.ts'
import { axeViolations, sidewaysScrollers, streams, walkTabOrder } from '../tabwalk.ts'

// The Review section's writes on the branch's pull request — edit, merge,
// finish, re-run — each from a form that is its last look.

const ready: PullRequest = {
  number: 42,
  url: 'https://forge.example.com/pull/42',
  title: 'Redact tokens',
  state: 'open',
  draft: false,
  approvals: 1,
  changes_requested: false,
  mergeable: 'clean',
}

// snapshotWith is the branch's pull request in a state, its CI in another.
function snapshotWith(pull: PullRequest, ci: 'passed' | 'failed'): Snapshot {
  return {
    issues: { total: 0, start_at: 0, unavailable: [], issues: [] },
    branch: {
      name: 'fix/PROJ-1-redact',
      issue_link: '',
      detached: false,
      head: 'b2b2b2b',
      upstream: 'origin/fix/PROJ-1-redact',
      push_remote: 'origin',
      ahead: 0,
      behind: 0,
      base: 'origin/main',
      commits: [{ hash: 'b2b2b2b2', subject: 'fix: redact tokens', unpushed: false }],
    },
    changes: { changes: [] },
    review: {
      found: true,
      announced: false,
      pull,
      ci: {
        state: ci,
        total: 1,
        done: 1,
        failed: ci === 'failed' ? 1 : 0,
        checks: [{ name: 'build', state: ci, url: '', id: '7', log_available: true }],
      },
    },
    messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
    branches: [],
    commit_types: ['feat', 'fix'],
    suggested_scope: '',
    hooks_unmanaged: 0,
    tasks: { available: true, reason: '', linked: [] },
    here: '/home/ana/src/api',
  }
}

// opensReview serves the pull request's writes and opens the Review section.
// It answers each write sent, by method and path, with its body.
async function opensReview(page: Page, snapshot: Snapshot): Promise<string[]> {
  const sent: string[] = []
  await streams(page, snapshot)
  await page.route('**/api/pull-request', (route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({ json: { title: 'Redact tokens', body: 'Why: they leak.' } })
    }

    sent.push(`PATCH ${route.request().postData() ?? ''}`)

    return route.fulfill({ json: ready })
  })
  await page.route('**/api/pull-request/merge', (route) => {
    if (route.request().method() === 'GET') {
      return route.fulfill({ json: { pull: ready, methods: ['squash', 'rebase'] } })
    }

    sent.push(`POST merge ${route.request().postData() ?? ''}`)

    return route.fulfill({ json: { ...ready, state: 'merged' } })
  })
  await page.route('**/api/review/rerun', (route) => {
    sent.push('POST rerun')

    return route.fulfill({ json: { reran: true } })
  })
  await page.route('**/api/branch/finish', (route) => {
    sent.push('POST finish')

    return route.fulfill({ json: { ...snapshot.branch, name: 'main' } })
  })
  await page.goto('/')
  await openSection(page, 'Review')

  return sent
}

test('a merge goes by the method chosen, only from its preview', async ({ page }) => {
  // Arrange
  const sent = await opensReview(page, snapshotWith(ready, 'passed'))
  await page.getByRole('button', { name: 'Merge', exact: true }).click()
  const preview = page.getByRole('form', { name: 'Merge #42' })
  await preview.getByRole('radio', { name: 'Rebase and merge' }).check()
  expect(sent).toEqual([])

  // Act
  await preview.getByRole('button', { name: 'Merge', exact: true }).click()

  // Assert
  await expect(page.getByText('Merged #42.')).toBeVisible()
  expect(sent).toEqual(['POST merge {"method":"rebase"}'])
})

// looks open each of the Review section's forms, in the state it needs. A
// radio group is one Tab stop, on the choice made; the arrow keys reach the
// rest, so the merge's other method is the one stop Tab passes by.
const looks: Record<
  string,
  { pull: PullRequest; ci: 'passed' | 'failed'; button: string; form: string; arrows?: string[] }
> = {
  'the editor': { pull: ready, ci: 'passed', button: 'Edit pull request', form: 'Edit #42' },
  'the merge preview': {
    pull: ready,
    ci: 'passed',
    button: 'Merge',
    form: 'Merge #42',
    arrows: ['Rebase and merge'],
  },
  'the re-run look': {
    pull: ready,
    ci: 'failed',
    button: 'Re-run failed checks',
    form: 'Re-run the failed checks on #42',
  },
  'the finish look': {
    pull: { ...ready, state: 'merged' },
    ci: 'passed',
    button: 'Finish the branch',
    form: 'Finish fix/PROJ-1-redact',
  },
}

for (const theme of themes) {
  for (const [name, look] of Object.entries(looks)) {
    test(`${name} fits ${String(widths[0])} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width: widths[0], height })
      await opensReview(page, snapshotWith(look.pull, look.ci))
      await page.getByRole('button', { name: look.button, exact: true }).click()
      await expect(page.getByRole('form', { name: look.form })).toBeVisible()

      // Act: Tab once round the page.
      const { missed, hidden } = await walkTabOrder(page)

      // Assert
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(missed, 'never reached by Tab').toEqual(look.arrows ?? [])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
