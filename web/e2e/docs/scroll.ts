import type { Locator, Page } from '@playwright/test'
import type { Point } from './cover.ts'

// How the walkthrough's film scrolls the page: smoothly, as a person does,
// and only as far as it must; where a part of the page stands against the
// window; and when a scroll has settled. film.ts keeps each scroll as a
// stretch the film moves through.

// scrollTops is where every pane on the page stands, as one line to compare.
export function scrollTops(page: Page): Promise<string> {
  return page.evaluate(() =>
    [...document.querySelectorAll('*')].map((pane) => pane.scrollTop).join(' '),
  )
}

// focusScrolls watches every element the page's script gives the focus
// from now on and gives a check that stops watching and names each pane
// the browser scrolled to show one, and how far: where the panes stood as
// focus() was called against where they stand once it returns, so a pane
// that moves because the page grew or shrank is not counted.
export async function focusScrolls(page: Page): Promise<() => Promise<string[]>> {
  const watch = await page.evaluateHandle(() => {
    const scrolled: string[] = []
    // eslint-disable-next-line @typescript-eslint/unbound-method -- called on each element given the focus, as the page calls it
    const focus = HTMLElement.prototype.focus
    HTMLElement.prototype.focus = function (this: HTMLElement, options?: FocusOptions) {
      const panes = [...document.querySelectorAll('*')].map((pane) => ({
        pane,
        top: pane.scrollTop,
      }))
      focus.call(this, options)
      for (const { pane, top } of panes.filter(({ pane, top }) => pane.scrollTop !== top)) {
        scrolled.push(`${pane.tagName} from ${String(top)} to ${String(pane.scrollTop)}`)
      }
    }

    return () => {
      HTMLElement.prototype.focus = focus

      return scrolled
    }
  })

  return async () => {
    const scrolled = await watch.evaluate((stop) => stop())
    await watch.dispose()

    return scrolled
  }
}

// settled waits, a frame at a time, until the panes have moved from where
// they stood before a scroll began and then stood still for two frames, and
// says whether they moved; a scroll that never starts, since the page was
// already as far as it goes, gives up after a dozen.
export function settled(page: Page, before: string): Promise<boolean> {
  return page.evaluate(
    (from) =>
      new Promise<boolean>((resolve) => {
        const panes = () =>
          [...document.querySelectorAll('*')].map((pane) => pane.scrollTop).join(' ')
        let last = from
        let moved = false
        let still = 0
        const watch = () => {
          const now = panes()
          moved ||= now !== last
          still = now === last ? still + 1 : 0
          last = now
          if ((moved && still >= 2) || (!moved && still >= 12)) {
            resolve(moved)

            return
          }
          requestAnimationFrame(watch)
        }
        requestAnimationFrame(watch)
      }),
    before,
  )
}

// scrollTo starts a smooth scroll, as a person's, that brings target into
// view: its end at its pane's end, or past whichever edge is nearest.
export async function scrollTo(target: Locator, block: 'end' | 'nearest'): Promise<void> {
  await target.evaluate((element, to) => {
    element.scrollIntoView({ block: to, behavior: 'smooth' })
  }, block)
}

// wholeInView says whether shown lies wholly inside the window and every
// pane around it that clips what overflows it.
export function wholeInView(shown: Locator): Promise<boolean> {
  return shown.evaluate((element) => {
    const box = element.getBoundingClientRect()
    let within = box.top >= 0 && box.bottom <= window.innerHeight
    for (let around = element.parentElement; around !== null; around = around.parentElement) {
      if (getComputedStyle(around).overflowY !== 'visible') {
        const pane = around.getBoundingClientRect()
        within &&= box.top >= pane.top && box.bottom <= pane.bottom
      }
    }

    return within
  })
}

// lineRoom is how far below a control the line that says what its press did
// reaches: the gap above the line and the line on its ground, one row of
// text tall. The film's click to a line fails when the line takes more.
const lineRoom = 48

// scrollRoomBelow starts a smooth scroll of the pane target lies in, the
// nearest around it that scrolls, the least that shows lineRoom below target
// clear of the pane's scroll padding; none when that room already shows.
export function scrollRoomBelow(target: Locator): Promise<void> {
  return target.evaluate((element, room) => {
    let pane = element.parentElement
    while (pane !== null && !/^(auto|scroll)$/.test(getComputedStyle(pane).overflowY)) {
      pane = pane.parentElement
    }
    if (pane === null) {
      return
    }
    const foot = parseFloat(getComputedStyle(pane).scrollPaddingBottom) || 0
    const short =
      element.getBoundingClientRect().bottom + room - (pane.getBoundingClientRect().bottom - foot)
    if (short > 0) {
      pane.scrollBy({ top: short, behavior: 'smooth' })
    }
  }, lineRoom)
}

