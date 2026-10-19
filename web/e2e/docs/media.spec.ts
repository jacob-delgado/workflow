import { join } from 'node:path'
import type { Page, TestInfo } from '@playwright/test'
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
// section it is in, and how it is opened from the cockpit.
interface Screen {
  name: string
  file: string
  heading: string
  open: (page: Page) => Promise<void>
}

// section is the screenshot of a section as the rail opens it.
function section(name: string): Screen {
  return {
    name,
    file: name.toLowerCase(),
    heading: name,
    open: (page) => openSection(page, name),
  }
}

// screens are the screenshots the docs show.
const screens: Screen[] = [
  // The cockpit already opens on Issues, so opening it again only pins the
  // section, and its Assert checks what openCockpit left.
  section('Issues'),
  section('Branch'),
  section('Review'),
  {
    name: 'the Slack announcement preview',
    file: 'announcement',
    heading: 'Slack',
    open: openAnnouncementPreview,
  },
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
    expect(await fontsLoaded(page)).toBe(true)
    await page.mouse.move(0, 0)
    await page.screenshot({ path: imagePath(testInfo, screen.file), animations: 'disabled' })
  })
}
