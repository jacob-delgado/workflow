import { pinTheme } from '../support/cockpit.ts'
import { now } from './capture.ts'
import { expect, metadataOf, test } from './demo.ts'
import { startFilm } from './film.ts'
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
  const film = await startFilm(page, metadataOf(testInfo, 'walkthroughDir'))
  await page.goto('/')
  await expect(page.getByRole('heading', { level: 1, name: 'Issues' })).toBeVisible()

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
