import { expect, test, type Locator, type Page } from '@playwright/test'
import { height, openCockpit, openSection, themes, widths } from '../cockpit.ts'
import { axeViolations, pageScrolls, sidewaysScrollers } from '../tabwalk.ts'

// The page's keyboard beyond Tab on the populated build, whose keys have the
// single-key shortcuts on: the ? sheet and the Ctrl+K palette, each at a
// narrow, a middling and a wide window in both themes. Neither may scroll the
// page or sideways, Tab must stay inside each and reach every control in view,
// and axe must find nothing with either open.

// trappedLap presses Tab once round the open dialog's controls and one more,
// and names each stop that left the dialog or was out of view with focus. The
// lap wraps through the browser's own controls, which a modal dialog leaves
// reachable, as the page's body: that stop is the wrap, not an escape.
async function trappedLap(page: Page, dialog: Locator): Promise<string[]> {
  const stops = await dialog.evaluate(
    (shown) => shown.querySelectorAll('a[href], button, input, select, textarea').length,
  )
  const strays: string[] = []

  for (let step = 0; step <= stops; step++) {
    await page.keyboard.press('Tab')
    const stray = await page.evaluate(() => {
      const focused = document.activeElement
      if (focused === document.body) {
        return ''
      }

      if (!(focused instanceof HTMLElement) || focused.closest('dialog') === null) {
        return `left the dialog for ${focused?.localName ?? 'nothing'}`
      }

      const box = focused.getBoundingClientRect()
      const seen =
        box.top >= 0 && box.left >= 0 && box.bottom <= innerHeight && box.right <= innerWidth

      return seen ? '' : `${focused.textContent.trim()} out of view`
    })
    if (stray !== '') {
      strays.push(stray)
    }
  }

  return strays
}

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
        expect(await trappedLap(page, sheet)).toEqual([])

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
        expect(await trappedLap(page, palette)).toEqual([])
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
