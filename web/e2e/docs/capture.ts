import type { Page } from '@playwright/test'

// What every image the docs show is held to before it is taken, a
// screenshot's and each of the walkthrough's keyframes alike: the page's own
// face loaded, no text or control cut by the edge of the window or of a pane,
// and nothing caught mid-animation.

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
// whole box, as far as a pane that ends it with an ellipsis lets it run, and
// seen, what of it is in view once the window and every pane around it that
// clips what overflows it have cut it, or null when none of it is.
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
// and one that ends a line it clips with an ellipsis ends the line there, so
// the panes around it judge the line as far as the ellipsis, not the whole
// text it truncates.
export function partsShown(page: Page, boxes: string): Promise<Part[]> {
  return page.evaluate((selector) => {
    const boxOf = (rect: DOMRectReadOnly): Box => ({
      left: rect.left,
      top: rect.top,
      right: rect.right,
      bottom: rect.bottom,
    })
    const viewOf = (from: Element | null, drawn: Box): Omit<Part, 'what'> => {
      const whole = { ...drawn }
      const seen = {
        left: Math.max(drawn.left, 0),
        top: Math.max(drawn.top, 0),
        right: Math.min(drawn.right, window.innerWidth),
        bottom: Math.min(drawn.bottom, window.innerHeight),
      }
      for (let around = from; around !== null; around = around.parentElement) {
        const style = getComputedStyle(around)
        const pane = around.getBoundingClientRect()
        if (style.clipPath !== 'none') {
          return { whole, seen: null }
        }
        if (style.overflowX !== 'visible') {
          if (style.textOverflow === 'ellipsis') {
            whole.left = Math.max(whole.left, pane.left)
            whole.right = Math.min(whole.right, pane.right)
          }
          seen.left = Math.max(seen.left, pane.left)
          seen.right = Math.min(seen.right, pane.right)
        }
        if (style.overflowY !== 'visible') {
          seen.top = Math.max(seen.top, pane.top)
          seen.bottom = Math.min(seen.bottom, pane.bottom)
        }
      }

      return { whole, seen: seen.right > seen.left && seen.bottom > seen.top ? seen : null }
    }
    const shown = (element: Element) => element.checkVisibility({ visibilityProperty: true })
    const lines = (node: Node, holder: Element): Part[] => {
      const range = document.createRange()
      range.selectNodeContents(node)

      return [...range.getClientRects()].map((line) => ({
        what: node.textContent?.trim() ?? '',
        ...viewOf(holder, boxOf(line)),
      }))
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
        const named = element.getAttribute('aria-label') ?? element.textContent.trim()
        const what = `${element.tagName.toLowerCase()} ${named.slice(0, 60)}`.trim()
        parts.push({
          what,
          ...viewOf(element.parentElement, boxOf(element.getBoundingClientRect())),
        })
      }
    }

    return parts
  }, boxes)
}

// Cut is a line of text or a control an edge cuts through, the window's or
// a pane's that clips what overflows it: what it is, its whole box, and what
// of it shows.
export interface Cut {
  what: string
  whole: Box
  seen: Box
}

// cutBoxes are the elements an edge may not cut, beside the lines of text.
export const cutBoxes = 'input, textarea, select, button, svg, img'

// cutTolerance is how far, in pixels, an edge may clip a part before it
// counts as cut: a line's box can overhang its pane by a hair.
export const cutTolerance = 1

// partlySeen says whether an edge clips a part that still partly shows.
function partlySeen(whole: Box, seen: Box): boolean {
  return (
    seen.left > whole.left + cutTolerance ||
    seen.top > whole.top + cutTolerance ||
    seen.right < whole.right - cutTolerance ||
    seen.bottom < whole.bottom - cutTolerance
  )
}

// cutAtTheEdge is every line of text and every control the page shows only
// in part, cut by any edge of the window or of a pane around it that clips
// what overflows it: a row a pane's bottom cuts as much as one the window's
// does, and a line scrolled up under the top of the main pane. What a pane
// hides wholly is not cut, only out of view.
export async function cutAtTheEdge(page: Page): Promise<Cut[]> {
  const parts = await partsShown(page, cutBoxes)

  return parts.flatMap(({ what, whole, seen }) =>
    seen !== null && partlySeen(whole, seen) ? [{ what, whole, seen }] : [],
  )
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
