import type { Page } from '@playwright/test'
import type { LocalData } from '../../src/api/generated/types.gen.ts'
import { height, openCockpit, openSection, themes, widths } from '../support/cockpit.ts'
import { expectReachableAndClean } from '../support/reachable.ts'
import { expect, test } from '../support/fixtures.ts'

// Settings' Local data area: the store's files listed, each removal behind a
// confirm step, and the listing read again once a removal is done. The flows run
// on the hermetic build, whose answers this spec gives and records; the layout
// runs on the populated build, below the mockup's configuration form.

const listing = {
  dir: '/home/ana/.local/state/workflow',
  files: [
    {
      name: 'workflow.db',
      kind: 'cache',
      bytes: 94_208,
      size: '92.0 KiB',
      holds: [
        { what: 'scopes', count: 3 },
        { what: 'cached issues', count: 41 },
      ],
    },
    {
      name: 'kept.db',
      kind: 'kept',
      bytes: 24_576,
      size: '24.0 KiB',
      holds: [{ what: 'repository groups', count: 2 }],
    },
  ],
  consequences: {
    cache:
      'The last scope, what was announced and the cached issue lists are made again as you work.',
    all:
      "Whom each code owner is on Slack and each repository's groups go with it: " +
      'people and group associations will be asked again.',
  },
} satisfies LocalData

// answersStore answers the local data from a store that a removal empties as
// its scope says, and returns each request's method and query, in order.
async function answersStore(page: Page): Promise<string[]> {
  const asked: string[] = []
  let files: LocalData['files'] = listing.files
  await page.route('**/api/local-data**', (route) => {
    const request = route.request()
    const url = new URL(request.url())
    asked.push(`${request.method()} ${url.search}`)
    if (request.method() === 'DELETE') {
      const scope = url.searchParams.get('scope')
      files = scope === 'all' ? [] : files.filter((file) => file.kind === 'kept')
    }

    return route.fulfill({ json: { ...listing, files } })
  })

  return asked
}

// opensLocalData opens Settings and waits for its Local data listing. The
// hermetic build reads no configuration, so the form above says so; the area
// reads and removes on its own.
async function opensLocalData(page: Page): Promise<void> {
  await page.goto('/')
  await openSection(page, 'Settings')
  await expect(page.getByRole('table', { name: 'Local data files' })).toBeVisible()
}

test('lists the store, confirms, removes the cache and reads the listing again', async ({
  page,
}) => {
  // Arrange
  // The Local data area, its cache's removal asked for.
  const asked = await answersStore(page)
  await opensLocalData(page)
  await page.getByRole('button', { name: 'Remove cache…' }).click()
  const question = page.getByRole('group', { name: 'Remove workflow.db?' })
  await expect(question).toBeFocused()

  // Act
  await question.getByRole('button', { name: 'Remove' }).click()

  // Assert
  // Said, removed from the listing, and read again after the removal.
  await expect(page.getByRole('status').filter({ hasText: 'Removed workflow.db.' })).toBeVisible()
  const table = page.getByRole('table', { name: 'Local data files' })
  await expect(table.getByText('workflow.db')).toBeHidden()
  await expect(table.getByText('kept.db')).toBeVisible()
  await expect.poll(() => asked).toEqual(['GET ', 'DELETE ?scope=cache', 'GET '])
})

test('removing everything warns first, and Cancel sends nothing', async ({ page }) => {
  // Arrange
  const asked = await answersStore(page)
  await opensLocalData(page)
  await page.getByRole('button', { name: 'Remove everything…' }).click()
  const question = page.getByRole('group', { name: 'Remove workflow.db and kept.db?' })
  await expect(question).toContainText('people and group associations will be asked again')

  // Act
  await question.getByRole('button', { name: 'Cancel' }).click()

  // Assert
  await expect(page.getByRole('button', { name: 'Remove everything…' })).toBeFocused()
  expect(asked).toEqual(['GET '])
})

for (const theme of themes) {
  for (const width of widths) {
    test(
      `the confirm step fits ${String(width)} px in the ${theme} theme, reachable and clean`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange
        // The populated Settings, in this theme, at this width.
        await openCockpit(page, { width, height }, theme)
        await openSection(page, 'Settings')
        await expect(page.getByRole('table', { name: 'Local data files' })).toBeVisible()

        // Act
        // Open the confirm step that clears everything.
        await page.getByRole('button', { name: 'Remove everything…' }).click()
        await expect(page.getByRole('group', { name: /remove workflow\.db/i })).toBeFocused()

        // Assert
        // Tab reaches the step's controls, and it is clean.
        await expectReachableAndClean(page, { reaches: ['Cancel', 'Remove'] })
      },
    )
  }
}

test(
  'the mockup removes its store and lists what is left',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await openCockpit(page, { width: 1440, height }, 'dark')
    await openSection(page, 'Settings')
    await expect(page.getByRole('button', { name: 'Forget dan…' })).toBeVisible()
    await page.getByRole('button', { name: 'Remove everything…' }).click()

    // Act
    await page.getByRole('button', { name: 'Remove', exact: true }).click()

    // Assert
    // The people and groups went with the kept file.
    await expect(page.getByText('Removed workflow.db and kept.db.')).toBeVisible()
    await expect(page.getByText(/no local data/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Forget dan…' })).toBeHidden()
    await expect(page.getByRole('checkbox', { name: '@api-reviewers' })).not.toBeChecked()
  },
)
