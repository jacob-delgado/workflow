import { expect, test } from './support/fixtures.ts'
import { pinTheme } from './support/cockpit.ts'

// The "system" choice resolves to the OS color scheme — before paint, in the
// script index.html loads first, public/theme.js, and as it changes, through the matchMedia
// listener useApplyTheme registers. Neither is reachable from the jsdom unit
// tests, so it is proven here end to end.
test('the system theme resolves to the OS scheme and follows it as it changes', async ({
  page,
}) => {
  // Arrange: choose "system" and open under a dark OS
  await pinTheme(page, 'system')
  await page.emulateMedia({ colorScheme: 'dark' })

  // Act: load the app
  await page.goto('/')

  // Assert: the first paint resolved to dark
  // eslint-disable-next-line no-restricted-syntax, playwright/no-raw-locators -- data-theme is the resolved theme itself, the value the pre-paint script, public/theme.js, and useApplyTheme set on the root element; no role, name or text carries it
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')

  // Act: the OS switches to light while the app is open
  await page.emulateMedia({ colorScheme: 'light' })

  // Assert: it follows, without a reload
  // eslint-disable-next-line no-restricted-syntax, playwright/no-raw-locators -- data-theme is the resolved theme itself, the value the pre-paint script, public/theme.js, and useApplyTheme set on the root element; no role, name or text carries it
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')
})
