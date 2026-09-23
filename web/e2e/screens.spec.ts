import { expect, test } from '@playwright/test'

// Screenshots of every populated section, in both themes, at a narrow, a
// middling and a wide window — saved into test-results, which CI uploads, for
// a reviewer to read a visual change by eye. Nothing is compared: a baseline
// would differ by the OS's fonts, and would commit images that a reviewer
// cannot diff anyway.
const themes = ['dark', 'light'] as const
const widths = [640, 1024, 1440] as const
// The mockup's messaging service is Slack, so its section is named for it.
const sectionNames = ['Issues', 'Branch', 'Review', 'Slack', 'Settings']

for (const theme of themes) {
  for (const width of widths) {
    test(
      `screenshots every section in the ${theme} theme at ${String(width)} px`,
      {
        tag: '@populated',
      },
      async ({ page }, testInfo) => {
        // Arrange: pin the theme before the app paints, size the window, and
        // open the checked-out issue so the Issues screen shows its story.
        await page.addInitScript((value) => {
          window.localStorage.setItem('workflow-theme', value)
        }, theme)
        await page.emulateMedia({ reducedMotion: 'reduce' })
        await page.setViewportSize({ width, height: 900 })
        await page.goto('/')
        await page.getByRole('button', { name: /redact tokens before/i }).click()
        await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()
        const nav = page.getByRole('navigation', { name: 'Sections' })

        for (const name of sectionNames) {
          // Act: open the section.
          await nav.getByRole('button', { name, exact: true }).click()

          // Assert: its heading has settled; then the screen is saved as drawn.
          await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
          await page.screenshot({
            path: testInfo.outputPath(`${String(width)}-${theme}-${name.toLowerCase()}.png`),
            fullPage: true,
          })
        }
      },
    )
  }
}
