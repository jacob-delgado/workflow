import type { Locator, Page } from '@playwright/test'
import { partsShown, pointerId, type Box } from './capture.ts'

// Where the walkthrough's pointer may rest while a beat holds a frame: never
// over what a viewer reads in it. The arrow, a few pixels around it, must
// cover no line of text, no control, icon, image or status line, and
// nothing of what the beat shows; where it does, it parks at the nearest
// spot that covers nothing, wholly inside the window. After a press it
// parks where nothing the press brings up can land under it.

// Point is where the mouse is, in the window's CSS pixels.
export interface Point {
  x: number
  y: number
}

// Size is the window's, in CSS pixels.
interface Size {
  width: number
  height: number
}

// arrowPad is how far around the arrow nothing may lie.
const arrowPad = 4

// ringGap and ringSpots are where a park is looked for: on rings this far
// apart around the pointer, nearest first, at this many points each; and
// down the empty rail below its last section, and along the row beside the
// main pane's column, this far apart.
const ringGap = 40
const ringSpots = 16

// takenBy is every element the arrow may not cover, beside the lines of
// text.
const takenBy = 'svg, img, input, select, textarea, button, [role="status"]'

// Taken is a part of the page the arrow may not cover, by what of it shows.
interface Taken {
  what: string
  box: Box
}

// Ground is the page under the pointer: the arrow's box, padded, and every
// part of the page it may not cover.
interface Ground {
  arrow: Box
  taken: Taken[]
}

// arrowBox is the box the arrow draws in, padded by arrowPad.
async function arrowBox(page: Page): Promise<Box> {
  const box = await page.evaluate((id) => {
    const drawn = document.getElementById(id)?.querySelector('svg')?.getBoundingClientRect()

    return drawn === undefined
      ? null
      : { left: drawn.left, top: drawn.top, right: drawn.right, bottom: drawn.bottom }
  }, pointerId)
  if (box === null) {
    throw new Error('the walkthrough draws no pointer')
  }

  return {
    left: box.left - arrowPad,
    top: box.top - arrowPad,
    right: box.right + arrowPad,
    bottom: box.bottom + arrowPad,
  }
}

// shownBox is the box of what the beat shows as the page stands, read in one
// evaluation that waits for nothing; none once the page no longer shows it,
// or draws it nowhere: a click can take away or rename the control it
// pressed at any moment the server answers, and a box read by waiting would
// wait for it.
async function shownBox(shown: Locator): Promise<Box | null> {
  return shown.evaluateAll((all) => {
    const drawn = all.at(0)?.getBoundingClientRect()

    return drawn === undefined || (drawn.width === 0 && drawn.height === 0)
      ? null
      : { left: drawn.left, top: drawn.top, right: drawn.right, bottom: drawn.bottom }
  })
}

// groundOf is the ground under the pointer as the page stands, what the beat
// shows taken with the rest while the page still shows it.
async function groundOf(page: Page, shown: Locator): Promise<Ground> {
  const arrow = await arrowBox(page)
  const parts = await partsShown(page, takenBy)
  const beat = await shownBox(shown)
  const taken = parts.flatMap((part) =>
    part.seen === null ? [] : [{ what: part.what, box: part.seen }],
  )
  if (beat !== null) {
    taken.push({ what: 'what the beat shows', box: beat })
  }

  return { arrow, taken }
}

// overlap says whether two boxes share any area.
function overlap(one: Box, other: Box): boolean {
  return (
    one.left < other.right &&
    other.left < one.right &&
    one.top < other.bottom &&
    other.top < one.bottom
  )
}

// covered is what of the taken an arrow drawn in a box covers.
function covered(arrow: Box, taken: Taken[]): string[] {
  return taken.filter((part) => overlap(arrow, part.box)).map((part) => part.what)
}

// pointerCovers is what the pointer, as it stands, covers of the page and of
// what the beat shows, each named; nothing when it rests clear.
export async function pointerCovers(page: Page, shown: Locator): Promise<string[]> {
  const ground = await groundOf(page, shown)

  return covered(ground.arrow, ground.taken)
}

// rings are the spots around a point, a ring at a time out to reach, the
// nearest ring first.
function rings(at: Point, reach: number): Point[] {
  const spots: Point[] = []
  for (let radius = ringGap; radius <= reach; radius += ringGap) {
    for (let spot = 0; spot < ringSpots; spot++) {
      const angle = (2 * Math.PI * spot) / ringSpots
      spots.push({
        x: Math.round(at.x + radius * Math.cos(angle)),
        y: Math.round(at.y + radius * Math.sin(angle)),
      })
    }
  }

  return spots
}

