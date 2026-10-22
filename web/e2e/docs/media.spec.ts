import { join } from 'node:path'
import type { Locator, Page, TestInfo } from '@playwright/test'
import { height, openAnnouncementPreview, openCockpit, openSection } from '../support/cockpit.ts'
import { cutAtTheEdge, fontsLoaded, now, stillAnimating } from './capture.ts'
import { expect, test } from './demo.ts'
import {
  brisk,
  ciReads,
  commit,
  goTo,
  moveIntoProgress,
  stage,
  startWork,
  toOpenPull,
  workStory,
} from './loop.ts'

// The docs' screenshots: each screen the docs show, over the real --web
// server running the demo's world (cmd/demo -web), in the dark theme the
// terminal recording shares, at a wide window, written into the docs site's
// assets. Each screen is taken over a demo of its own, driven through the
// page to the state the docs describe. Nothing compares them, since their
// pixels differ by the OS's fonts; each is checked only for what makes it the
// screen it names before it is taken.

// size is the window every screenshot is: the widest the specs hold the
// cockpit to.
const size = { width: 1440, height }

// Screen is a screenshot: what it shows, its file's name, the heading of the
// section it is in, the world its demo departs into, the state the loop is
// taken to first from the cockpit, how the screen is then opened, what it
// must show for the story the docs tell there, and what must show whole in
// it, uncut by an edge of the window or of its pane.
interface Screen {
  name: string
  file: string
  heading: string
  world: { holdCI: boolean; autoAnnounce: boolean }
  arrange: (page: Page) => Promise<void>
  open: (page: Page) => Promise<void>
  agrees: (page: Page) => Promise<void>
  whole: (page: Page) => Locator[]
}

// section is the screenshot of a section as the rail opens it, on the demo as
// it begins.
function section(name: string): Screen {
  return {
    name,
    file: name.toLowerCase(),
    heading: name,
    world: { holdCI: false, autoAnnounce: true },
    arrange: () => Promise.resolve(),
    open: (page) => openSection(page, name),
    agrees: () => Promise.resolve(),
    whole: (page) => [page.getByRole('heading', { level: 1, name })],
  }
}

// hide takes what matches a selector out of the page, leaving its space to
// what follows: display, not visibility, since a visibility what it holds
// inherits would still be mid-transition when the edges are checked.
async function hide(page: Page, selector: string): Promise<void> {
  await page.addStyleTag({ content: `${selector} { display: none; }` })
}

// startedInProgress takes the loop to work started on PROJ-412 and the
// issue moved into progress, as the story has it once work begins.
async function startedInProgress(page: Page): Promise<void> {
  await startWork(page, brisk)
  await moveIntoProgress(page, brisk)
}

// openedWithCIRunning takes the loop to #128 open, PROJ-412 in progress,
// three of its four checks passed and the end-to-end one held running.
async function openedWithCIRunning(page: Page): Promise<void> {
  await toOpenPull(page, brisk)
  await ciReads(page, brisk, 3)
}

// toTheTop scrolls the pane around an element, the nearest that scrolls,
// until the element's top meets the pane's own, past the scroll padding
// scrollIntoView would leave above it.
async function toTheTop(target: Locator): Promise<void> {
  await target.evaluate((element) => {
    let pane = element.parentElement
    while (pane !== null && !/^(auto|scroll)$/.test(getComputedStyle(pane).overflowY)) {
      pane = pane.parentElement
    }
    pane?.scrollBy({ top: element.getBoundingClientRect().top - pane.getBoundingClientRect().top })
  })
}

