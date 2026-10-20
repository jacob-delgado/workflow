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

// fontsLoaded waits for the page's fonts and says whether its own face is
// among them, so no image draws its words in a fallback font.
export function fontsLoaded(page: Page): Promise<boolean> {
  return page.evaluate(async () => {
    await document.fonts.ready

    return document.fonts.check('16px "Recursive Variable"')
  })
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
