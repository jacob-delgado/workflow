import { expect, type Locator, type Page } from '@playwright/test'
import { themeStorageKey } from '../src/shell/themeKey.ts'

// The populated cockpit as the specs that hold it or picture it open it: in
// both themes, at a narrow, a middling and a wide window.
export const themes = ['dark', 'light'] as const
export const widths = [640, 1024, 1440] as const
export const height = 900
// The mockup's messaging service is Slack, so its section is named for it.
export const sectionNames = [
  'Issues',
  'Branch',
  'Review',
  'Slack',
  'Reviews',
  'Tasks',
  'Summary',
  'Repositories',
  'Settings',
]

// pinTheme saves a theme choice before the app paints, so the whole run is in
// it from the first frame.
export async function pinTheme(page: Page, choice: string): Promise<void> {
  await page.addInitScript(
    ({ key, value }) => {
      window.localStorage.setItem(key, value)
    },
    { key: themeStorageKey, value: choice },
  )
}

// openCockpit opens the populated cockpit in a theme in a window of a size,
// with the checked-out issue's detail open beside, or under, the list. The
// theme is pinned before the app paints, and motion is reduced, so nothing is
// caught mid-transition. The detail has settled once its comment box has the
// Markdown bar the mockup's configuration turns on: the box waits for that
// read behind a Reading status, then draws with its bar.
export async function openCockpit(
  page: Page,
  size: { width: number; height: number },
  theme: string,
): Promise<void> {
  await pinTheme(page, theme)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.setViewportSize(size)
  await page.goto('/')
  await page.getByRole('button', { name: /redact tokens before/i }).click()
  await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()
  await expect(page.getByRole('tablist', { name: 'Comment' })).toBeVisible()
}

// noFile answers the page as a server with no configuration file: the
// configuration not found, and a setup offered in the repository or the home
// directory, with a keychain.
const noFile = {
  status: 404,
  contentType: 'application/problem+json',
  body: JSON.stringify({
    type: 'https://jacob-delgado.github.io/workflow/docs/errors/#not-found',
    title: 'Not found',
    status: 404,
    detail: 'no .workflow.json applies where the server works; set one up in Settings',
    code: 'not_found',
  }),
}

// openFirstRun opens Settings in a theme in a window of a size, on a server
// with no configuration file, where it sets one up.
export async function openFirstRun(
  page: Page,
  size: { width: number; height: number },
  theme: string,
): Promise<void> {
  await pinTheme(page, theme)
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.setViewportSize(size)
  await page.route('**/api/config', (route) => route.fulfill(noFile))
  await page.route('**/api/config/setup', (route) =>
    route.fulfill({
      json: {
        needed: true,
        keychain: true,
        places: [
          {
            place: 'repository',
            path: '/home/ana/src/api/.workflow.json',
            shown: '~/src/api/.workflow.json',
          },
          { place: 'home', path: '/home/ana/.workflow.json', shown: '~/.workflow.json' },
        ],
      },
    }),
  )
  await page.goto('/')
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name: 'Settings', exact: true })
    .click()
  await expect(page.getByRole('form', { name: 'Set up workflow' })).toBeVisible()
  await expect(readings(page)).toHaveCount(0)
}

// readings are the "Reading …" status lines a section shows while a read of
// its own is in flight. Every read says one (web/src/lib/Status.tsx), and the
// controls it fills in draw only as it ends: Summary's period, Settings' code
// owners, groups and local data, each after the heading is up. A walk counted
// before they draw runs out of Tabs before it gets back round to the header.
function readings(page: Page): Locator {
  return page
    .getByRole('main')
    .getByRole('status')
    .filter({ hasText: /^Reading/ })
}

// openSection opens a section from the rail, and waits for it to settle: its
// heading up, and no read of its own still in flight.
export async function openSection(page: Page, name: string): Promise<void> {
  await page
    .getByRole('navigation', { name: 'Sections' })
    .getByRole('button', { name, exact: true })
    .click()
  await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
  await expect(readings(page)).toHaveCount(0)
}
