import type { Locator } from '@playwright/test'
import {
  height,
  openCockpit,
  openSection,
  openSettings,
  openSettingsTab,
  themes,
  widths,
} from '../support/cockpit.ts'
import { expectReachableAndClean } from '../support/reachable.ts'
import { expect, test } from '../support/fixtures.ts'

// Settings a section at a time, on the populated build: every section at
// every width in both themes, the list of sections beside the one shown or
// above it, the save bar that stays in view below whichever is shown, and a
// setting refused beside itself.

// settingsSections are Settings' sections, in the order its list offers them.
const settingsSections = [
  'Jira',
  'Messaging',
  'Forge and pull requests',
  'Commits and branches',
  'Taskwarrior',
  'Terminal and keys',
  'Advanced',
  'People and groups',
  'Local data',
]

// bottomOf is where an element's box ends, down the window.
function bottomOf(element: Locator): Promise<number> {
  return element.evaluate((drawn) => drawn.getBoundingClientRect().bottom)
}

for (const theme of themes) {
  for (const width of widths) {
    for (const section of settingsSections) {
      test(
        `Settings' ${section} fits ${String(width)} px in the ${theme} theme, reachable and clean`,
        { tag: '@populated' },
        async ({ page }) => {
          // Arrange
          await openCockpit(page, { width, height }, theme)

          // Act
          await openSettings(page, section)

          // Assert
          await expect(page.getByRole('tabpanel', { name: section })).toBeVisible()
          await expectReachableAndClean(page)
        },
      )
    }
  }
}

const orientations = [
  { width: 640, orientation: 'horizontal' },
  { width: 1024, orientation: 'vertical' },
  { width: 1440, orientation: 'vertical' },
]

for (const { width, orientation } of orientations) {
  test(
    `the sections are listed ${orientation === 'vertical' ? 'beside' : 'above'} the one shown at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width, height }, 'dark')

      // Act
      await openSection(page, 'Settings')

      // Assert
      const list = page.getByRole('tablist', { name: 'Settings sections' })
      // A list that names no orientation is horizontal, as ARIA reads one.
      expect((await list.getAttribute('aria-orientation')) ?? 'horizontal').toBe(orientation)
      const listBox = await list.boundingBox()
      const panelBox = await page.getByRole('tabpanel', { name: 'Jira' }).boundingBox()
      const beside = (listBox?.x ?? 0) + (listBox?.width ?? 0) <= (panelBox?.x ?? 0)
      expect(beside).toBe(orientation === 'vertical')
    },
  )
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

for (const theme of themes) {
  test(
    `a refused setting is shown beside itself in the ${theme} theme, reachable and clean`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width: 1024, height }, theme)
      await openSettings(page, 'Advanced')
      await page.getByRole('textbox', { name: 'CI interval' }).fill('soon')
      await openSettingsTab(page, 'Jira')
      await page.getByRole('button', { name: 'Save changes' }).click()

      // Act
      await page.getByRole('button', { name: 'Show' }).click()

      // Assert
      const refused = page.getByRole('textbox', { name: 'CI interval' })
      await expect(refused).toBeFocused()
      await expect(refused).toHaveAttribute('aria-invalid', 'true')
      await expect(page.getByRole('region', { name: 'Save' })).toContainText(
        '1 unsaved change, 1 to fix in Advanced',
      )
      await expectReachableAndClean(page)
    },
  )
}
