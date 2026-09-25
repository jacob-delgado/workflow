import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'
import { height, openCockpit, openSection, sectionNames, themes, widths } from './cockpit.ts'

// The populated cockpit at a narrow, a middling and a wide window, in both
// themes: nothing scrolls sideways, the page does not scroll at all, every
// control the keyboard can reach is in view once it has focus, and axe finds
// nothing.

// A control counts as in view when this much of it is, allowing a rounding
// pixel at a scrolled edge.
const inView = 0.99

// sidewaysScrollers names what scrolls sideways: the page, or any part of it.
function sidewaysScrollers(): string[] {
  const scrollers: string[] = []
  const page = document.scrollingElement
  if (page !== null && page.scrollWidth > window.innerWidth) {
    scrollers.push(`the page (${String(page.scrollWidth)} px in ${String(window.innerWidth)})`)
  }

  for (const element of document.querySelectorAll<HTMLElement>('body *')) {
    const { overflowX } = getComputedStyle(element)
    const scrolls = overflowX === 'auto' || overflowX === 'scroll'
    if (scrolls && element.scrollWidth > element.clientWidth) {
      const label = element.getAttribute('aria-label') ?? element.id
      scrollers.push(
        `<${element.localName}> ${label} (${String(element.scrollWidth)} px in ${String(element.clientWidth)})`,
      )
    }
  }

  return scrollers
}

// pageScrolls says whether the page itself scrolls down, which the shell never
// does: the content, or a pane in it, scrolls instead.
function pageScrolls(): boolean {
  return (document.scrollingElement?.scrollHeight ?? 0) > window.innerHeight
}

// tabStops are where Tab stops: every link, button and field not disabled or
// taken out of the order.
function tabStops(): HTMLElement[] {
  const candidates = document.querySelectorAll<HTMLElement>(
    'a[href], button, input, select, textarea, [tabindex]',
  )

  return [...candidates].filter((stop) => stop.tabIndex >= 0 && !stop.matches(':disabled'))
}

// drawnOnly are the stops a sighted user can see: those with a size, and not
// hidden.
function drawnOnly(stops: HTMLElement[]): HTMLElement[] {
  return stops.filter((stop) => {
    const box = stop.getBoundingClientRect()

    return box.width > 1 && box.height > 1 && getComputedStyle(stop).visibility !== 'hidden'
  })
}

interface Stop {
  // index is where the focused control sits among the drawn ones, or -1.
  index: number
  // words are the focused control's text, to name one that is not drawn.
  words: string
  // shown is how much of the focused control is in view.
  shown: number
}

// focusedStop says which control has focus, and how much of it is in view:
// what focus must reveal of it, clipped by the window and by every part of the
// page around it that scrolls or clips.
function focusedStop(controls: HTMLElement[]): Stop {
  // revealed is what focus must bring into view: the whole control, or a text
  // area's first line, since the browser brings the caret into view, not the
  // whole box.
  function revealed(control: HTMLElement): { top: number; bottom: number } {
    const box = control.getBoundingClientRect()
    if (!(control instanceof HTMLTextAreaElement)) {
      return box
    }

    const style = getComputedStyle(control)
    const line = ['borderTopWidth', 'paddingTop', 'lineHeight'] as const

    return {
      top: box.top,
      bottom: box.top + line.reduce((sum, key) => sum + parseFloat(style[key]), 0),
    }
  }

  // visibleArea is the part of the window a control can be seen through.
  function visibleArea(control: HTMLElement) {
    const view = { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight }
    for (let clip = control.parentElement; clip !== null; clip = clip.parentElement) {
      const style = getComputedStyle(clip)
      if (style.overflowX !== 'visible' || style.overflowY !== 'visible') {
        const area = clip.getBoundingClientRect()
        view.left = Math.max(view.left, area.left + clip.clientLeft)
        view.top = Math.max(view.top, area.top + clip.clientTop)
        view.right = Math.min(view.right, area.left + clip.clientLeft + clip.clientWidth)
        view.bottom = Math.min(view.bottom, area.top + clip.clientTop + clip.clientHeight)
      }
    }

    return view
  }

  const focused = document.activeElement
  if (!(focused instanceof HTMLElement) || focused === document.body) {
    return { index: -1, words: 'the page', shown: 1 }
  }

  const { left, right, width } = focused.getBoundingClientRect()
  const { top, bottom } = revealed(focused)
  const view = visibleArea(focused)
  const across = Math.max(0, Math.min(right, view.right) - Math.max(left, view.left))
  const down = Math.max(0, Math.min(bottom, view.bottom) - Math.max(top, view.top))

  return {
    index: controls.indexOf(focused),
    words: focused.textContent.trim(),
    shown: (across * down) / (width * (bottom - top)),
  }
}