// issues is the Issues section on the demo as it begins, PROJ-412 at To Do
// and its work not started, with the detail scrolled until its Open in Jira
// link meets the pane's top, so its actions, and below them the whole work
// story, the start it offers and every stage with its line, show whole. The
// key, title and type it scrolls past are the selected row's beside it; the
// reporter and assignee are scrolled out of view, and the Tasks card below
// the story is out of view too: the pane is too short to hold the story whole
// with either, and the story is what the screen is for. Nothing in the image
// shows the detail scrolled, so the page's prose beside it says so.
const issues: Screen = {
  ...section('Issues'),
  open: async (page) => {
    await openSection(page, 'Issues')
    await toTheTop(page.getByRole('article').getByRole('link', { name: /open in jira/i }))
  },
  agrees: async (page) => {
    const detail = page.getByRole('article')
    await expect(detail.getByText('To Do', { exact: true }).first()).toBeVisible()
    await expect(detail.getByRole('button', { name: 'Start work', exact: true })).toBeVisible()
  },
  whole: (page) => [
    page.getByRole('button', { name: 'Change status', exact: true }),
    workStory(page),
  ],
}

// The redaction's three changes, as the demo's work tree holds them: two
// files it modifies and the test it adds, which git does not track yet.
const requestLogFile = 'internal/wiring/reqlog.go'
const redactFile = 'internal/config/redact.go'
const requestLogTestFile = 'internal/wiring/reqlog_test.go'

// branch is the Branch section from its heading down to the working tree's
// last change, its Show diff button included, mid-loop as the film tells
// it, before the push: work on PROJ-412 started and moved into progress, one
// of the redaction's three changes committed as 4f2c9e1, the film's own
// first commit, the branch not pushed yet, a second change staged and the
// test it adds still untracked, each with its buttons. The commit form below
// it, which the page's Branch prose describes, begins just above the
// window's edge, so it is taken out, leaving the space below blank, rather
// than shown cut through its labels. The staging's notice stands under the
// file it staged, which it agrees with.
const branch: Screen = {
  ...section('Branch'),
  arrange: async (page) => {
    await startedInProgress(page)
    await goTo(page, brisk, 'Branch')
    await stage(page, brisk, requestLogFile)
    await commit(page, brisk, 'redact tokens in the request log')
    await stage(page, brisk, redactFile)
  },
  open: async (page) => {
    await openSection(page, 'Branch')
    await hide(page, 'form[aria-label="Commit staged changes"]')
  },
  agrees: async (page) => {
    const tree = page.getByRole('region', { name: 'Working tree' })
    await expect(page.getByRole('definition').filter({ hasText: /^not pushed yet$/ })).toBeVisible()
    await expect(
      page.getByRole('region', { name: 'Commits', exact: true }).getByRole('listitem'),
    ).toHaveText([/^4f2c9e1\s*fix\(config\): redact tokens in the request log$/])
    await expect(
      tree.getByRole('button', { name: `Unstage ${redactFile}`, exact: true }),
    ).toBeVisible()
    await expect(
      tree.getByRole('button', { name: `Stage ${requestLogTestFile}`, exact: true }),
    ).toBeVisible()
  },
  whole: (page) => [
    page.getByRole('region', { name: 'Working tree' }).getByRole('listitem').last(),
  ],
}

// moveToReview is the offer #128 opening leaves on the Review section: move
// PROJ-412, in progress, on to In Review.
function moveToReview(page: Page): Locator {
  return page.getByRole('button', { name: 'Move PROJ-412 to In Review', exact: true })
}

// review is the Review section once #128 is open, PROJ-412 in progress and
// offered on to In Review, three of its checks passed and the end-to-end one
// still running.
const review: Screen = {
  ...section('Review'),
  world: { holdCI: true, autoAnnounce: true },
  arrange: openedWithCIRunning,
  whole: (page) => [page.getByRole('heading', { level: 1, name: 'Review' }), moveToReview(page)],
}

