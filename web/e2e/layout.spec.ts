import { AxeBuilder } from '@axe-core/playwright'
import { expect, test, type Page } from '@playwright/test'
import { height, openCockpit, openSection, sectionNames, themes, widths } from './cockpit.ts'

// The populated cockpit at a narrow, a middling and a wide window, in both
// themes: nothing scrolls sideways, every control the keyboard can reach is in
// view once it has focus, and axe finds nothing.

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
  name: string
  // shown is how much of the focused control is in view: its box clipped by
  // the window and by every part of the page that scrolls or clips.
  shown: number
}

// focusedStop says which control has focus, and how much of it is in view.
function focusedStop(controls: HTMLElement[]): Stop {
  const focused = document.activeElement
  if (!(focused instanceof HTMLElement) || focused === document.body) {
    return { index: -1, name: 'the page', shown: 1 }
  }

  const box = focused.getBoundingClientRect()
  const view = { left: 0, top: 0, right: window.innerWidth, bottom: window.innerHeight }
  for (let clip = focused.parentElement; clip !== null; clip = clip.parentElement) {
    const style = getComputedStyle(clip)
    if (style.overflowX !== 'visible' || style.overflowY !== 'visible') {
      const area = clip.getBoundingClientRect()
      view.left = Math.max(view.left, area.left + clip.clientLeft)
      view.top = Math.max(view.top, area.top + clip.clientTop)
      view.right = Math.min(view.right, area.left + clip.clientLeft + clip.clientWidth)
      view.bottom = Math.min(view.bottom, area.top + clip.clientTop + clip.clientHeight)
    }
  }

  const across = Math.max(0, Math.min(box.right, view.right) - Math.max(box.left, view.left))
  const down = Math.max(0, Math.min(box.bottom, view.bottom) - Math.max(box.top, view.top))
  const name = focused.getAttribute('aria-label') ?? focused.textContent.trim()

  return {
    index: controls.indexOf(focused),
    name: name || focused.id,
    shown: (across * down) / (box.width * box.height),
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
  const reached = new Set<number>()
  const hidden: string[] = []
  for (let step = 0; step < lap; step++) {
    await page.keyboard.press('Tab')
    const stop = await controls.evaluate(focusedStop)
    reached.add(stop.index)
    if (stop.shown < inView) {
      hidden.push(`${stop.name} (${String(Math.round(stop.shown * 100))}% in view)`)
    }
  }

  const names = await controls.evaluate(controlNames)

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

          // Assert: nothing scrolls sideways; Tab reaches every drawn control,
          // each in view as it has focus; and axe finds nothing.
          expect(await page.evaluate(sidewaysScrollers), `${name}: scrolls sideways`).toEqual([])
          expect(missed, `${name}: never reached by Tab`).toEqual([])
          expect(hidden, `${name}: out of view with focus`).toEqual([])
          expect(await axeViolations(page), `${name}: axe`).toBe('')
        }
      },
    )
  }
}