// controlNames are how a failure names each control: its label, or its words.
function controlNames(controls: HTMLElement[]): string[] {
  return controls.map((control) => {
    const { labels } = control as Partial<Pick<HTMLInputElement, 'labels'>>
    const label = labels ? [...labels].at(0)?.textContent : null

    return control.getAttribute('aria-label') ?? label ?? control.textContent.trim()
  })
}

// walkTabOrder presses Tab once round the page — each stop, and the page
// itself as the order wraps — and reports the drawn controls it never reached
// and the ones that had focus while out of view.
async function walkTabOrder(page: Page): Promise<{ missed: string[]; hidden: string[] }> {
  const stops = await page.evaluateHandle(tabStops)
  const lap = (await stops.evaluate((all) => all.length)) + 1
  const controls = await stops.evaluateHandle(drawnOnly)
  const names = await controls.evaluate(controlNames)
  const reached = new Set<number>()
  const hidden: string[] = []
  for (let step = 0; step < lap; step++) {
    await page.keyboard.press('Tab')
    const stop = await controls.evaluate(focusedStop)
    reached.add(stop.index)
    if (stop.shown < inView) {
      const name = names[stop.index] ?? stop.words
      hidden.push(`${name} (${String(Math.round(stop.shown * 100))}% in view)`)
    }
  }

  return { missed: names.filter((_, index) => !reached.has(index)), hidden }
}

async function axeViolations(page: Page): Promise<string> {
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  return violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
}

for (const theme of themes) {
  for (const width of widths) {
    test(
      `every section fits ${String(width)} px in the ${theme} theme, reachable and clean`,
      { tag: '@populated' },
      async ({ page }) => {
        // Arrange: the populated cockpit at this width, in this theme.
        await openCockpit(page, { width, height }, theme)

        for (const name of sectionNames) {
          // Act: open the section, and Tab once round it.
          await openSection(page, name)
          const { missed, hidden } = await walkTabOrder(page)

          // Assert: nothing scrolls sideways, nor the page down; Tab reaches
          // every drawn control, each in view as it has focus; and axe finds
          // nothing.
          expect(await page.evaluate(sidewaysScrollers), `${name}: scrolls sideways`).toEqual([])
          expect(await page.evaluate(pageScrolls), `${name}: the page scrolls`).toBe(false)
          expect(missed, `${name}: never reached by Tab`).toEqual([])
          expect(hidden, `${name}: out of view with focus`).toEqual([])
          expect(await axeViolations(page), `${name}: axe`).toBe('')
        }
      },
    )
  }
}

// From md (768 px) the rail names each section beside its icon; below it, the
// rail keeps only the icons, and each name stays the button's accessible name,
// and its title, which a pointer resting on the icon shows.
const railCases = [
  { width: 640, named: false },
  { width: 1024, named: true },
  { width: 1440, named: true },
]

