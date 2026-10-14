import type { Locator } from '@playwright/test'
import { height, openCockpit, openSection, widths } from '../support/cockpit.ts'
import { expect, test } from '../support/fixtures.ts'

// Settings a section at a time, on the populated build: the list of
// sections, and the save bar that stays in view below whichever is shown.

// bottomOf is where an element's box ends, down the window.
function bottomOf(element: Locator): Promise<number> {
  return element.evaluate((drawn) => drawn.getBoundingClientRect().bottom)
}

for (const width of widths) {
  test(
    `the save bar meets the bottom of the content at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width, height }, 'dark')

      // Act
      await openSection(page, 'Settings')

      // Assert
      const bar = page.getByRole('region', { name: 'Save' })
      await expect(bar).toBeInViewport({ ratio: 1 })
      expect(await bottomOf(bar)).toBeCloseTo(await bottomOf(page.getByRole('main')), 0)
    },
  )
}