// announcement is the Slack section with the announcement composer open while
// CI still runs, on a demo that arms no announcement as #128 opens, so the
// composer is all the section offers: the preview whole, with Announce when
// CI passes under it. The Channels list below it, which the composer's own
// channel choice repeats, is taken out, so it cannot straddle the window's
// edge.
const announcement: Screen = {
  name: 'the Slack announcement preview',
  file: 'announcement',
  heading: 'Slack',
  world: { holdCI: true, autoAnnounce: false },
  arrange: openedWithCIRunning,
  open: async (page) => {
    await openAnnouncementPreview(page)
    await hide(page, 'section[aria-labelledby="channels-heading"]')
  },
  agrees: async (page) => {
    await expect(page.getByText(/Armed when/)).toHaveCount(0)
    await expect(page.getByRole('button', { name: 'Stop waiting' })).toHaveCount(0)
  },
  whole: (page) => [
    page.getByRole('group', { name: 'Announcement preview' }),
    page.getByRole('button', { name: 'Announce when CI passes', exact: true }),
  ],
}

// summary is the Summary section once work on PROJ-412 has started and the
// issue moved into progress.
const summary: Screen = {
  ...section('Summary'),
  arrange: startedInProgress,
}

// firstTask is the first task the Tasks section lists, under its first
// heading when it heads its lists.
function firstTask(page: Page): Locator {
  return page
    .getByRole('main')
    .getByRole('list', { name: /^(For my issues|Tasks)$/ })
    .first()
    .getByRole('button')
    .first()
}

// tasks is the Tasks section once work on PROJ-412 has started and the issue
// moved into progress: its task, not started, listed first and selected, its
// detail beside the list. The lines that annotate and modify it, which the
// page's Tasks prose describes, end the detail at the pane's bottom, so they
// are taken out rather than shown cut through.
const tasks: Screen = {
  ...section('Tasks'),
  arrange: startedInProgress,
  open: async (page) => {
    await openSection(page, 'Tasks')
    await hide(page, 'article[aria-labelledby="task-detail-heading"] > div:last-child')
  },
  agrees: async (page) => {
    await expect(firstTask(page)).toHaveAttribute('aria-current', 'true')
    await expect(firstTask(page)).toHaveAccessibleName(/^pending\b.*PROJ-412/)
  },
  whole: (page) => [page.getByRole('heading', { level: 1, name: 'Tasks' }), firstTask(page)],
}

// screens are the screenshots the docs show.
const screens: Screen[] = [
  // The cockpit already opens on Issues, so opening it again pins the
  // section before its detail is scrolled.
  issues,
  branch,
  review,
  announcement,
  summary,
  tasks,
]

// imagePath is where the screenshot of a file's name goes: the directory the
// run's project names.
function imagePath(testInfo: TestInfo, file: string): string {
  const mediaDir: unknown = testInfo.project.metadata.mediaDir
  if (typeof mediaDir !== 'string') {
    throw new Error('the docs-media project names no mediaDir')
  }

  return join(mediaDir, `web-${file}.png`)
}

// The page's Content-Security-Policy refuses the <style> element addStyleTag
// adds, and what a screenshot hides is drawn anew as the stream updates, so a
// style set on the element itself would not last. The screenshots are of the
// app's look, which the policy does not change.
test.use({ bypassCSP: true })

for (const screen of screens) {
  test.describe(screen.name, () => {
    test.use(screen.world)

    test(`screenshots ${screen.name} for the docs`, async ({ page }, testInfo) => {
      // Arrange
      await page.clock.setFixedTime(now)
      await openCockpit(page, size, 'dark')
      await screen.arrange(page)

      // Act
      await screen.open(page)

      // Assert
      // The pointer is parked off the controls so none is caught mid-hover,
      // and the capture finishes any transition first, as screens.spec.ts
      // says.
      await expect(page.getByRole('heading', { level: 1, name: screen.heading })).toBeInViewport()
      await screen.agrees(page)
      for (const whole of screen.whole(page)) {
        await expect(whole).toBeInViewport({ ratio: 1 })
      }
      expect(await cutAtTheEdge(page)).toEqual([])
      expect(await fontsLoaded(page)).toBe(true)
      await expect.poll(() => stillAnimating(page)).toEqual([])
      await page.mouse.move(0, 0)
      await page.screenshot({ path: imagePath(testInfo, screen.file), animations: 'disabled' })
    })
  })
}
