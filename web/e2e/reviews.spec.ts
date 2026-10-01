import { expect, test, type Page } from '@playwright/test'
import type { ReviewQueue, ReviewRequest } from '../src/api/generated/types.gen.ts'
import { openSection, pinTheme, themes, widths, height } from './cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, walkTabOrder } from './tabwalk.ts'

// The Reviews section against a queue answered here: it is read again on
// opening once 30 seconds have passed, and its filter narrows it.

const hour = 3_600_000

// requestNumbered is a queued request, opened some hours before now.
function requestNumbered(
  number: number,
  hoursAgo: number,
  fields: Pick<ReviewRequest, 'author' | 'repository' | 'draft' | 'ci'>,
): ReviewRequest {
  return {
    number,
    url: `https://forge.example.com/pull/${String(number)}`,
    title: `change number ${String(number)}`,
    opened_at: new Date(Date.now() - hoursAgo * hour).toISOString(),
    ...fields,
  }
}

// The queue the terminal's facetsWorld lists, oldest first.
const queue = {
  available: true,
  requests: [
    requestNumbered(5, 40, {
      author: 'kwan',
      repository: 'example/repo',
      draft: true,
      ci: 'running',
    }),
    requestNumbered(12, 26, {
      author: 'kwan',
      repository: 'example/other',
      draft: false,
      ci: 'passed',
    }),
    requestNumbered(3, 10, { author: 'mira', repository: '', draft: false, ci: 'none' }),
    requestNumbered(7, 3, {
      author: 'mira',
      repository: 'example/repo',
      draft: false,
      ci: 'failed',
    }),
  ],
} satisfies ReviewQueue

// answersQueue answers every read of the queue, and counts them.
async function answersQueue(page: Page): Promise<{ reads: number }> {
  const counted = { reads: 0 }
  await page.route('**/api/reviews', (route) => {
    counted.reads += 1

    return route.fulfill({ json: queue })
  })

  return counted
}

// listed is the number of each request a list of the queue shows, in order.
async function listed(page: Page, name = 'Review requests'): Promise<string[]> {
  const links = page.getByRole('list', { name, exact: true }).getByRole('link')

  return (await links.evaluateAll((all) => all.map((link) => link.getAttribute('href') ?? ''))).map(
    (href) => href.split('/').at(-1) ?? '',
  )
}

// press presses the filter's button for a value.
async function press(page: Page, name: string): Promise<void> {
  const button = page.getByRole('group', { name: 'Filter' }).getByRole('button', { name })
  await button.click()
  await expect(button).toHaveAttribute('aria-pressed', 'true')
}

test('switching back to Reviews within 30 seconds reads nothing again', async ({ page }) => {
  // Arrange: Reviews read once, then another section open.
  await page.clock.install()
  const counted = await answersQueue(page)
  await page.goto('/')
  await openSection(page, 'Reviews')
  await openSection(page, 'Branch')
  await page.clock.fastForward(20_000)

  // Act
  await openSection(page, 'Reviews')

  // Assert
  // A read the reopening starts goes out as the queue is drawn; let the
  // network settle first, so one would have been counted.
  await expect(page.getByRole('list', { name: 'Review requests', exact: true })).toBeVisible()
  await page.waitForLoadState('networkidle')
  expect(counted.reads).toBe(1)
})

test('switching back to Reviews after 30 seconds reads the queue again', async ({ page }) => {
  // Arrange: Reviews read once, then another section open past the 30 seconds.
  await page.clock.install()
  const counted = await answersQueue(page)
  await page.goto('/')
  await openSection(page, 'Reviews')
  await openSection(page, 'Branch')
  await page.clock.fastForward(31_000)

  // Act
  await openSection(page, 'Reviews')

  // Assert
  await expect.poll(() => counted.reads).toBe(2)
})

test('a repository and a CI state narrow the queue together', async ({ page }) => {
  // Arrange
  await answersQueue(page)
  await page.goto('/')
  await openSection(page, 'Reviews')

  // Act
  await press(page, 'example/repo 2')
  await press(page, 'CI failed 1')

  // Assert
  expect(await listed(page)).toEqual(['7'])
  await expect(
    page.getByText('4 pull requests wait on your review, oldest first; 1 shown.'),
  ).toBeVisible()
})

test('two CI states widen the queue, and hold when it is sorted', async ({ page }) => {
  // Arrange
  await answersQueue(page)
  await page.goto('/')
  await openSection(page, 'Reviews')
  await press(page, 'CI failed 1')
  await press(page, 'CI none 1')

  // Act
  await page.getByRole('combobox', { name: 'Sort' }).selectOption('Newest first')

  // Assert
  expect(await listed(page)).toEqual(['7', '3'])
})

test('by repository heads each repository the filter leaves', async ({ page }) => {
  // Arrange
  await answersQueue(page)
  await page.goto('/')
  await openSection(page, 'Reviews')
  await press(page, 'by kwan 2')

  // Act
  await page.getByRole('combobox', { name: 'Sort' }).selectOption('By repository')

  // Assert
  await expect(page.getByRole('heading', { level: 3 })).toHaveText([
    'example/other',
    'example/repo',
  ])
  expect(await listed(page, 'Review requests in example/repo')).toEqual(['5'])
})

for (const theme of themes) {
  for (const width of widths) {
    test(`a filtered queue by repository fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange: the queue, in this theme, at this width.
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await answersQueue(page)
      await page.goto('/')
      await openSection(page, 'Reviews')

      // Act: narrow it, group it, and Tab once round the page.
      await press(page, 'ready 3')
      await page.getByRole('combobox', { name: 'Sort' }).selectOption('By repository')
      await expect(page.getByRole('heading', { level: 3 }).first()).toBeVisible()
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways, nor the page down; Tab reaches the
      // sort and the filter, and every other drawn control, each in view; and
      // axe finds nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
      expect(reached, 'reached by Tab').toEqual(expect.arrayContaining(['Sort', 'ready 3']))
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
