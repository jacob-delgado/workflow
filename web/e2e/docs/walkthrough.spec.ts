import type { Page } from '@playwright/test'
import { pinTheme, readings } from '../support/cockpit.ts'
import { fontsLoaded, nextFrame, now, stillAnimating } from './capture.ts'
import { expect, metadataOf, test } from './demo.ts'
import { prepareFilm } from './film.ts'
import {
  announcedOnGreen,
  armedForCI,
  ciReads,
  goTo,
  pickIssue,
  storyDone,
  toOpenPull,
} from './loop.ts'

// The docs' walkthrough of the web interface: the whole loop over the real
// --web server running the demo's world, filmed at a person's pace, from an
// issue picked to its announcement posted on its own once CI passed, and
// written as frames for scripts/docs-media.sh to encode. The demo's CI keeps
// its own schedule here, so the film shows it run and pass.

// captions are the walkthrough's beats, in the order the loop reaches them.
const captions = [
  'PROJ-412 picked, its work not started',
  'Started work on fix/PROJ-412-redact-tokens-in-the-request-log',
  'PROJ-412 moved to In Progress',
  'Every change staged',
  'Committed fix(config): redact tokens in the request log',
  'Pushed fix/PROJ-412-redact-tokens-in-the-request-log',
  'The pull request composed, its reviewers proposed',
  '#128 opened, its announcement waiting for CI',
  'The announcement armed when #128 opened, waiting for CI',
  'CI 4 of 4 done',
  'Announced to #dev-workflow once CI passed',
  "PROJ-412's work story, every stage done",
]

// live waits until the page stands as a person first sees it, so the film
// opens on it rather than on a blank page, the connecting screen or a
// header and rail still filling in: the stream live, the header naming the
// build and the repository, the rail naming Slack, the Issues list's View
// drawn, nothing reading, the page's own face loaded, nothing moving, and
// two frames drawn since, so what the page last changed is on screen.
async function live(page: Page, version: string): Promise<void> {
  const header = page.getByRole('banner')
  await expect(header.getByRole('status').filter({ hasText: /^Live$/ })).toBeVisible()
  await expect(header.getByText(version, { exact: true })).toBeVisible()
  await expect(header.getByRole('button', { name: /^Working in / })).toBeVisible()
  const rail = page.getByRole('navigation', { name: 'Sections' })
  await expect(rail.getByRole('button', { name: 'Slack', exact: true })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'View' })).toBeVisible()
  await expect(readings(page)).toHaveCount(0)
  expect(await fontsLoaded(page)).toBe(true)
  await expect.poll(() => stillAnimating(page)).toEqual([])
  await nextFrame(page)
  await nextFrame(page)
}

// The page's Content-Security-Policy refuses the inline style the pointer
// is drawn with; the film is of the app's look, which the policy does not
// change.
test.use({ bypassCSP: true })

test('films the loop from an issue to its announcement for the docs', async ({
  page,
}, testInfo) => {
  // Arrange
  test.setTimeout(180_000)
  await page.clock.setFixedTime(now)
  await pinTheme(page, 'dark')
  const camera = await prepareFilm(page, metadataOf(testInfo, 'walkthroughDir'))
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Issues' })).toBeVisible()
  await live(page, metadataOf(testInfo, 'version'))
  const film = await camera.roll()

  // Act
  await pickIssue(page, film.pace)
  await toOpenPull(page, film.pace)
  await goTo(page, film.pace, 'Slack')
  await armedForCI(page, film.pace)
  await goTo(page, film.pace, 'Review')
  await ciReads(page, film.pace, 4)
  await goTo(page, film.pace, 'Slack')
  await announcedOnGreen(page, film.pace)
  await goTo(page, film.pace, 'Issues')
  await storyDone(page, film.pace)
  await film.finish()

  // Assert
  expect(film.captions()).toEqual(captions)
  expect(film.stalls()).toEqual([])
})