for (const { width, named } of railCases) {
  test(
    `the rail ${named ? 'names its sections' : 'keeps only its icons'} at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await page.setViewportSize({ width, height })
      await page.goto('/')
      const rail = page.getByRole('navigation', { name: 'Sections' })
      const buttons = sectionNames.map((name) => rail.getByRole('button', { name, exact: true }))

      // Act: measure each name as it is drawn.
      const drawn = await Promise.all(
        buttons.map(async (button, index) => {
          const name = await button.getByText(sectionNames[index], { exact: true }).boundingBox()

          return (name?.width ?? 0) > 1
        }),
      )

      // Assert: each button keeps its name, as its title too, drawn only from
      // md up.
      for (const [index, button] of buttons.entries()) {
        await expect(button).toBeVisible()
        await expect(button).toHaveAttribute('title', sectionNames[index])
      }
      expect(drawn).toEqual(sectionNames.map(() => named))
    },
  )
}

// The header holds still while a section scrolls beneath it: the page itself
// never scrolls, the content does.
for (const width of widths) {
  test(
    `the header stays in view as a long section scrolls at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange: the settings, longer than the window.
      await page.setViewportSize({ width, height })
      await page.goto('/')
      await openSection(page, 'Settings')
      const save = page.getByRole('button', { name: 'Save changes' })

      // Act: take the form's last control into view.
      await save.focus()

      // Assert: it is in view, and so is the header above it.
      await expect(save).toBeInViewport()
      await expect(page.getByRole('button', { name: /change theme/i })).toBeInViewport()
    },
  )
}

// In a window shorter than the rail, the rail scrolls itself, not the page:
// the header stays in view as the rail's last section takes focus.
test(
  'the rail scrolls itself in a window shorter than it',
  { tag: '@populated' },
  async ({ page }) => {
    // Arrange
    await page.setViewportSize({ width: 640, height: 240 })
    await page.goto('/')
    const rail = page.getByRole('navigation', { name: 'Sections' })
    const settings = rail.getByRole('button', { name: 'Settings', exact: true })

    // Act
    await settings.focus()

    // Assert
    await expect(settings).toBeInViewport()
    await expect(page.getByRole('button', { name: /change theme/i })).toBeInViewport()
    expect(await page.evaluate(pageScrolls), 'the page scrolls').toBe(false)
  },
)

