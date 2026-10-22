import { expect, type Locator, type Page } from '@playwright/test'
import { nextFrame, pointerId } from './capture.ts'
import { clearSpot, type Point } from './cover.ts'

// The walkthrough's pointer: an arrow drawn over the page where the mouse is,
// since a headless browser draws none, that glides to each control before it
// is pressed and pulses as it is. It is hidden from assistive technology and
// takes no pointer events, so the page acts as it would without it, and it
// sits outside the body, so the edge check never reads it.

// Glide is a glide's span on the film's clock: when it began and ended.
interface Glide {
  began: number
  ended: number
}

// Pointer moves the mouse in eased steps, the arrow following, and says
// when each glide ran: to a control, or off whatever it covers of the page
// and of what a beat shows, when it covers anything and a clear spot is to
// be had. press presses the control it glided to where it stands, and still
// waits for its pulse to end.
export interface Pointer {
  glide: (target: Locator) => Promise<Glide>
  press: (target: Locator) => Promise<void>
  clear: (shown: Locator) => Promise<Glide | undefined>
  still: () => Promise<void>
}

// pixelsPerStep and the step bounds set a glide's pace: a step a frame, each
// about this far, so a short glide still eases and a long one stays brisk.
const pixelsPerStep = 24
const fewestSteps = 12
const mostSteps = 36

// parkSteps is how many frames the pointer takes to move off what a beat
// shows: a short, quick move, since it goes nowhere a viewer follows.
const parkSteps = 6

// arrow is the pointer's look, the outline of a desktop's arrow, drawn white
// and edged in black.
const arrow = 'M1 1 L1 23 L7 17 L11 27 L15 25 L11 15 L20 15 Z'

// drawPointer puts the arrow on every document the page loads, at the
// mouse, and pulses it at each press: the drawing itself, about the arrow's
// tip, since the box around it is placed by its transform, and scaling that
// would pull the arrow toward the window's corner. It is drawn off the
// window until the mouse first moves, which placePointer makes it do.
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
        drawing.style.transformOrigin = '1px 1px'
        window.addEventListener('mousedown', () => {
          drawing.animate([{ scale: '1' }, { scale: '0.8' }, { scale: '1' }], {
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

// centerOf is the middle of a control, which the film has scrolled into
// view.
async function centerOf(target: Locator): Promise<Point> {
  const box = await target.boundingBox()
  if (box === null) {
    throw new Error('the pointer cannot glide to a control that is not shown')
  }

  return { x: box.x + box.width / 2, y: box.y + box.height / 2 }
}

// pressable holds a control to what Playwright's click checks before it
// presses, short of the scroll that click makes to reach it: shown, enabled,
// and the topmost element at the point the press lands, or inside it.
async function pressable(target: Locator, at: Point): Promise<void> {
  await expect(target, 'the control the pointer presses').toBeVisible()
  await expect(target, 'the control the pointer presses').toBeEnabled()
  const over = await target.evaluate((element, { x, y }) => {
    const hit = document.elementFromPoint(x, y)

    return hit === element || (hit !== null && element.contains(hit))
      ? null
      : (hit?.outerHTML.slice(0, 120) ?? 'nothing')
  }, at)
  if (over !== null) {
    throw new Error(
      `the press at (${String(at.x)}, ${String(at.y)}) lands on ${over}, not on the control`,
    )
  }
}

// pointerStill waits for every animation the pointer runs, its pulse, to
// end, so a beat's checks and keyframe see the arrow at its size.
async function pointerStill(page: Page): Promise<void> {
  await page.evaluate(async (id) => {
    const running = document.getElementById(id)?.getAnimations({ subtree: true }) ?? []
    await Promise.all(running.map((animation) => animation.finished.catch(() => animation)))
  }, pointerId)
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
  const glideTo = async (to: Point, steps: number): Promise<Glide> => {
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
    glide: async (target) => {
      const to = await centerOf(target)

      return glideTo(to, stepsBetween(at, to))
    },
    // A person presses where the pointer stands, scrolling nothing to reach
    // the control, where Playwright's click scrolls one its page counts
    // partly hidden.
    press: async (target) => {
      await pressable(target, at)
      await page.mouse.down()
      await page.mouse.up()
    },
    clear: async (shown) => {
      const spot = await clearSpot(page, at, shown)

      return spot === undefined ? undefined : glideTo(spot, parkSteps)
    },
    still: () => pointerStill(page),
  }
}
