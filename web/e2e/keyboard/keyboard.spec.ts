import type { Page } from '@playwright/test'
import { height, openCockpit, openSection, themes, widths } from '../support/cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers, walkTabOrder } from '../support/tabwalk.ts'
import { expect, test } from '../support/fixtures.ts'

// The page's keyboard beyond Tab on the populated build, whose keys have the
// single-key shortcuts on: the ? sheet and the Ctrl+K palette, each at a
// narrow, a middling and a wide window in both themes. Neither may scroll the
// page or sideways, Tab must stay inside each and reach every control in view,
// and axe must find nothing with either open.

// trapped is a lap of Tab round an open dialog that keeps it: every control
// in it reached, each in view, and none outside it. The lap wraps through the
// browser's own controls, which a modal dialog leaves reachable, as the
// page's body: that stop is the wrap, not an escape.
const trapped = { missed: [], hidden: [], left: [] }

// heldToTheLayout names what is wrong with the page as it stands: what scrolls
// sideways, the page scrolling down, and what axe finds.
async function heldToTheLayout(page: Page): Promise<string[]> {
  return [
    ...(await page.evaluate(sidewaysScrollers)),
    ...((await page.evaluate(pageScrolls)) ? ['the page scrolls'] : []),
    ...[await axeViolations(page)].filter((found) => found !== ''),
  ]
}

for (const theme of themes) {
  for (const width of widths) {
    test(
      `the ? sheet fits ${String(width)} px in the ${theme} theme, trapped and clean`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange: the populated cockpit, its issue open, focus on the issue.
        await openCockpit(page, { width, height }, theme)
        const opener = page.getByRole('button', { name: /redact tokens before/i })
        await opener.focus()

        // Act: open the sheet.
        await page.keyboard.press('?')

        // Assert: it lists comment under Issues on c, fits, keeps Tab, and is
        // clean.
        const sheet = page.getByRole('dialog', { name: 'Keyboard shortcuts' })
        await expect(
          sheet.getByRole('table', { name: 'Issues' }).getByRole('row', { name: 'c comment' }),
        ).toBeVisible()
        expect(await heldToTheLayout(page)).toEqual([])
        expect(await walkTabOrder(page, { within: sheet })).toMatchObject(trapped)

        // Act: close it.
        await page.keyboard.press('Escape')

        // Assert: focus is back where it was.
        await expect(sheet).toBeHidden()
        await expect(opener).toBeFocused()
      },
    )
  }
}

for (const theme of themes) {
  for (const width of widths) {
    test(
      `the palette fits ${String(width)} px in the ${theme} theme, trapped and clean`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange: the populated cockpit, on Branch.
        await openCockpit(page, { width, height }, theme)
        await openSection(page, 'Branch')

        // Act: open the palette, and narrow it.
        await page.keyboard.press('Control+k')
        await page.keyboard.type('stage')

        // Assert: it lists Stage all first, fits, keeps Tab, and is clean.
        const palette = page.getByRole('dialog', { name: 'Command palette' })
        await expect(palette.getByRole('option').first()).toHaveText(/^stage all/)
        expect(await heldToTheLayout(page)).toEqual([])
        expect(await walkTabOrder(page, { within: palette })).toMatchObject(trapped)
      },
    )
  }
}

test('Ctrl+K, "stage all", Enter stages every change', { tag: '@populated' }, async ({ page }) => {
  // Arrange: the populated cockpit, on Branch.
  await openCockpit(page, { width: 1440, height }, 'dark')
  await openSection(page, 'Branch')

  // Act
  await page.keyboard.press('Control+k')
  await page.keyboard.type('stage all')
  await page.keyboard.press('Enter')

  // Assert
  await expect(page.getByText('Staged every change.')).toBeVisible()
  await expect(page.getByRole('dialog')).toBeHidden()
})

test('a key typed in the search box only types it', { tag: '@populated' }, async ({ page }) => {
  // Arrange
  await openCockpit(page, { width: 1440, height }, 'dark')
  const search = page.getByRole('searchbox', { name: 'Search' }).first()

  // Act
  await search.fill('')
  await search.press('c')

  // Assert
  await expect(search).toHaveValue('c')
  await expect(search).toBeFocused()
})
