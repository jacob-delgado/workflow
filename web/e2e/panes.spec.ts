import { expect, test, type Locator, type Page } from '@playwright/test'
import { height, openCockpit, openSection, widths } from './cockpit.ts'

// The issue panes in the populated cockpit: where the detail sits against the
// list, what scrolls, where each pane opens, and the focus rings a pane's edge
// could clip.

interface Box {
  x: number
  y: number
  width: number
  height: number
}

// boxOf is where an element is drawn; one that is not fails the test.
async function boxOf(locator: Locator): Promise<Box> {
  const box = await locator.boundingBox()
  expect(box, 'drawn').not.toBeNull()

  return box ?? { x: 0, y: 0, width: 0, height: 0 }
}

// placement says where the detail starts against the list: beside it, at or
// past its right edge; under it, at or below its bottom; or neither.
function placement(list: Box, detail: Box): string {
  if (detail.x >= list.x + list.width) {
    return 'beside'
  }

  return detail.y >= list.y + list.height ? 'under' : 'overlapping'
}

// Below lg the open issue's detail sits under the issue list; from lg, beside
// it. Either way the list scrolls in its own pane rather than growing the page,
// and from lg the panes fit the window, so the content does not scroll at all.
const paneCases = [
  { width: 640, detail: 'under', contentScrolls: true },
  { width: 1024, detail: 'beside', contentScrolls: false },
  { width: 1440, detail: 'beside', contentScrolls: false },
]

for (const { width, detail, contentScrolls } of paneCases) {
  test(
    `the detail sits ${detail} the issue list at ${String(width)} px, which scrolls in its pane`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange: the checked-out issue open, in a window shorter than the list.
      await openCockpit(page, { width, height: 600 }, 'dark')
      const list = page.getByRole('list', { name: 'Issues' })

      // Act: measure the list, the detail, and what scrolls.
      const drawn = placement(await boxOf(list), await boxOf(page.getByRole('article')))
      const listScrolls = await list.evaluate((pane) => pane.scrollHeight > pane.clientHeight)
      const scrolls = await page
        .getByRole('main')
        .evaluate((main) => main.scrollHeight > main.clientHeight)

      // Assert
      expect(drawn).toBe(detail)
      expect(listScrolls, 'the list scrolls in its pane').toBe(true)
      expect(scrolls, 'the content scrolls').toBe(contentScrolls)
    },
  )
}

// From lg the detail scrolls in its own pane, and each issue opens at its top
// however far down the last one was read.
test(
  'an issue opens at the top of its pane at 1440 px',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange: the checked-out issue read to its end, in a window shorter than it.
    await openCockpit(page, { width: 1440, height: 600 }, 'dark')
    await page.getByRole('article').evaluate((article) => {
      const pane = article.parentElement
      if (pane !== null) {
        pane.scrollTop = pane.scrollHeight
      }
    })

    // Act
    await page.getByRole('button', { name: /refuse to start/i }).click()

    // Assert
    await expect(page.getByRole('link', { name: /open in jira/i })).toBeVisible()
    const heading = page.getByRole('heading', { level: 2, name: /refuse to start/i })
    await expect(heading).toBeInViewport({ ratio: 1 })
  },
)

// The selection outlives a change of section, and the list's pane opens with
// the selected issue in view, however far down the list it sits.
test(
  'the selected issue is in view as the issues reopen at 640 px',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange: the list's last issue selected, and another section opened.
    await openCockpit(page, { width: 640, height }, 'dark')
    const last = page.getByRole('button', { name: /^PROJ-377/ })
    await last.click()
    await openSection(page, 'Branch')

    // Act
    await openSection(page, 'Issues')

    // Assert
    await expect(last).toBeInViewport({ ratio: 1 })
  },
)

// ringFits says whether a control's focus ring, two pixels round its box, is
// drawn whole inside the nearest part of the page around it that scrolls.
function ringFits(control: HTMLElement): boolean {
  let pane = control.parentElement
  while (pane !== null && getComputedStyle(pane).overflowY === 'visible') {
    pane = pane.parentElement
  }
  if (pane === null) {
    return true
  }

  const ring = 2
  const box = control.getBoundingClientRect()
  const area = pane.getBoundingClientRect()
  const left = area.left + pane.clientLeft
  const top = area.top + pane.clientTop

  return (
    box.left - ring >= left &&
    box.top - ring >= top &&
    box.right + ring <= left + pane.clientWidth &&
    box.bottom + ring <= top + pane.clientHeight
  )
}

function firstIssue(page: Page): Locator {
  return page.getByRole('list', { name: 'Issues' }).getByRole('button').first()
}

function jiraLink(page: Page): Locator {
  return page.getByRole('link', { name: /open in jira/i })
}

// Each pane keeps a few pixels inside its edge, so a control against it has
// its focus ring drawn whole: a row in the list at every width, and the
// detail's link from lg, where the detail scrolls in a pane of its own.
const ringCases = [
  ...widths.map((width) => ({ width, name: 'the first issue', control: firstIssue })),
  { width: 1024, name: "the detail's link", control: jiraLink },
  { width: 1440, name: "the detail's link", control: jiraLink },
]

for (const { width, name, control } of ringCases) {
  test(
    `the focus ring of ${name} is drawn whole in its pane at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await openCockpit(page, { width, height }, 'dark')

      // Act
      await control(page).focus()

      // Assert
      expect(await control(page).evaluate(ringFits)).toBe(true)
    },
  )
}
