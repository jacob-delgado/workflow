import type { JSHandle, Locator, Page } from '@playwright/test'

// A lap of the Tab order, round the page or a part of it, that says which
// drawn controls it reached, which it never reached, which had focus out of
// view, and where it stopped outside the part walked.

// A control counts as in view when this much of it is, allowing a rounding
// pixel at a scrolled edge.
const inView = 0.99

// tabStops are where Tab stops in a part of the page: every link, button,
// field and disclosure's summary not disabled or taken out of the order.
function tabStops(part: ParentNode): HTMLElement[] {
  const candidates = part.querySelectorAll<HTMLElement>(
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
  // inside is whether the focused control is in the part of the page walked.
  inside: boolean
}

// focusedStop says which control has focus, whether it is in the part of the
// page walked, and how much of it is in view: what focus must reveal of it,
// clipped by the window and by every part of the page around it that scrolls
// or clips.
function focusedStop(controls: HTMLElement[], part: ParentNode): Stop {
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
    return { index: -1, words: 'the page', shown: 1, inside: true }
  }

  const { left, right, width } = focused.getBoundingClientRect()
  const { top, bottom } = revealed(focused)
  const view = visibleArea(focused)
  const across = Math.max(0, Math.min(right, view.right) - Math.max(left, view.left))
  const down = Math.max(0, Math.min(bottom, view.bottom) - Math.max(top, view.top))
  // A control drawn at no size shows none of itself, where its share of
  // itself in view would read 0 over 0.
  const share = (across * down) / (width * (bottom - top))

  return {
    index: controls.indexOf(focused),
    words: focused.textContent.trim(),
    shown: Number.isFinite(share) ? share : 0,
    inside: part.contains(focused),
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
// reached, those it never reached, those that had focus while out of view,
// and the stops it made outside the part of the page walked.
interface TabWalk {
  reached: string[]
  missed: string[]
  hidden: string[]
  left: string[]
}

// Walk says what a lap walks: the whole page, or the part of it within a
// locator — an open dialog, say, which Tab must not leave.
interface Walk {
  within?: Locator
}

// walkTabOrder presses Tab once round the page, or the part of it walked —
// each stop, and the page itself as the order wraps — and reports what it
// reached, what it never reached, what had focus while out of view, and where
// it stopped outside the part walked. The lap ends as focus comes back to its
// first stop, so it follows the browser's own order: a part that scrolls with
// nothing in it to take focus, a dialog that scrolls among them, is a stop
// there that no selector names.
export async function walkTabOrder(page: Page, { within }: Walk = {}): Promise<TabWalk> {
  const part = await walked(page, within)
  const stops = await part.evaluateHandle(tabStops)
  const most = 2 * (await stops.evaluate((all) => all.length)) + 2
  const controls = await stops.evaluateHandle(drawnOnly)
  const names = await controls.evaluate(controlNames)
  const walk: TabWalk = { reached: [], missed: [], hidden: [], left: [] }
  const reached = new Set<number>()
  await page.keyboard.press('Tab')
  const first = await page.evaluateHandle(() => document.activeElement)
  for (let step = 0; step < most; step++) {
    const stop = await controls.evaluate(focusedStop, part)
    if (stop.inside) {
      reached.add(stop.index)
    } else {
      walk.left.push(stop.words)
    }
    if (stop.inside && stop.shown < inView) {
      const name = names[stop.index] ?? stop.words
      walk.hidden.push(`${name} (${String(Math.round(stop.shown * 100))}% in view)`)
    }

    await page.keyboard.press('Tab')
    if (await first.evaluate((began) => began === document.activeElement)) {
      walk.reached = names.filter((_, index) => reached.has(index))
      walk.missed = names.filter((_, index) => !reached.has(index))

      return walk
    }
  }

  throw new Error(`Tab did not come back round to its first stop in ${String(most)} presses`)
}

// walked is the part of the page a lap walks: the one within the locator, or
// the whole document.
function walked(page: Page, within?: Locator): Promise<JSHandle<ParentNode>> {
  if (within === undefined) {
    return page.evaluateHandle((): ParentNode => document)
  }

  return within.evaluateHandle((part): ParentNode => part)
}
