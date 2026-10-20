import { join } from 'node:path'
import type { Locator, Page, TestInfo } from '@playwright/test'
import { height, openAnnouncementPreview, openCockpit, openSection } from '../support/cockpit.ts'
import { expect, test } from './demo.ts'
import { brisk, commit, goTo, push, stage, startWork, toOpenPull, ciReads } from './loop.ts'

// The docs' screenshots: each screen the docs show, over the real --web
// server running the demo's world (cmd/demo -web), in the dark theme the
// terminal recording shares, at a wide window, written into the docs site's
// assets. Each screen is taken over a demo of its own, driven through the
// page to the state the docs describe. Nothing compares them, since their
// pixels differ by the OS's fonts; each is checked only for what makes it the
// screen it names before it is taken.

// now is the moment every screenshot's browser holds: the demo's day as it
// begins, four in the afternoon in UTC (epoch in internal/demo/world.go). The
// demo dates its reviews and tasks from that moment, so with the browser's
// clock held there, every age, due date and running timer reads the same on
// each run.
const now = new Date('2026-09-16T16:00:00Z')

// size is the window every screenshot is: the widest the specs hold the
// cockpit to.
const size = { width: 1440, height }

// Screen is a screenshot: what it shows, its file's name, the heading of the
// section it is in, the state the loop is taken to first from the cockpit,
// how the screen is then opened, and what must show whole in it, uncut by the
// window's edge. holdCI keeps the demo's end-to-end check running, for a
// screen that shows CI part way through.
interface Screen {
  name: string
  file: string
  heading: string
  holdCI: boolean
  arrange: (page: Page) => Promise<void>
  open: (page: Page) => Promise<void>
  whole: (page: Page) => Locator[]
}

// section is the screenshot of a section as the rail opens it, on the demo as
// it begins.
function section(name: string): Screen {
  return {
    name,
    file: name.toLowerCase(),
    heading: name,
    holdCI: false,
    arrange: () => Promise.resolve(),
    open: (page) => openSection(page, name),
    whole: (page) => [page.getByRole('heading', { level: 1, name })],
  }
}

// workStory is the selected issue's work story, in its detail.
function workStory(page: Page): Locator {
  return page.getByRole('article').getByRole('region', { name: 'Work story' })
}

// openedWithCIRunning takes the loop to #128 open, three of its four checks
// passed and the end-to-end one held running.
async function openedWithCIRunning(page: Page): Promise<void> {
  await toOpenPull(page, brisk)
  await ciReads(page, brisk, 3)
}

// issues is the Issues section with the selected issue's detail scrolled to
// its work story, so the whole of it, the start it offers and every stage
// with its line, fits the window's height. The key, title, type and people
// it scrolls past are the selected row's beside it; nothing in the image
// shows the detail scrolled, so the page's prose beside it says so.
const issues: Screen = {
  ...section('Issues'),
  open: async (page) => {
    await openSection(page, 'Issues')
    await workStory(page).evaluate((story) => {
      story.scrollIntoView({ block: 'start' })
    })
  },
  whole: (page) => [workStory(page).getByRole('listitem').last()],
}

// branch is the Branch section from its heading down to the working tree's
// last change, its Show diff button included, once work on PROJ-412 has two
// commits, the older pushed, and one change still to commit. The commit form
// below it, which the page's Branch prose describes, begins just above the
// window's edge, so it is taken out, leaving the space below blank, rather
// than shown cut through its labels: display, not visibility, since a
// visibility the labels inherit would still be mid-transition when the edge
// is checked.
const branch: Screen = {
  ...section('Branch'),
  arrange: async (page) => {
    await startWork(page, brisk)
    await goTo(page, brisk, 'Branch')
    await stage(page, brisk, 'internal/wiring/reqlog.go')
    await commit(page, brisk, 'redact tokens in the request log')
    await push(page, brisk)
    await stage(page, brisk, 'internal/config/redact.go')
    await commit(page, brisk, 'mask the token in each logged header')
  },
  open: async (page) => {
    await openSection(page, 'Branch')
    await page.addStyleTag({
      content: 'form[aria-label="Commit staged changes"] { display: none; }',
    })
  },
  whole: (page) => [
    page.getByRole('region', { name: 'Working tree' }).getByRole('listitem').last(),
  ],
}

// review is the Review section once #128 is open, three of its checks passed
// and the end-to-end one still running.
const review: Screen = {
  ...section('Review'),
  holdCI: true,
  arrange: openedWithCIRunning,
}

// announcement is the Slack section with the announcement preview open while
// CI still runs, the preview whole and, under it, the announcement armed when
// #128 opened, waiting for CI to pass. The Channels list below begins under
// the window's edge.
const announcement: Screen = {
  name: 'the Slack announcement preview',
  file: 'announcement',
  heading: 'Slack',
  holdCI: true,
  arrange: openedWithCIRunning,
  open: openAnnouncementPreview,
  whole: (page) => [
    page.getByRole('group', { name: 'Announcement preview' }),
    page.getByRole('button', { name: 'Stop waiting' }),
  ],
}

// screens are the screenshots the docs show.
const screens: Screen[] = [
  // The cockpit already opens on Issues, so opening it again pins the
  // section before its detail is scrolled.
  issues,
  branch,
  review,
  announcement,
  section('Summary'),
  section('Tasks'),
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

// fontsLoaded waits for the page's fonts and says whether its own face is
// among them, so no screenshot draws its words in a fallback font.
function fontsLoaded(page: Page): Promise<boolean> {
  return page.evaluate(async () => {
    await document.fonts.ready

    return document.fonts.check('16px "Recursive Variable"')
  })
}

// cutAtTheEdge is every run of text the window's bottom edge cuts through:
// a line that begins above the edge and ends below it, shown, and not
// already clipped above the edge by a pane that scrolls.
function cutAtTheEdge(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const edge = window.innerHeight
    // shownBottom is how far down a line of text below the element shows,
    // once every pane around it that clips its overflow has cut it.
    const shownBottom = (element: Element, bottom: number): number => {
      let shown = bottom
      for (let around: Element | null = element; around !== null; around = around.parentElement) {
        if (getComputedStyle(around).overflowY !== 'visible') {
          shown = Math.min(shown, around.getBoundingClientRect().bottom)
        }
      }

      return shown
    }
    const cut: string[] = []
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
      const text = node.textContent?.trim() ?? ''
      const holder = node.parentElement
      if (text === '' || holder?.checkVisibility({ visibilityProperty: true }) !== true) {
        continue
      }
      const range = document.createRange()
      range.selectNodeContents(node)
      const crossing = [...range.getClientRects()].some(
        (line) => line.top < edge && line.bottom > edge && shownBottom(holder, line.bottom) >= edge,
      )
      if (crossing) {
        cut.push(text)
      }
    }

    return cut
  })
}

// The page's Content-Security-Policy refuses the <style> element addStyleTag
// adds, and the commit form the Branch screenshot hides is drawn anew as the
// stream updates, so a style set on the element itself would not last. The
// screenshots are of the app's look, which the policy does not change.
test.use({ bypassCSP: true })

for (const screen of screens) {
  test.describe(screen.name, () => {
    test.use({ holdCI: screen.holdCI })

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
      for (const whole of screen.whole(page)) {
        await expect(whole).toBeInViewport({ ratio: 1 })
      }
      expect(await cutAtTheEdge(page)).toEqual([])
      expect(await fontsLoaded(page)).toBe(true)
      await page.mouse.move(0, 0)
      await page.screenshot({ path: imagePath(testInfo, screen.file), animations: 'disabled' })
    })
  })
}
