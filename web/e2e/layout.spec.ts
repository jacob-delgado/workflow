import type { Page } from '@playwright/test'
import {
  confirmSteps,
  height,
  openCockpit,
  openFirstRun,
  openSection,
  sectionNames,
  themes,
  widths,
} from './support/cockpit.ts'
import { expectReachableAndClean, pageScrolls } from './support/reachable.ts'
import { expect, test } from './support/fixtures.ts'

// The populated cockpit at a narrow, a middling and a wide window, in both
// themes: nothing scrolls sideways, the page does not scroll at all, every
// control the keyboard can reach is in view once it has focus, and axe finds
// nothing.

for (const theme of themes) {
  for (const width of widths) {
    for (const name of sectionNames) {
      test(
        `${name} fits ${String(width)} px in the ${theme} theme, reachable and clean`,
        { tag: '@populated' },
        async ({ page }) => {
          // Arrange
          await openCockpit(page, { width, height }, theme)

          // Act
          await openSection(page, name)

          // Assert
          await expectReachableAndClean(page)
        },
      )
    }
  }
}

// Settings on a server with no configuration file, where it sets one up, at
// each width in both themes.
for (const theme of themes) {
  for (const width of widths) {
    test(`the first-run setup fits ${String(width)} px in the ${theme} theme, reachable and clean`, async ({
      page,
    }) => {
      // Arrange
      await openFirstRun(page, { width, height }, theme)

      // Act & Assert
      // Tab passes by the home directory's radio button: the arrow keys reach
      // it from the one chosen.
      await expectReachableAndClean(page, {
        reaches: ['Write ~/src/api/.workflow.json'],
        passedBy: ['~/.workflow.jsonYour home directory: it applies everywhere.'],
      })
    })
  }
}

// widestContent is the width of the widest field, paragraph or list a section
// draws: the measure its content is set at.
async function widestContent(page: Page): Promise<number> {
  const main = page.getByRole('main')
  const widths = await Promise.all(
    (['textbox', 'combobox', 'paragraph', 'list'] as const).map((role) =>
      main
        .getByRole(role)
        .evaluateAll((parts) => parts.map((part) => part.getBoundingClientRect().width)),
    ),
  )

  return Math.max(...widths.flat())
}

for (const name of sectionNames.filter((section) => section !== 'Branch')) {
  test(
    `${name} sets its content at the Branch section's measure in a wide window`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width: 1440, height }, 'dark')
      await openSection(page, 'Branch')
      const measure = await widestContent(page)

      // Act
      await openSection(page, name)

      // Assert
      expect(await widestContent(page), 'wider than Branch').toBeLessThanOrEqual(measure + 1)
    },
  )
}

for (const theme of themes) {
  for (const width of widths) {
    for (const { step, section, opener, group, adds } of confirmSteps) {
      test(
        `the ${step} fits ${String(width)} px in the ${theme} theme, reachable and clean`,
        { tag: '@populated' },
        async ({ page }) => {
          // Arrange
          await openCockpit(page, { width, height }, theme)
          await openSection(page, section)

          // Act
          await page.getByRole('button', { name: opener }).click()
          await expect(page.getByRole('group', { name: group })).toBeVisible()

          // Assert
          await expectReachableAndClean(page, { reaches: adds })
        },
      )
    }
  }
}

// From md (768 px) the rail names each section beside its icon; below it, the
// rail keeps only the icons, and each name stays the button's accessible name,
// and its title, which a pointer resting on the icon shows.
const railCases = [
  { width: 640, named: false },
  { width: 1024, named: true },
  { width: 1440, named: true },
]

for (const { width, named } of railCases) {
  test(
    `the rail ${named ? 'names its sections' : 'keeps only its icons'} at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await page.setViewportSize({ width, height })
      await page.goto('/')
      const rail = page.getByRole('navigation', { name: 'Sections' })
      const buttons = sectionNames.map((name) => rail.getByRole('button', { name, exact: true }))

      // Act
      const drawn = await Promise.all(
        buttons.map(async (button, index) => {
          const name = await button.getByText(sectionNames[index], { exact: true }).boundingBox()

          return (name?.width ?? 0) > 1
        }),
      )

      // Assert
      for (const [index, button] of buttons.entries()) {
        await expect(button).toBeVisible()
        await expect(button).toHaveAttribute('title', sectionNames[index])
      }
      expect(drawn).toEqual(sectionNames.map(() => named))
    },
  )
}

// The header holds still while a section scrolls beneath it: the page itself
// never scrolls, the content does.
for (const width of widths) {
  test(
    `the header stays in view as a long section scrolls at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      // The settings run longer than the window, so Save starts out of view.
      await page.setViewportSize({ width, height })
      await page.goto('/')
      await openSection(page, 'Settings')
      const save = page.getByRole('button', { name: 'Save changes' })

      // Act
      await save.focus()

      // Assert
      await expect(save).toBeInViewport()
      await expect(page.getByRole('button', { name: /change theme/i })).toBeInViewport()
    },
  )
}

// In a window shorter than the rail, the rail scrolls itself, not the page:
// the header stays in view as the rail's last section takes focus.
test(
  'the rail scrolls itself in a window shorter than it',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await page.setViewportSize({ width: 640, height: 240 })
    await page.goto('/')
    const rail = page.getByRole('navigation', { name: 'Sections' })
    const settings = rail.getByRole('button', { name: 'Settings', exact: true })

    // Act
    await settings.focus()

    // Assert
    await expect(settings).toBeInViewport()
    await expect(page.getByRole('button', { name: /change theme/i })).toBeInViewport()
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
  },
)

// The skip link is the first stop, drawn as it has focus, and it hands focus to
// the content.
for (const width of widths) {
  test(
    `the skip link takes focus to the content at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await page.setViewportSize({ width, height })
      await page.goto('/')
      const skip = page.getByRole('link', { name: 'Skip to content' })
      // The page can be drawn after its load event, as the mockup's is once its
      // server is in place, and a Tab pressed before that reaches nothing.
      await expect(skip).toBeAttached()

      // Act: the first Tab
      await page.keyboard.press('Tab')

      // Assert: it lands on the skip link, drawn in view
      await expect(skip).toBeFocused()
      await expect(skip).toBeInViewport({ ratio: 1 })

      // Act: follow it
      await page.keyboard.press('Enter')

      // Assert: the content has focus
      await expect(page.getByRole('main')).toBeFocused()
    },
  )
}

// A section opens at its top, wherever the last one was scrolled to, so the
// content that takes focus as it opens shows its heading.
for (const width of widths) {
  test(
    `a section opens at its top at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      // Focusing Save scrolls the settings to their end.
      await page.setViewportSize({ width, height })
      await page.goto('/')
      await openSection(page, 'Settings')
      await page.getByRole('button', { name: 'Save changes' }).focus()

      // Act
      await openSection(page, 'Branch')

      // Assert
      await expect(page.getByRole('heading', { level: 1, name: 'Branch' })).toBeInViewport({
        ratio: 1,
      })
    },
  )
}
