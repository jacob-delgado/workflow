import type { Locator, Page } from '@playwright/test'
import { nextFrame, pointerId } from './capture.ts'

// The walkthrough's pointer: an arrow drawn over the page where the mouse is,
// since a headless browser draws none, that glides to each control before it
// is pressed and pulses as it is. It is hidden from assistive technology and
// takes no pointer events, so the page acts as it would without it, and it
// sits outside the body, so the edge check never reads it.

// Point is where the mouse is, in the window's CSS pixels.
interface Point {
  x: number
  y: number
}

// Glide is a glide's span on the film's clock: when it began and ended.
interface Glide {
  began: number
  ended: number
}

// Pointer moves the mouse to a control in eased steps, the arrow following,
// and says when each glide ran.
export interface Pointer {
  glide: (target: Locator) => Promise<Glide>
  park: (target: Locator) => Promise<Glide | undefined>
}

// pixelsPerStep and the step bounds set a glide's pace: a step a frame, each
// about this far, so a short glide still eases and a long one stays brisk.
const pixelsPerStep = 24
const fewestSteps = 12
const mostSteps = 36

// arrow is the pointer's look, the outline of a desktop's arrow, drawn white
// and edged in black.
const arrow = 'M1 1 L1 23 L7 17 L11 27 L15 25 L11 15 L20 15 Z'

// drawPointer puts the arrow on every document the page loads, at the
// mouse, and pulses it at each press. It is drawn off the window until the
// mouse first moves, which placePointer makes it do.
export async function drawPointer(page: Page): Promise<void> {
  await page.addInitScript(
    ({ id, look }) => {
      window.addEventListener('DOMContentLoaded', () => {
        const pointer = document.createElement('div')
        pointer.id = id
        pointer.setAttribute('aria-hidden', 'true')
        const svg = 'http://www.w3.org/2000/svg'
        const drawing = document.createElementNS(svg, 'svg')
        const outline = document.createElementNS(svg, 'path')
        for (const [name, value] of Object.entries({
          width: '22',
          height: '28',
          viewBox: '0 0 22 28',
        })) {
          drawing.setAttribute(name, value)
        }
        for (const [name, value] of Object.entries({
          d: look,
          fill: '#fff',
          stroke: '#000',
          'stroke-width': '1.5',
          'stroke-linejoin': 'round',
        })) {
          outline.setAttribute(name, value)
        }
        drawing.append(outline)
        pointer.append(drawing)
        Object.assign(pointer.style, {
          position: 'fixed',
          left: '0',
          top: '0',
          zIndex: '2147483647',
          pointerEvents: 'none',
          transform: 'translate(-100px, -100px)',
        })
        document.documentElement.append(pointer)
        window.addEventListener('mousemove', (event) => {
          pointer.style.transform = `translate(${String(event.clientX)}px, ${String(event.clientY)}px)`
        })
        window.addEventListener('mousedown', () => {
          pointer.animate([{ scale: '1' }, { scale: '0.8' }, { scale: '1' }], {
            duration: 240,
            easing: 'ease-out',
          })
        })
      })
    },
    { id: pointerId, look: arrow },
  )
}

// eased is how far along a glide is at progress, from 0 to 1: slow out of
// its start, quick through the middle and slow into its end.
function eased(progress: number): number {
  return progress < 0.5 ? 2 * progress * progress : 1 - (-2 * progress + 2) ** 2 / 2
}

// stepsBetween is how many frames a glide from one point to another takes.
function stepsBetween(from: Point, to: Point): number {
  const distance = Math.hypot(to.x - from.x, to.y - from.y)

  return Math.min(mostSteps, Math.max(fewestSteps, Math.round(distance / pixelsPerStep)))
}

// parkingGap is how far beside or below a control the pointer rests once it
// has pressed it.
const parkingGap = 40

// clearSpot is the first spot beside a control, then below it, where the
// pointer covers no text and no control, so what the press brings up stays
// legible while a beat holds it; none when both are taken.
async function clearSpot(target: Locator): Promise<Point | undefined> {
  const box = await target.boundingBox()
  if (box === null) {
    return undefined
  }

  const spots = [
    { x: box.x + box.width + parkingGap, y: box.y + box.height / 2 },
    { x: box.x + box.width / 2, y: box.y + box.height + parkingGap },
  ]

  return target.page().evaluate((candidates) => {
    const clear = (spot: { x: number; y: number }) => {
      const under = document.elementFromPoint(spot.x, spot.y)
      if (under === null || spot.x >= window.innerWidth || spot.y >= window.innerHeight) {
        return false
      }
      const control = under.closest('button, a, input, textarea, select, label, [role="status"]')
      const text = [...under.childNodes].some(
        (node) => node.nodeType === Node.TEXT_NODE && (node.textContent ?? '').trim() !== '',
      )

      return control === null && !text
    }

    return candidates.find(clear)
  }, spots)
}

// centerOf is the middle of a control, which the film has scrolled into
// view.
async function centerOf(target: Locator): Promise<Point> {
  const box = await target.boundingBox()
  if (box === null) {
    throw new Error('the pointer cannot glide to a control that is not shown')
  }

  return { x: box.x + box.width / 2, y: box.y + box.height / 2 }
}

// placePointer moves the mouse to from, the arrow drawPointer drew with it,
// and once a frame shows it there returns the pointer, with clock, the
// film's, to time its glides by.
export async function placePointer(page: Page, from: Point, clock: () => number): Promise<Pointer> {
  await page.mouse.move(from.x, from.y)
  await nextFrame(page)
  let at = from

  // glideTo moves the mouse from where it is to a point in eased steps, a
  // frame each, and says when the glide ran.
  const glideTo = async (to: Point): Promise<Glide> => {
    const steps = stepsBetween(at, to)
    const began = clock()
    for (let step = 1; step <= steps; step++) {
      const along = eased(step / steps)
      await page.mouse.move(at.x + (to.x - at.x) * along, at.y + (to.y - at.y) * along)
      await nextFrame(page)
    }
    at = to

    return { began, ended: clock() }
  }

  return {
    glide: async (target) => glideTo(await centerOf(target)),
    park: async (target) => {
      const spot = await clearSpot(target)

      return spot === undefined ? undefined : glideTo(spot)
    },
  }
}
