import { expect, test, type Page } from '@playwright/test'
import type { LocalData } from '../../src/api/generated/types.gen.ts'
import { height, openCockpit, openSection, themes, widths } from '../cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, walkTabOrder } from '../tabwalk.ts'

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
      holds: [
        { what: 'scopes', count: 3 },
        { what: 'cached issues', count: 41 },
      ],
    },
    {
      name: 'kept.db',
      kind: 'kept',
      bytes: 24_576,
      holds: [{ what: 'repository groups', count: 2 }],
    },
  ],
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

    return route.fulfill({ json: { dir: listing.dir, files } })
  })

  return asked
}

// opensLocalData opens Settings and waits for its Local data listing. The
// hermetic build reads no configuration, so the form above says so; the area
// reads and removes on its own.
async function opensLocalData(page: Page): Promise<void> {
  await page.goto('/')
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Settings', exact: true })
    .click()
  await expect(page.getByRole('table', { name: 'Local data files' })).toBeVisible()
}

test('lists the store, confirms, removes the cache and reads the listing again', async ({
  page,
}) => {
  // Arrange: the Local data area, its cache's removal asked for.
  const asked = await answersStore(page)
  await opensLocalData(page)
  await page.getByRole('button', { name: 'Remove cache…' }).click()
  const question = page.getByRole('group', { name: 'Remove workflow.db?' })
  await expect(question).toBeFocused()

  // Act
  await question.getByRole('button', { name: 'Remove' }).click()

  // Assert: said, removed from the listing, and read again after the removal.
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
        // Arrange: the populated Settings, in this theme, at this width.
        await openCockpit(page, { width, height }, theme)
        await openSection(page, 'Settings')
        await expect(page.getByRole('table', { name: 'Local data files' })).toBeVisible()

        // Act: open the confirm step that clears everything, and Tab once round.
        await page.getByRole('button', { name: 'Remove everything…' }).click()
        await expect(page.getByRole('group', { name: /remove workflow\.db/i })).toBeFocused()
        const { reached, missed, hidden } = await walkTabOrder(page)

        // Assert: nothing scrolls sideways, nor the page down; Tab reaches the
        // step's controls and every other drawn one, each in view; and axe
        // finds nothing.
        expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
        expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
        expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(['Cancel', 'Remove']))
        expect(missed, 'never reached by Tab').toEqual([])
        expect(hidden, 'out of view with focus').toEqual([])
        expect(await axeViolations(page), 'axe').toBe('')
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

    // Assert: the people and groups went with the kept file.
    await expect(page.getByText('Removed workflow.db and kept.db.')).toBeVisible()
    await expect(page.getByText(/no local data/i)).toBeVisible()
    await expect(page.getByRole('button', { name: 'Forget dan…' })).toBeHidden()
    await expect(page.getByRole('checkbox', { name: '@api-reviewers' })).not.toBeChecked()
  },
)