// The skip link is the first stop, drawn as it has focus, and it hands focus to
// the content.
for (const width of widths) {
  test(
    `the skip link takes focus to the content at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange
      await page.setViewportSize({ width, height })
      await page.goto('/')
      const skip = page.getByRole('link', { name: 'Skip to content' })

      // Act: the first Tab
      await page.keyboard.press('Tab')

      // Assert: it lands on the skip link, drawn in view
      await expect(skip).toBeFocused()
      await expect(skip).toBeInViewport({ ratio: 1 })

      // Act: follow it
      await page.keyboard.press('Enter')

      // Assert: the content has focus
      await expect(page.getByRole('main')).toBeFocused()
    },
  )
}

// A section opens at its top, wherever the last one was scrolled to, so the
// content that takes focus as it opens shows its heading.
for (const width of widths) {
  test(
    `a section opens at its top at ${String(width)} px`,
    { tag: '@populated' },
    async ({ page }) => {
      // Arrange: the settings, scrolled to their end.
      await page.setViewportSize({ width, height })
      await page.goto('/')
      await openSection(page, 'Settings')
      await page.getByRole('button', { name: 'Save changes' }).focus()

      // Act
      await openSection(page, 'Branch')

      // Assert
      await expect(page.getByRole('heading', { level: 1, name: 'Branch' })).toBeInViewport({
        ratio: 1,
      })
    },
  )
}

const issuesTotal = 12

// streamedIssue is one of the issues the stream carries, or a later page adds.
function streamedIssue(number: number) {
  return {
    key: `PROJ-${String(number)}`,
    summary: `Issue number ${String(number)} of the view`,
    status: 'To Do',
    status_category: 'new',
    type: 'Task',
  }
}

// A stream carrying the first eight of the view's twelve issues, so the list
// offers to load more.
const pagedSnapshot = {
  issues: { total: issuesTotal, start_at: 0, issues: [1, 2, 3, 4, 5, 6, 7, 8].map(streamedIssue) },
  branch: {
    name: '',
    detached: false,
    head: '',
    upstream: '',
    push_remote: '',
    ahead: 0,
    behind: 0,
    base: '',
    commits: [],
  },
  changes: { changes: [] },
  review: { found: false },
  messaging: { service: 'Slack', configured: false, channel: '', channels: [], author: '' },
  branches: [],
  suggested_scope: '',
}

// streams answers the event stream with one snapshot.
async function streams(page: Page, snapshot: object): Promise<void> {
  await page.route('**/api/events**', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: `event: snapshot\ndata: ${JSON.stringify(snapshot)}\n\n`,
    }),
  )
}

test('a loaded page hands focus to its first issue, in view in the list, at 640 px', async ({
  page,
}) => {
  // Arrange: the stream's eight issues, and the next page answered here, in a
  // window where the list scrolls in its pane.
  await streams(page, pagedSnapshot)
  await page.route(/\/api\/issues\?/, (route) =>
    route.fulfill({
      json: { total: issuesTotal, start_at: 8, issues: [9, 10, 11, 12].map(streamedIssue) },
    }),
  )
  await page.setViewportSize({ width: 640, height: 700 })
  await page.goto('/')

  // Act
  await page.getByRole('button', { name: 'Load more' }).click()

  // Assert: the first issue the page added has focus, and is in view.
  const added = page.getByRole('button', { name: /^PROJ-9/ })
  await expect(added).toBeFocused()
  await expect(added).toBeInViewport({ ratio: 1 })
})

// A branch and a file whose names each hold a word wider than the content.
const unbrokenSnapshot = {
  ...pagedSnapshot,
  branch: {
    ...pagedSnapshot.branch,
    name: 'fix/PROJ-1',
    head: 'abc1234',
    upstream: 'origin/redact_every_authorization_header_before_the_request_log_writes_it',
    base: 'origin/main',
  },
  changes: {
    changes: [
      {
        path: 'internal/tui/testdata/TestScreenDrawsEveryPaneAtTheNarrowestWidthItAllows.golden',
        kind: 'modified',
        staged: false,
        has_unstaged: true,
        conflicted: false,
      },
    ],
  },
}

// A source build's health: its version is a commit marked dirty, the widest
// the header draws.
const sourceBuild = {
  version: 'ddbb935d6c04-dirty',
  dry_run: false,
  forge_noun: 'pull request',
  forge_sigil: '#',
}

// widthDrawn is how wide an element is drawn.
function widthDrawn(element: Element): number {
  return element.getBoundingClientRect().width
}

// termsSlack is how much wider the terms' column is than the widest term in it.
function termsSlack(terms: Element[]): number {
  const column = Math.max(...terms.map((term) => term.getBoundingClientRect().width))
  const words = terms.map((term) => {
    const range = document.createRange()
    range.selectNodeContents(term)

    return range.getBoundingClientRect().width
  })

  return column - Math.max(...words)
}

test('the branch and the header fit 320 px, wrapping a name wider than the content', async ({
  page,
}) => {
  // Arrange: the stream's branch and working tree, and a source build's
  // version in the header, at the narrowest width a page must reflow to.
  await streams(page, unbrokenSnapshot)
  await page.route('**/api/health', (route) => route.fulfill({ json: sourceBuild }))
  await page.setViewportSize({ width: 320, height })
  await page.goto('/')

  // Act
  await openSection(page, 'Branch')

  // Assert: nothing scrolls sideways; the file's path takes a line of its
  // own; and the terms' column is as wide as its widest term.
  const path = page.getByText(/^internal\/tui\/testdata/)
  const row = page.getByRole('listitem').filter({ has: path })
  await expect(path).toBeVisible()
  expect(await page.evaluate(sidewaysScrollers)).toEqual([])
  expect(await path.evaluate(widthDrawn), 'the path takes the row').toBeGreaterThanOrEqual(
    (await row.evaluate(widthDrawn)) - 1,
  )
  expect(await page.getByRole('term').evaluateAll(termsSlack), 'the terms column').toBeLessThan(1)
})
