import { join } from 'node:path'
import type { Locator, Page, TestInfo } from '@playwright/test'
import { height, openAnnouncementPreview, openCockpit, openSection } from '../support/cockpit.ts'
import { expect, test } from '../support/fixtures.ts'

// The docs' screenshots: each screen the docs show, on the populated mock
// build, in the dark theme the terminal recording shares, at a wide window,
// written into the docs site's assets. Nothing compares them, since their
// pixels differ by the OS's fonts; each is checked only for what makes it the
// screen it names before it is taken.

// now is the moment every screenshot shows: mid-afternoon in UTC on the day
// the mockup's Summary calls today (web/src/dev/mockActivity.ts). The mockup
// dates its reviews and tasks relative to the clock, so with it held, every
// age, due date and running timer reads the same on each run.
const now = new Date('2026-09-16T15:00:00Z')

// size is the window every screenshot is: the widest the specs hold the
// cockpit to.
const size = { width: 1440, height }

// Screen is a screenshot: what it shows, its file's name, the heading of the
// section it is in, how it is opened from the cockpit, and what must show
// whole in it, uncut by the window's edge.
interface Screen {
  name: string
  file: string
  heading: string
  open: (page: Page) => Promise<void>
  whole: (page: Page) => Locator[]
}

// section is the screenshot of a section as the rail opens it.
function section(name: string): Screen {
  return {
    name,
    file: name.toLowerCase(),
    heading: name,
    open: (page) => openSection(page, name),
    whole: (page) => [page.getByRole('heading', { level: 1, name })],
  }
}

// issues is the Issues section with the selected issue's detail scrolled to
// its reporter, so the whole of its work story, every stage and its line,
// fits the window's height. The key, title and type it scrolls past are the
// selected row's beside it; nothing in the image shows the detail scrolled,
// so the page's prose beside it says so.
const issues: Screen = {
  ...section('Issues'),
  open: async (page) => {
    await openSection(page, 'Issues')
    await page
      .getByRole('article')
      .getByText('Reporter', { exact: true })
      .evaluate((term) => {
        term.parentElement?.scrollIntoView({ block: 'start' })
      })
  },
  whole: (page) => [
    page
      .getByRole('article')
      .getByRole('region', { name: 'Work story' })
      .getByRole('listitem')
      .last(),
  ],
}

// branch is the Branch section from its heading down to the working tree's
// last change, its Show diff button included. The commit form below it, which
// the page's Branch prose describes, begins just above the window's edge, so
// it is taken out, leaving the space below blank, rather than shown cut
// through its labels: display, not visibility, since a visibility the labels
// inherit would still be mid-transition when the edge is checked.
const branch: Screen = {
  ...section('Branch'),
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

// channels is the Slack section's list of the channels it can post to.
function channels(page: Page): Locator {
  return page.getByRole('region', { name: 'Channels' })
}

// announcement is the Slack section with the announcement preview open,
// scrolled to its end, so its Channels list ends above the pane's padding
// rather than on the window's edge, and the whole section shows.
const announcement: Screen = {
  name: 'the Slack announcement preview',
  file: 'announcement',
  heading: 'Slack',
  open: async (page) => {
    await openAnnouncementPreview(page)
    await page.getByRole('main').evaluate((main) => {
      main.scrollTop = main.scrollHeight
    })
  },
  whole: (page) => [
    page.getByRole('group', { name: 'Announcement preview' }),
    channels(page).getByRole('listitem').last(),
  ],
}

// screens are the screenshots the docs show.
const screens: Screen[] = [
  // The cockpit already opens on Issues, so opening it again pins the
  // section before its detail is scrolled.
  issues,
  branch,
  section('Review'),
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
// mock's stream updates, so a style set on the element itself would not last.
// The screenshots are of the app's look, which the policy does not change.
test.use({ bypassCSP: true })

for (const screen of screens) {
  test(`screenshots ${screen.name} for the docs`, async ({ page }, testInfo) => {
    // Arrange
    await page.clock.setFixedTime(now)
    await openCockpit(page, size, 'dark')

    // Act
    await screen.open(page)

    // Assert
    // The pointer is parked off the controls so none is caught mid-hover, and
    // the capture finishes any transition first, as screens.spec.ts says.
    await expect(page.getByRole('heading', { level: 1, name: screen.heading })).toBeInViewport()
    for (const whole of screen.whole(page)) {
      await expect(whole).toBeInViewport({ ratio: 1 })
    }
    expect(await cutAtTheEdge(page)).toEqual([])
    expect(await fontsLoaded(page)).toBe(true)
    await page.mouse.move(0, 0)
    await page.screenshot({ path: imagePath(testInfo, screen.file), animations: 'disabled' })
  })
}
