import {
  confirmSteps,
  height,
  openFirstRun,
  openSection,
  pinTheme,
  sectionNames,
  sectionsWith,
  themes,
} from './support/cockpit.ts'
import { expect, problem, test } from './support/fixtures.ts'
import { axeViolations } from './support/reachable.ts'

// Every section, in both themes: a light theme is only real once its contrast
// holds up, so the scan runs the whole cockpit in each. The hermetic build has
// no messaging set up, so its section is named Messaging.
const hermeticSections = sectionsWith('Messaging')

// Scan the resting state, not mid-animation frames: reduced motion collapses
// transitions to instant, so axe never samples a half-faded element (whose
// transient blended colors are a false contrast failure).
test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
})

// unreachable is the answer a read gets when the service behind it is down.
const unreachable = problem(
  'unreachable',
  'the service could not be reached; check the network, then try again',
)

for (const theme of themes) {
  for (const name of hermeticSections) {
    test(`no accessibility violations in ${name} in the ${theme} theme`, async ({ page }) => {
      // Arrange
      // Pin the theme before the app paints, so the whole run is in it, and
      // answer the health read as a --dry-run server would, so the read-only
      // banner is on screen for the scan (the hermetic server has no API). The
      // review queue's read, the task list's, the repositories' and the
      // configuration's fail as unreachable, and every other read the section
      // makes as a route the server does not have (the test support/fixtures.ts
      // makes), so each reason and its Try again are scanned; the populated
      // build scans the queue, the tasks and the form themselves.
      await pinTheme(page, theme)
      await page.route('**/api/health', (route) =>
        route.fulfill({
          json: { version: '1.2.3', dry_run: true, forge_noun: 'pull request', forge_sigil: '#' },
        }),
      )
      await page.route('**/api/reviews', (route) => route.fulfill(unreachable))
      await page.route('**/api/tasks', (route) => route.fulfill(unreachable))
      await page.route('**/api/repositories', (route) => route.fulfill(unreachable))
      await page.route('**/api/config', (route) => route.fulfill(unreachable))
      await page.goto('/')
      await expect(page.getByText(/every write is held back/i)).toBeVisible()

      // Act
      await openSection(page, name)

      // Assert
      expect(await axeViolations(page)).toBe('')
    })
  }
}

for (const theme of themes) {
  test(`no accessibility violations in the first-run setup in the ${theme} theme`, async ({
    page,
  }) => {
    // Arrange
    await openFirstRun(page, { width: 1024, height }, theme)

    // Act
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

    // Assert
    expect(await axeViolations(page), `${theme} / first-run setup`).toBe('')
  })
}

for (const theme of themes) {
  for (const name of sectionNames) {
    test(
      `no accessibility violations in the populated ${name} in the ${theme} theme`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange
        // The checked-out issue is opened, so its detail and work story are
        // scanned beside the list.
        await pinTheme(page, theme)
        await page.goto('/')
        await page.getByRole('button', { name: /redact tokens before/i }).click()
        await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()

        // Act
        await openSection(page, name)

        // Assert
        expect(await axeViolations(page)).toBe('')
      },
    )
  }
}

for (const theme of themes) {
  for (const { step, section, opener, group } of confirmSteps) {
    test(
      `no accessibility violations in the ${step} in the ${theme} theme`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange
        await pinTheme(page, theme)
        await page.goto('/')
        await openSection(page, section)

        // Act
        await page.getByRole('button', { name: opener }).click()
        await expect(page.getByRole('group', { name: group })).toBeVisible()

        // Assert
        expect(await axeViolations(page), `${theme} / populated ${step}`).toBe('')
      },
    )
  }
}
