import { test } from '@playwright/test'
import { height, openCockpit, openSection, sectionNames, themes, widths } from './cockpit.ts'

// Screenshots of every populated section, in both themes, at a narrow, a
// middling and a wide window — saved into test-results, which CI uploads, for
// a reviewer to read a visual change by eye. Nothing is compared: a baseline
// would differ by the OS's fonts, and would commit images that a reviewer
// cannot diff anyway.
for (const theme of themes) {
  for (const width of widths) {
    test(
      `screenshots every section in the ${theme} theme at ${String(width)} px`,
      {
        tag: '@populated',
      },
      async ({ page }, testInfo) => {
        // Arrange: the populated cockpit in this theme at this width, with the
        // checked-out issue open so the Issues screen shows its story.
        await openCockpit(page, { width, height }, theme)

        for (const name of sectionNames) {
          // Act: open the section, and let it settle.
          await openSection(page, name)

          // Assert: the screen is saved as drawn, with the pointer parked off
          // the controls so none is caught mid-hover. Under reduced motion
          // every element transitions every property for 0.01ms
          // (web/src/index.css), so an inherited color reaches an icon's
          // strokes a frame or more after the text beside it: the capture
          // finishes those transitions first rather than catching the last
          // section's colors on the way out.
          await page.mouse.move(0, 0)
          await page.screenshot({
            path: testInfo.outputPath(`${String(width)}-${theme}-${name.toLowerCase()}.png`),
            fullPage: true,
            animations: 'disabled',
          })
        }
      },
    )
  }
}