// railSpots are the spots down the rail's empty end, below its last section,
// nearest first, when the pointer stands on the rail, as a section it opened
// leaves it; none when it stands elsewhere.
async function railSpots(page: Page, at: Point): Promise<Point[]> {
  const rail = page.getByRole('navigation', { name: 'Sections' })
  const box = await rail.boundingBox()
  const last = await rail.getByRole('button').last().boundingBox()
  const onRail = box !== null && at.x >= box.x && at.x <= box.x + box.width
  if (!onRail || last === null) {
    return []
  }

  const below = last.y + last.height + ringGap
  const count = Math.max(0, Math.floor((box.y + box.height - below) / ringGap))

  return Array.from({ length: count }, (_, step) => ({ x: at.x, y: below + step * ringGap }))
}

// fitsAt says of a spot whether the arrow, moved there from at, lies wholly
// inside the window and covers nothing it may not.
function fitsAt(ground: Ground, view: Size, at: Point): (spot: Point) => boolean {
  return (spot) => {
    const { arrow } = ground
    const moved = {
      left: arrow.left + spot.x - at.x,
      top: arrow.top + spot.y - at.y,
      right: arrow.right + spot.x - at.x,
      bottom: arrow.bottom + spot.y - at.y,
    }
    const inside =
      moved.left >= 0 && moved.top >= 0 && moved.right <= view.width && moved.bottom <= view.height

    return inside && covered(moved, ground.taken).length === 0
  }
}

// clearSpot is where the pointer, standing at, should park so it covers
// nothing it may not: the empty rail when it stands on the rail, then the
// nearest spot around it whose arrow lies wholly inside the window. None
// when it already covers nothing, or when no spot is clear.
export async function clearSpot(page: Page, at: Point, shown: Locator): Promise<Point | undefined> {
  const ground = await groundOf(page, shown)
  const view = page.viewportSize()
  if (view === null || covered(ground.arrow, ground.taken).length === 0) {
    return undefined
  }

  const spots = [...(await railSpots(page, at)), ...rings(at, Math.hypot(view.width, view.height))]

  return spots.find(fitsAt(ground, view, at))
}

// measuredEnd is the right edge of the column the main pane sets its
// content at: the furthest any element in it drawn at a capped width
// reaches, as a section's content measure is, whatever it draws inside;
// the pane's left edge when nothing in it is capped.
function measuredEnd(main: Locator): Promise<number> {
  return main.evaluate((pane) => {
    const capped = [...pane.querySelectorAll('*')].filter(
      (element) =>
        getComputedStyle(element).maxWidth !== 'none' &&
        element.checkVisibility() &&
        element.getBoundingClientRect().width > 0,
    )

    return Math.max(
      pane.getBoundingClientRect().left,
      ...capped.map((element) => element.getBoundingClientRect().right),
    )
  })
}

// columnEnd is how far right the main pane's column reaches: the column its
// content is set at, or whatever it draws further right, as a list that
// grows with the window does; none when the pointer stands outside the
// pane, as on the rail.
async function columnEnd(page: Page, at: Point, ground: Ground): Promise<number | undefined> {
  const main = page.getByRole('main')
  const pane = await main.boundingBox()
  if (pane === null || at.x < pane.x || at.y < pane.y || at.y > pane.y + pane.height) {
    return undefined
  }

  const inMain = ground.taken.filter(({ box }) => box.left >= pane.x && box.top >= pane.y)

  return Math.max(await measuredEnd(main), ...inMain.map(({ box }) => box.right))
}

// besideSpots are the spots right of the main pane's column on the row the
// pointer stands on, nearest the column first, raised as far as the arrow
// needs to fit above the window's foot. Every line a press says is set
// inside that column, so nothing it says can come up under them; none when
// the pointer stands outside the pane, or the column reaches its right side.
async function besideSpots(page: Page, at: Point, ground: Ground, view: Size): Promise<Point[]> {
  const end = await columnEnd(page, at, ground)
  if (end === undefined) {
    return []
  }

  const y = Math.min(at.y, view.height - (ground.arrow.bottom - at.y))
  const count = Math.max(0, Math.floor((view.width - end) / ringGap) - 1)

  return Array.from({ length: count }, (_, step) => ({ x: end + (step + 1) * ringGap, y }))
}

// pressedSpot is where the pointer, standing at on a control it just
// pressed, should park so that nothing the press brings up lands under it:
// beside the main pane's column on the control's row when a spot there is
// clear, and else where clearSpot finds, the rail's empty end for a section
// opened from the rail.
export async function pressedSpot(
  page: Page,
  at: Point,
  pressed: Locator,
): Promise<Point | undefined> {
  const ground = await groundOf(page, pressed)
  const view = page.viewportSize()
  const beside =
    view === null
      ? undefined
      : (await besideSpots(page, at, ground, view)).find(fitsAt(ground, view, at))

  return beside ?? clearSpot(page, at, pressed)
}
