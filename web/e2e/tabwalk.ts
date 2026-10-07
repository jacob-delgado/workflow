import { AxeBuilder } from '@axe-core/playwright'
import type { Page } from '@playwright/test'
import type { Snapshot } from '../src/api/generated/types.gen.ts'

// What the layout and accessibility specs share: a lap of the Tab order that
// says which drawn controls it reached, which it never reached and which had
// focus out of view, and a hermetic stream that answers with one snapshot, so
// a spec can open a step before it walks or scans it; and what a layout is
// held to beside the walk — nothing scrolls sideways, the page does not
// scroll, and axe finds nothing.

// A control counts as in view when this much of it is, allowing a rounding
// pixel at a scrolled edge.
const inView = 0.99

// tabStops are where Tab stops: every link, button, field and disclosure's
// summary not disabled or taken out of the order.
function tabStops(): HTMLElement[] {
  const candidates = document.querySelectorAll<HTMLElement>(
    'a[href], button, input, select, textarea, details > summary, [tabindex]',
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

// controlNames are how a walk names each control: its label's own words, or
// its words.
function controlNames(controls: HTMLElement[]): string[] {
  // labelWords are what a control's label says around it, leaving out the
  // control's own text, such as the options of a select the label holds.
  function labelWords(control: HTMLElement): string | undefined {
    const { labels } = control as Partial<Pick<HTMLInputElement, 'labels'>>
    const label = labels ? [...labels].at(0) : undefined
    if (label === undefined) {
      return undefined
    }

    const around = [...label.childNodes].filter((part) => !part.contains(control))

    return around
      .map((part) => part.textContent)
      .join('')
      .trim()
  }

  return controls.map(
    (control) =>
      control.getAttribute('aria-label') ?? labelWords(control) ?? control.textContent.trim(),
  )
}

// TabWalk is what one lap of the Tab order found: the drawn controls it
// reached, those it never reached, and those that had focus while out of view.
interface TabWalk {
  reached: string[]
  missed: string[]
  hidden: string[]
}

// walkTabOrder presses Tab once round the page — each stop, and the page
// itself as the order wraps — and reports what it reached, what it never
// reached, and what had focus while out of view.
export async function walkTabOrder(page: Page): Promise<TabWalk> {
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

  return {
    reached: names.filter((_, index) => reached.has(index)),
    missed: names.filter((_, index) => !reached.has(index)),
    hidden,
  }
}

// streams answers the event stream with one snapshot.
export async function streams(page: Page, snapshot: Snapshot): Promise<void> {
  await page.route('**/api/events**', (route) =>
    route.fulfill({
      contentType: 'text/event-stream',
      body: `event: snapshot\ndata: ${JSON.stringify(snapshot)}\n\n`,
    }),
  )
}

// sidewaysScrollers names what scrolls sideways: the page, or any part of it.
export function sidewaysScrollers(): string[] {
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
export function pageScrolls(): boolean {
  return (document.scrollingElement?.scrollHeight ?? 0) > window.innerHeight
}

// axeViolations names the WCAG A/AA violations axe finds on screen, or is
// empty.
export async function axeViolations(page: Page): Promise<string> {
  const { violations } = await new AxeBuilder({ page })
    .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa'])
    .analyze()

  return violations.map((v) => `${v.id} (${String(v.nodes.length)})`).join(', ')
}
