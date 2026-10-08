import type { Page } from '@playwright/test'
import type { ReviewFacet, ReviewQueue, ReviewRequest } from '../../src/api/generated/types.gen.ts'
import { openSection, pinTheme, themes, widths, height } from '../support/cockpit.ts'
import { expectReachableAndClean } from '../support/reachable.ts'
import { expect, test } from '../support/fixtures.ts'

// The Reviews section against a queue answered here: it is read again on
// opening once 30 seconds have passed, and its filter narrows it.

const hour = 3_600_000

// facetsOf is what the server says a request holds in each facet, labeled as
// it labels them.
function facetsOf(
  fields: Pick<ReviewRequest, 'author' | 'repository' | 'draft' | 'ci'>,
): ReviewFacet[] {
  const readiness = fields.draft ? 'draft' : 'ready'

  return [
    {
      kind: 'repository',
      value: fields.repository,
      label: fields.repository === '' ? 'no repository' : fields.repository,
    },
    { kind: 'ci', value: fields.ci, label: `CI ${fields.ci}` },
    { kind: 'draft', value: readiness, label: readiness },
    { kind: 'author', value: fields.author, label: `by ${fields.author}` },
  ]
}

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
    facets: facetsOf(fields),
    ...fields,
  }
}

// offered is a value the server offers the queue's filter.
function offered(kind: ReviewFacet['kind'], value: string, label: string): ReviewFacet {
  return { kind, value, label }
}

// The queue the terminal's facetsWorld lists, oldest first, with what the
// server offers its filter, in its order.
const queue = {
  available: true,
  facet_order: [
    offered('repository', '', 'no repository'),
    offered('repository', 'example/other', 'example/other'),
    offered('repository', 'example/repo', 'example/repo'),
    offered('ci', 'failed', 'CI failed'),
    offered('ci', 'passed', 'CI passed'),
    offered('ci', 'running', 'CI running'),
    offered('ci', 'none', 'CI none'),
    offered('draft', 'draft', 'draft'),
    offered('draft', 'ready', 'ready'),
    offered('author', 'kwan', 'by kwan'),
    offered('author', 'mira', 'by mira'),
  ],
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
async function listed(page: Page, name = 'Waiting on your review'): Promise<string[]> {
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

// routedSoFar returns once every request the page has sent so far has reached
// its route: a request sent after them, which the page answers only once its
// own route has run, cannot overtake them there.
async function routedSoFar(page: Page): Promise<void> {
  await page.evaluate(async () => {
    await fetch('/api/health')
  })
}

test('switching back to Reviews within 30 seconds reads nothing again', async ({ page }) => {
  // Arrange
  // Reviews read once, then another section open.
  await page.clock.install()
  const counted = await answersQueue(page)
  await page.goto('/')
  await openSection(page, 'Reviews')
  await openSection(page, 'Branch')
  await page.clock.fastForward(20_000)

  // Act
  await openSection(page, 'Reviews')

  // Assert
  // The queue is drawn again, and a read the reopening started, which goes out
  // as the queue is drawn, would have been counted by now.
  await expect(
    page.getByRole('list', { name: 'Waiting on your review', exact: true }),
  ).toBeVisible()
  await routedSoFar(page)
  expect(counted.reads).toBe(1)
})

test('switching back to Reviews after 30 seconds reads the queue again', async ({ page }) => {
  // Arrange
  // Reviews read once, then another section open past the 30 seconds.
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
  expect(await listed(page, 'Waiting on your review in example/repo')).toEqual(['5'])
})

for (const theme of themes) {
  for (const width of widths) {
    test(`a filtered queue by repository fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await answersQueue(page)
      await page.goto('/')
      await openSection(page, 'Reviews')

      // Act
      // Narrow it, and group it.
      await press(page, 'ready 3')
      await page.getByRole('combobox', { name: 'Sort' }).selectOption('By repository')
      await expect(page.getByRole('heading', { level: 3 }).first()).toBeVisible()

      // Assert
      // Tab reaches the sort and the filter, and it is clean.
      await expectReachableAndClean(page, { reaches: ['Sort', 'ready 3'] })
    })
  }
}
