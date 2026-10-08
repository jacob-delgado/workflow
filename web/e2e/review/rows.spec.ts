import { height, openCockpit, openSection, themes } from '../support/cockpit.ts'
import { expect, test } from '../support/fixtures.ts'

// The Review section's rows set every value at one left edge, whether or not a
// state's mark stands before it: a mark holds its own slot, and a value with
// none keeps that slot empty, so the column reads as one.

for (const theme of themes) {
  test(
    `every Review value starts at one left edge in the ${theme} theme at 1440 px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width: 1440, height }, theme)

      // Act
      await openSection(page, 'Review')

      // Assert: where each value's words begin, past any mark before them.
      const starts = await page
        .getByRole('region', { name: /^#128/ })
        .locator('dd')
        .evaluateAll((values) =>
          values.map((value) => {
            const walker = document.createTreeWalker(value, NodeFilter.SHOW_TEXT)
            const words = walker.nextNode()
            if (words === null) {
              return Number.NaN
            }
            const range = document.createRange()
            range.selectNodeContents(words)

            return Math.round(range.getBoundingClientRect().left)
          }),
        )
      expect(starts.length).toBeGreaterThan(2)
      expect(new Set(starts).size).toBe(1)
    },
  )
}
