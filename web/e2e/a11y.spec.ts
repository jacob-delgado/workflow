import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Locator, type Page } from '@playwright/test'
import {
  height,
  openFirstRun,
  openSection,
  pinTheme,
  sectionNames as populatedSectionNames,
  themes,
} from './support/cockpit.ts'
import { problem } from './support/fixtures.ts'

// Every section, in both themes: a light theme is only real once its contrast
// holds up, so the scan runs the whole cockpit in each. The section labels are
// the nav buttons' accessible names and the content heading's text.
const sectionNames = [
  'Issues',
  'Branch',
  'Review',
  'Messaging',
  'Reviews',
  'Tasks',
  'Summary',
  'Repositories',
  'Settings',
]

// Scan the resting state, not mid-animation frames: reduced motion collapses
// transitions to instant, so axe never samples a half-faded element (whose
// transient blended colors are a false contrast failure).
test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})

// settled is what shows once a section has drawn what it will with no API to
// answer it: Reviews, Tasks, Summary, Repositories and Settings each read their own endpoint
// after their heading appears, and the read fails here, so the scan waits on
// its Try again — the first, where Settings offers one for each of its reads;
// every other section settles with its heading.
function settled(page: Page, name: string): Locator {
  return ['Reviews', 'Tasks', 'Summary', 'Repositories', 'Settings'].includes(name)
    ? page.getByRole('button', { name: 'Try again' }).first()
    : page.getByRole('heading', { level: 1, name })
}

// unreachable is the answer a read gets when the service behind it is down.
const unreachable = problem(
  'unreachable',
  'the service could not be reached; check the network, then try again',
)

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
    // Arrange: pin the theme before the app paints, so the whole run is in it,
    // and answer the health read as a --dry-run server would, so the read-only
    // banner is on screen for every scan (the hermetic server has no API).
    await pinTheme(page, theme)
    await page.route('**/api/health', (route) =>
      route.fulfill({
        json: { version: '1.2.3', dry_run: true, forge_noun: 'pull request', forge_sigil: '#' },
      }),
    )
    // The review queue's read, the task list's and the configuration's fail,
    // so each reason and its Try again are scanned, whenever the answer comes; the
    // populated build scans the queue, the tasks and the form themselves.
    await page.route('**/api/reviews', (route) => route.fulfill(unreachable))
    await page.route('**/api/tasks', (route) => route.fulfill(unreachable))
    await page.route('**/api/repositories', (route) => route.fulfill(unreachable))
    await page.route('**/api/config', (route) => route.fulfill(unreachable))
    await page.goto('/')
    await expect(page.getByText(/every write is held back/i)).toBeVisible()

    const nav = page.getByRole('navigation', { name: 'Sections' })

    for (const name of sectionNames) {
      // Act: open the section and let it settle.
      await nav.getByRole('button', { name, exact: true }).click()
      await expect(page.getByRole('heading', { level: 1, name })).toBeVisible()
      await expect(settled(page, name)).toBeVisible()

      // Assert: axe finds nothing on this section in this theme.
      const violations = await scan(page)
      const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
      expect(violations, `${theme} / ${name}: ${summary}`).toEqual([])
    }
  })
}

for (const theme of themes) {
  test(`no accessibility violations in the first-run setup in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange: a server with no configuration file, which Settings sets up.
    await openFirstRun(page, { width: 1024, height }, theme)

    // Act: a check that does not pass, which offers to write it anyway.
    await page.route('**/api/config/setup', (route) =>
      route.request().method() === 'POST'
        ? route.fulfill(
            problem('check_failed', 'Jira did not accept the token; check it, or keep it anyway'),
          )
        : route.fallback(),
    )
    await page.getByRole('textbox', { name: 'Address' }).fill('https://jira.example.com')
    await page.getByRole('button', { name: 'Write ~/src/api/.workflow.json' }).click()
    await expect(page.getByRole('button', { name: 'Write it anyway' })).toBeVisible()

    // Assert: axe finds nothing on the form or its refusal in this theme.
    const violations = await scan(page)
    const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
    expect(violations, `${theme} / first-run setup: ${summary}`).toEqual([])
  })
}

for (const theme of themes) {
  test(
    `no accessibility violations across the populated sections in the ${theme} theme`,
    {
      tag: '@populated',
    },
    async ({ page }) => {
      // Arrange: pin the theme before the app paints, and open the checked-out
      // issue, so its detail and work story are on screen beside the list.
      await pinTheme(page, theme)
      await page.goto('/')
      await page.getByRole('button', { name: /redact tokens before/i }).click()
      await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()

      for (const name of populatedSectionNames) {
        // Act: open the section and let it settle.
        await openSection(page, name)

        // Assert: axe finds nothing on this section, filled, in this theme.
        const violations = await scan(page)
        const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
        expect(violations, `${theme} / populated ${name}: ${summary}`).toEqual([])
      }
    },
  )
}

// The steps a click opens on the populated build before a write goes out:
// each is a group, named for what it asks.
const confirmSteps = [
  { step: 'push confirmation', section: 'Branch', opener: 'Push branch', group: /^Push / },
  {
    step: 'discard confirmation',
    section: 'Branch',
    opener: 'Discard internal/config/redact.go…',
    group: 'Discard the changes to internal/config/redact.go?',
  },
  {
    step: 'announcement preview',
    section: 'Slack',
    opener: 'Announce to Slack',
    group: 'Announcement preview',
  },
  {
    step: 'summary preview',
    section: 'Summary',
    opener: 'Post…',
    group: 'Summary preview',
  },
  {
    step: 'forget confirmation in People and groups',
    section: 'Settings',
    opener: 'Forget carla…',
    group: 'Forget carla?',
  },
  {
    step: 'remove confirmation in Local data',
    section: 'Settings',
    opener: 'Remove cache…',
    group: 'Remove workflow.db?',
  },
]

for (const theme of themes) {
  for (const { step, section, opener, group } of confirmSteps) {
    test(
      `no accessibility violations in the ${step} in the ${theme} theme`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange: the populated cockpit in this theme, on the step's section.
        await pinTheme(page, theme)
        await page.goto('/')
        await openSection(page, section)

        // Act: open the step.
        await page.getByRole('button', { name: opener }).click()
        await expect(page.getByRole('group', { name: group })).toBeVisible()

        // Assert: axe finds nothing on the step and the section around it.
        const violations = await scan(page)
        const summary = violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
        expect(violations, `${theme} / populated ${step}: ${summary}`).toEqual([])
      },
    )
  }
}
