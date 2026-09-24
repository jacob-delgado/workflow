import { expect, type Locator, type Page } from '@playwright/test'

// The populated cockpit as the specs that hold it or picture it open it: in
// both themes, at a narrow, a middling and a wide window.
export const themes = ['dark', 'light'] as const
export const widths = [640, 1024, 1440] as const
export const height = 900
// The mockup's messaging service is Slack, so its section is named for it.
export const sectionNames = ['Issues', 'Branch', 'Review', 'Slack', 'Reviews', 'Settings']

// openCockpit opens the populated cockpit in a theme in a window of a size,
// with the checked-out issue's detail open beside, or under, the list. The
// theme is pinned before the app paints, and motion is reduced, so nothing is
// caught mid-transition.
export async function openCockpit(
  page: Page,
  size: { width: number; height: number },
  theme: string,
): Promise<void> {
  await page.addInitScript((value) => {
    window.localStorage.setItem('workflow-theme', value)
  }, theme)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.setViewportSize(size)
  await page.goto('/')
  await page.getByRole('button', { name: /redact tokens before/i }).click()
  await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()
}

// settled is what shows once a section has drawn what it will: Reviews reads
// its own queue after its heading appears; every other section settles with its
// heading.
function settled(page: Page, name: string): Locator {
  return name === 'Reviews'
    ? page.getByRole('list', { name: 'Review requests' })
    : page.getByRole('heading', { level: 1, name })
}

// openSection opens a section from the rail, and waits for it to settle.
export async function openSection(page: Page, name: string): Promise<void> {
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name, exact: true })
    .click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(settled(page, name)).toBeVisible()
}
