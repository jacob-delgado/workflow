import type { Page } from '@playwright/test'
import {
  height,
  openCockpit,
  openSection,
  sectionNames,
  themes,
  widths,
} from './support/cockpit.ts'
import { expect, test } from './support/fixtures.ts'

// Screenshots of every populated section, in both themes, at a narrow, a
// middling and a wide window — saved into test-results, which CI uploads, for
// a reviewer to read a visual change by eye. Nothing is compared: a baseline
// would differ by the OS's fonts, and would commit images that a reviewer
// cannot diff anyway.

// overflowing is how far the content, or the furthest a pane in it that grows
// with the window, scrolls past what the window shows. A pane capped at a
// height, and a text area, grow with nothing, so they are left out.
function overflowing(page: Page): Promise<number> {
  return page.getByRole('main').evaluate((main) =>
    Math.max(
      ...[main, ...main.querySelectorAll<HTMLElement>('*')]
        .filter((part) => {
          const { overflowY, maxHeight } = getComputedStyle(part)
          const pane = /auto|scroll/.test(overflowY) && maxHeight === 'none'

          return part === main || (pane && part.localName !== 'textarea')
        })
        .map((part) => part.scrollHeight - part.clientHeight),
    ),
  )
}

// fitToContent grows the window by as much as the section scrolls, until it
// scrolls no more: the page itself holds still under its header, and a
// full-page capture sees only the window. A part that draws late, as a read of
// its own lands, grows it again.
async function fitToContent(page: Page, width: number): Promise<void> {
  await expect
    .poll(async () => {
      const more = await overflowing(page)
      if (more > 0) {
        await page.setViewportSize({
          width,
          height: (page.viewportSize()?.height ?? height) + more,
        })
      }

      return more
    })
    .toBe(0)
}

for (const theme of themes) {
  for (const width of widths) {
    for (const name of sectionNames) {
      test(
        `screenshots ${name} in the ${theme} theme at ${String(width)} px`,
        { tag: '@populated' },
        async ({ page }, testInfo) => {
          // Arrange
          await openCockpit(page, { width, height }, theme)

          // Act
          await openSection(page, name)
          await fitToContent(page, width)

          // Assert
          // The pointer is parked off the controls so none is caught mid-hover.
          // Under reduced motion every element transitions every property for
          // 0.01ms (web/src/index.css), so an inherited color reaches an icon's
          // strokes a frame or more after the text beside it: the capture
          // finishes those transitions first rather than catching the colors of
          // the section it left on the way out.
          await expect(page.getByRole('heading', { level: 1, name })).toBeInViewport()
          await page.mouse.move(0, 0)
          await page.screenshot({
            path: testInfo.outputPath(`${String(width)}-${theme}-${name.toLowerCase()}.png`),
            fullPage: true,
            animations: 'disabled',
          })
        },
      )
    }
  }
}