// Clearing is what scrollClear weighs a pane's scroll by: the elements an
// edge may not cut beside the lines of text, how far past an edge a part may
// overhang before it counts as cut, and the points the cuts show at.
export interface Clearing {
  boxes: string
  tolerance: number
  points: Point[]
}

// scrollClear starts a smooth scroll of the pane at the first point it can
// clear, the nearest pane there that scrolls: the shortest scroll after
// which its edges, and those of every pane around it that clips, cut no
// line of text or control in it, and what the beat shows, when it is whole,
// stays whole. It weighs each scroll on the page as it stands, every part
// in the pane moving with it, so a scroll that clears one cut by cutting
// the next line past the other edge is never taken. It says whether it
// started one; none when no point's pane can scroll clear.
export function scrollClear(shown: Locator, clearing: Clearing): Promise<boolean> {
  return shown.evaluate((element, { boxes, tolerance, points }) => {
    const scrolls = (pane: Element) =>
      /^(auto|scroll)$/.test(getComputedStyle(pane).overflowY) &&
      pane.scrollHeight > pane.clientHeight
    const paneAt = ({ x, y }: { x: number; y: number }) => {
      let pane = document.elementFromPoint(x, y)
      while (pane !== null && !scrolls(pane)) {
        pane = pane.parentElement
      }

      return pane
    }
    // span is how far top to bottom runs once every clipping element from
    // from up to, not including, stop has cut it, or null when one hides it.
    const span = (top: number, bottom: number, from: Element | null, stop: Element | null) => {
      const seen = { top, bottom }
      for (let around = from; around !== null && around !== stop; around = around.parentElement) {
        const style = getComputedStyle(around)
        if (style.clipPath !== 'none') {
          return null
        }
        if (style.overflowY !== 'visible') {
          const clip = around.getBoundingClientRect()
          seen.top = Math.max(seen.top, clip.top)
          seen.bottom = Math.min(seen.bottom, clip.bottom)
        }
      }

      return seen.bottom > seen.top ? seen : null
    }
    const shows = (part: Element) => part.checkVisibility({ visibilityProperty: true })
    const partsIn = (pane: Element) => {
      const drawn: { rect: DOMRectReadOnly; from: Element | null }[] = []
      const walker = document.createTreeWalker(pane, NodeFilter.SHOW_TEXT)
      for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
        const holder = node.parentElement
        if (node.textContent?.trim() && holder !== null && shows(holder)) {
          const range = document.createRange()
          range.selectNodeContents(node)
          drawn.push(...[...range.getClientRects()].map((rect) => ({ rect, from: holder })))
        }
      }
      for (const part of [...pane.querySelectorAll(boxes)].filter(shows)) {
        drawn.push({ rect: part.getBoundingClientRect(), from: part.parentElement })
      }

      return drawn.flatMap(({ rect, from }) => span(rect.top, rect.bottom, from, pane) ?? [])
    }
    const nearestClear = (pane: Element) => {
      const edge = pane.getBoundingClientRect()
      const view = span(Math.max(edge.top, 0), Math.min(edge.bottom, innerHeight), pane, null)
      const parts = partsIn(pane)
      const box = element.getBoundingClientRect()
      const whole = (by: number) =>
        view !== null &&
        box.top - by >= view.top - tolerance &&
        box.bottom - by <= view.bottom + tolerance
      const keeps = (by: number) => !pane.contains(element) || !whole(0) || whole(by)
      const cuts = (by: number) =>
        view !== null &&
        parts.some(
          ({ top, bottom }) =>
            Math.min(bottom - by, view.bottom) > Math.max(top - by, view.top) &&
            (view.top - (top - by) > tolerance || bottom - by - view.bottom > tolerance),
        )
      const least = -pane.scrollTop
      const most = pane.scrollHeight - pane.clientHeight - pane.scrollTop
      const tries = Array.from({ length: 2 * Math.ceil(Math.max(-least, most)) + 1 }, (_, step) =>
        step % 2 === 0 ? step / 2 : -(step + 1) / 2,
      )

      return tries.find((by) => by >= least && by <= most && keeps(by) && !cuts(by))
    }
    for (const pane of points.map(paneAt)) {
      const by = pane === null ? undefined : nearestClear(pane)
      if (pane !== null && by !== undefined && by !== 0) {
        pane.scrollBy({ top: by, behavior: 'smooth' })

        return true
      }
    }

    return false
  }, clearing)
}
