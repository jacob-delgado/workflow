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
// there that no selector names, and in Firefox so is any part that scrolls.
export async function walkTabOrder(page: Page, { within }: Walk = {}): Promise<TabWalk> {
  const part = await walked(page, within)
  const stops = await part.evaluateHandle(tabStops)
  const most = 2 * (await stops.evaluate((all) => all.length)) + 2
  const controls = await stops.evaluateHandle(drawnOnly)
  const names = await controls.evaluate(controlNames)
  const reached = new Set<number>()
  const hidden: string[] = []
  const left: string[] = []
  await pressTab(page)
  const first = await page.evaluateHandle(() => document.activeElement)
  for (let step = 0; step < most; step++) {
    const stop = await controls.evaluate(focusedStop, part)
    if (stop.inside) {
      reached.add(stop.index)
    } else {
      left.push(stop.words)
    }
    if (stop.inside && stop.shown < inView) {
      const name = names[stop.index] ?? stop.words
      hidden.push(`${name} (${String(Math.round(stop.shown * 100))}% in view)`)
    }

    await pressTab(page)
    if (await first.evaluate((began) => began === document.activeElement)) {
      return {
        reached: names.filter((_, index) => reached.has(index)),
        missed: names.filter((_, index) => !reached.has(index)),
        hidden,
        left,
      }
    }
  }

  throw new Error(`Tab did not come back round to its first stop in ${String(most)} presses`)
}

// pressTab presses Tab and waits for focus to come to rest, then, if the press
// took focus out of the document without the page seeing it go, brings focus
// back in at the top of its order.
//
// A press on the document's last stop takes focus out of it, to the browser's
// own controls. Chromium then has focus on the page itself, and the next press
// brings it back in at the top; WebKit's order comes round too. Playwright's
// Firefox keeps the last stop focused instead, since its patch skips the blur
// as focus leaves a page so that every page reads as focused, and each press
// after leaves from that stop again: the order would never come round.
async function pressTab(page: Page): Promise<void> {
  const leftUnseen = await page.evaluateHandle(watchForLeaving)
  await pressTabToRest(page)
  if (await leftUnseen.evaluate((left) => left())) {
    const putBack = await page.evaluateHandle(focusTop)
    await pressTabToRest(page)
    await putBack.evaluate((restore) => {
      restore()
    })
  }
}

// pressTabToRest presses Tab and waits for focus to come to rest. A Base UI
// dialog bounds its Tab order with focus guards hidden from assistive
// technology, each of which hands focus back inside as it takes it, and not
// always before the press resolves: read too soon, focus is on a guard outside
// the dialog. Focus never rests on what assistive technology is told is not
// there, so a guard that kept it would fail the wait, and a trap that leaks
// still fails the walk.
async function pressTabToRest(page: Page): Promise<void> {
  await page.keyboard.press('Tab')
  await page.waitForFunction(() => document.activeElement?.closest('[aria-hidden="true"]') == null)
}

// watchForLeaving watches the press about to be made, and gives back a check
// of whether it took focus out of the document unseen: the page left the key
// to the browser, yet focus came to nothing in the page and stayed where it
// was. A control that keeps Tab to itself prevents the key, so its lap still
// never closes. In Playwright's Firefox a trap that puts focus back on the
// document's last stop after the key does so with no focus event, since focus
// never left the stop, so the press reads as leaving and only Chromium's run
// catches that trap.
function watchForLeaving(): () => boolean {
  const from = document.activeElement
  let arrived = false
  let press: KeyboardEvent | undefined
  const onFocus = () => {
    arrived = true
  }
  const onKey = (event: KeyboardEvent) => {
    press ??= event
  }
  window.addEventListener('focusin', onFocus, true)
  window.addEventListener('keydown', onKey, true)

  return () => {
    window.removeEventListener('focusin', onFocus, true)
    window.removeEventListener('keydown', onKey, true)

    return press?.defaultPrevented === false && !arrived && document.activeElement === from
  }
}

// focusTop puts focus where Tab comes back into the document from the
// browser's own controls: the page itself, or an open modal dialog, outside
// which nothing takes focus. It gives back what puts the element as it was.
function focusTop(): () => void {
  const top = document.querySelector<HTMLElement>(':modal') ?? document.body
  const tabIndex = top.getAttribute('tabindex')
  top.tabIndex = -1
  top.focus({ preventScroll: true })

  return () => {
    if (tabIndex === null) {
      top.removeAttribute('tabindex')
    } else {
      top.setAttribute('tabindex', tabIndex)
    }
  }
}

// walked is the part of the page a lap walks: the one within the locator, or
// the whole document.
function walked(page: Page, within?: Locator): Promise<JSHandle<ParentNode>> {
  if (within === undefined) {
    return page.evaluateHandle((): ParentNode => document)
  }

  return within.evaluateHandle((part): ParentNode => part)
}
