import type { Page } from '@playwright/test'

// What every image the docs show is held to before it is taken, a
// screenshot's and each of the walkthrough's keyframes alike: the page's own
// face loaded, no run of text cut by the window's bottom edge, and nothing
// caught mid-animation.

// now is the moment every image's browser holds: the demo's day as it
// begins, four in the afternoon in UTC (epoch in internal/demo/world.go). The
// demo dates its reviews and tasks from that moment, so with the browser's
// clock held there, every age, due date and running timer reads the same on
// each run.
export const now = new Date('2026-09-16T16:00:00Z')

// pointerId is the id of the walkthrough's pointer (pointer.ts), whose own
// pulse is the one animation an image may catch running.
export const pointerId = 'walkthrough-pointer'

// nextFrame resolves once the browser has drawn its next frame.
export function nextFrame(page: Page): Promise<void> {
  return page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => {
          resolve()
        })
      }),
  )
}

// fontsLoaded waits for the page's fonts and says whether its own face is
// among them, so no image draws its words in a fallback font.
export function fontsLoaded(page: Page): Promise<boolean> {
  return page.evaluate(async () => {
    await document.fonts.ready

    return document.fonts.check('16px "Recursive Variable"')
  })
}

// Box is a rectangle in the window's CSS pixels.
export interface Box {
  left: number
  top: number
  right: number
  bottom: number
}

// Part is a line of text or an element the page draws: what it is, its
// whole box, and seen, what of it is in view once the window and every pane
// around it that clips what overflows it have cut it, or null when none of
// it is.
export interface Part {
  what: string
  whole: Box
  seen: Box | null
}

// partsShown is every line of text in the page's body, and every element
// there that matches boxes, as the parts the page draws. What is not drawn
// is left out: an element checkVisibility says is hidden, and text inside a
// clip path, as a screen reader's own text is, which counts as seen nowhere.
// A pane that clips along one axis cuts its content along that axis only,
// and one that ends a line it clips with an ellipsis does not cut it at all.
export function partsShown(page: Page, boxes: string): Promise<Part[]> {
  return page.evaluate((selector) => {
    const boxOf = (rect: DOMRectReadOnly): Box => ({
      left: rect.left,
      top: rect.top,
      right: rect.right,
      bottom: rect.bottom,
    })
    const seenOf = (from: Element | null, whole: Box): Box | null => {
      const seen = {
        left: Math.max(whole.left, 0),
        top: Math.max(whole.top, 0),
        right: Math.min(whole.right, window.innerWidth),
        bottom: Math.min(whole.bottom, window.innerHeight),
      }
      for (let around = from; around !== null; around = around.parentElement) {
        const style = getComputedStyle(around)
        const pane = around.getBoundingClientRect()
        if (style.clipPath !== 'none') {
          return null
        }
        if (style.overflowX !== 'visible' && style.textOverflow !== 'ellipsis') {
          seen.left = Math.max(seen.left, pane.left)
          seen.right = Math.min(seen.right, pane.right)
        }
        if (style.overflowY !== 'visible') {
          seen.top = Math.max(seen.top, pane.top)
          seen.bottom = Math.min(seen.bottom, pane.bottom)
        }
      }

      return seen.right > seen.left && seen.bottom > seen.top ? seen : null
    }
    const shown = (element: Element) => element.checkVisibility({ visibilityProperty: true })
    const lines = (node: Node, holder: Element): Part[] => {
      const range = document.createRange()
      range.selectNodeContents(node)

      return [...range.getClientRects()].map((line) => {
        const whole = boxOf(line)

        return { what: node.textContent?.trim() ?? '', whole, seen: seenOf(holder, whole) }
      })
    }
    const parts: Part[] = []
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
      const holder = node.parentElement
      if (node.textContent?.trim() && holder !== null && shown(holder)) {
        parts.push(...lines(node, holder))
      }
    }
    for (const element of document.body.querySelectorAll(selector)) {
      if (shown(element)) {
        const whole = boxOf(element.getBoundingClientRect())
        const named = element.getAttribute('aria-label') ?? element.textContent.trim()
        const what = `${element.tagName.toLowerCase()} ${named.slice(0, 60)}`.trim()
        parts.push({ what, whole, seen: seenOf(element.parentElement, whole) })
      }
    }

    return parts
  }, boxes)
}

// Cut is a run of text the window's bottom edge cuts through, and how far
// down its lowest cut line ends.
export interface Cut {
  text: string
  bottom: number
}

// cutAtTheEdge is every run of text the window's bottom edge cuts through:
// a line that begins above the edge and ends below it, shown, and not
// already clipped above the edge by a pane that scrolls.
export function cutAtTheEdge(page: Page): Promise<Cut[]> {
  return page.evaluate(() => {
    const edge = window.innerHeight
    // shownBottom is how far down a line of text below the element shows,
    // once every pane around it that clips its overflow has cut it.
    const shownBottom = (element: Element, bottom: number): number => {
      let shown = bottom
      for (let around: Element | null = element; around !== null; around = around.parentElement) {
        if (getComputedStyle(around).overflowY !== 'visible') {
          shown = Math.min(shown, around.getBoundingClientRect().bottom)
        }
      }

      return shown
    }
    const cut: { text: string; bottom: number }[] = []
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT)
    for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
      const text = node.textContent?.trim() ?? ''
      const holder = node.parentElement
      if (text === '' || holder?.checkVisibility({ visibilityProperty: true }) !== true) {
        continue
      }
      const range = document.createRange()
      range.selectNodeContents(node)
      const crossing = [...range.getClientRects()].filter(
        (line) => line.top < edge && line.bottom > edge && shownBottom(holder, line.bottom) >= edge,
      )
      if (crossing.length > 0) {
        cut.push({ text, bottom: Math.max(...crossing.map((line) => line.bottom)) })
      }
    }

    return cut
  })
}

// stillAnimating names every animation or transition still running on the
// page but the pointer's own, each by what it animates, so an image waits
// until a section has slid in and a row has stopped flashing.
export function stillAnimating(page: Page): Promise<string[]> {
  return page.evaluate(
    (pointer) =>
      document
        .getAnimations()
        .filter((animation) => animation.playState === 'running')
        .map((animation) => {
          const effect = animation.effect
          const target = effect instanceof KeyframeEffect ? effect.target : null

          return target
        })
        .filter((target) => target?.closest(`#${pointer}`) === null)
        .map((target) =>
          `${target?.tagName ?? 'nothing'} ${target?.getAttribute('class') ?? ''}`.trim(),
        ),
    pointerId,
  )
}
