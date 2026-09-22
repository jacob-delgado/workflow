import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'

// Every section, in both themes: a light theme is only real once its contrast
// holds up, so the scan runs the whole cockpit in each. The section labels are
// the nav buttons' accessible names and the content heading's text.
const themes = ['dark', 'light'] as const
const sectionNames = ['Issues', 'Branch', 'Review', 'Messaging', 'Settings']

// Scan the resting state, not mid-animation frames: reduced motion collapses
// transitions to instant, so axe never samples a half-faded element (whose
// transient blended colors are a false contrast failure).
test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})

// scan returns the WCAG A/AA violations axe finds on whatever is on screen.
async function scan(page: Page) {
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  return violations
}

for (const theme of themes) {
  test(`no accessibility violations across the sections in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: pin the theme before the app paints, so the whole run is in it.
    await page.addInitScript((value) => {
      window.localStorage.setItem('workflow-theme', value)
    }, theme)
    await page.goto('/')

    const nav = page.getByRole('navigation', { name: 'Sections' })

    for (const name of sectionNames) {
      // Act: open the section and let its heading settle.
      await nav.getByRole('button', { name, exact: true }).click()
      await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()

      // Assert: axe finds nothing on this section in this theme.
      const violations = await scan(page)
      const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
      expect(violations, `${theme} / ${name}: ${summary}`).toEqual([])
    }
  })
}
