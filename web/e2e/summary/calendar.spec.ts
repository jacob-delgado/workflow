import type { Page } from '@playwright/test'
import { mockActivity } from '../../src/dev/mockActivity.ts'
import { height, pinTheme, themes, widths } from '../support/cockpit.ts'
import { expect, problem, test } from '../support/fixtures.ts'
import { axeViolations, sidewaysScrollers, walkTabOrder } from '../support/tabwalk.ts'

// The Summary section: what was done, the calendar it is picked in, and the
// period each pick reads.

// opensSummary answers every read of what was done with the mockup's day for
// the period asked, keeps each query, and opens the section.
async function opensSummary(page: Page): Promise<string[]> {
  const asked: string[] = []
  await page.route('**/api/activity**', (route) => {
    const url = new URL(route.request().url())
    asked.push(url.search)
    const from = url.searchParams.get('from')
    const to = url.searchParams.get('to')

    return route.fulfill({ json: mockActivity(from === null || to === null ? null : { from, to }) })
  })
  await page.goto('/')
  await page.getByRole('button', { name: 'Summary' }).click()
  await expect(page.getByRole('list', { name: /Tuesday, September 15, 2026/ })).toBeVisible()

  return asked
}

test('a day picked from the calendar by keyboard is read', async ({ page }) => {
  // Arrange
  const asked = await opensSummary(page)
  await page.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await page.keyboard.press('ArrowLeft')
  await page.keyboard.press('Enter')

  // Assert
  await expect.poll(() => asked.at(-1)).toBe('?from=2026-09-14&to=2026-09-14')
  await expect(
    page.getByRole('heading', { level: 2, name: 'Monday, September 14, 2026' }),
  ).toBeVisible()
})

test('Shift and Enter reads the range from the day shown', async ({ page }) => {
  // Arrange
  const asked = await opensSummary(page)
  await page.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await page.keyboard.press('ArrowRight')
  await page.keyboard.press('ArrowRight')
  await page.keyboard.press('Shift+Enter')

  // Assert
  await expect.poll(() => asked.at(-1)).toBe('?from=2026-09-15&to=2026-09-17')
})

test('a day picked by keyboard keeps the focus in the calendar', async ({ page }) => {
  // Arrange
  await opensSummary(page)
  await page.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await page.keyboard.press('ArrowLeft')
  await page.keyboard.press('Enter')

  // Assert
  await expect(
    page.getByRole('heading', { level: 2, name: 'Monday, September 14, 2026' }),
  ).toBeVisible()
  await expect(page.getByRole('gridcell', { name: 'Monday, September 14, 2026' })).toBeFocused()
})

test('a period the server refuses leaves the calendar to pick another', async ({ page }) => {
  // Arrange
  await opensSummary(page)
  await page.route('**/api/activity?from=2026-09-14**', (route) =>
    route.fulfill(
      problem(
        'unprocessable',
        'the period could not be read: the period is longer than a year and a day',
      ),
    ),
  )
  await page.getByRole('gridcell', { name: 'Tuesday, September 15, 2026' }).focus()

  // Act
  await page.keyboard.press('ArrowLeft')
  await page.keyboard.press('Enter')

  // Assert
  await expect(page.getByText(/longer than a year and a day/)).toBeVisible()
  await expect(page.getByRole('gridcell', { name: 'Monday, September 14, 2026' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Earlier' })).toBeVisible()
})

test('a year picked some way back still offers the years since', async ({ page }) => {
  // Arrange
  await opensSummary(page)
  const year = page.getByRole('combobox', { name: 'Year' })

  // Act
  await year.selectOption('2021')

  // Assert
  await expect(year.getByRole('option', { name: '2026' })).toBeAttached()
})

for (const theme of themes) {
  for (const width of widths) {
    test(`the Summary fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await pinTheme(page, theme)
      await page.emulateMedia({ reducedMotion: 'reduce' })
      await page.setViewportSize({ width, height })
      await opensSummary(page)

      // Act: Tab once round the page.
      const { reached, missed, hidden } = await walkTabOrder(page)

      // Assert: nothing scrolls sideways; Tab reaches the steps, the copy, the
      // links, the selects, the day in focus and the month and year, each in
      // view; and axe finds nothing.
      expect(await page.evaluate(sidewaysScrollers), 'scrolls sideways').toEqual([])
      expect(reached, 'reached by Tab').toEqual(
        expect.arrayContaining([
          'Earlier',
          'Later',
          'Today',
          'Copy as Markdown',
          'PROJ-412 (opens in a new tab)',
          'Year',
          'Month',
          'Tuesday, September 15, 2026',
          'Whole month',
          'Whole year',
        ]),
      )
      expect(missed, 'never reached by Tab').toEqual([])
      expect(hidden, 'out of view with focus').toEqual([])
      expect(await axeViolations(page), 'axe').toBe('')
    })
  }
}
