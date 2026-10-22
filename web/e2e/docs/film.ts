import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Locator, type Page } from '@playwright/test'
import {
  cutAtTheEdge,
  cutBoxes,
  cutTolerance,
  fontsLoaded,
  stillAnimating,
  type Cut,
} from './capture.ts'
import { pointerCovers, type Point } from './cover.ts'
import { heldKeys } from './held.ts'
import type { Pace } from './loop.ts'
import { drawPointer, placePointer, type Pointer } from './pointer.ts'

// The walkthrough's film: the page's screencast kept frame by frame with the
// time each was drawn, and the beats the loop's steps hold, written for
// scripts/docs-media.sh to encode and check. The browser sends a frame only
// when it draws one, so a frame lasts until the next; a beat holds the first
// frame that shows its keyframe for a beat's length more (held.ts), so the
// film can hold a step without the test waiting. A still stretch the page
// spends waiting on the demo is cut to stillMs, since nothing in it moves.

// size is the film's frame: the window the walkthrough's project opens.
const size = { width: 1280, height: 800 }

// The film's pace: how long a beat holds its frame, the longest a still
// frame plays, how long the last frame holds, the delay between keys typed,
// and the most two frames inside a glide or typed text may lie apart before
// the film counts the stretch as stalled.
const beatMs = 1200
const stillMs = 600
const lastMs = 3000
const keyDelay = 45
const stallMs = 250

// everyNthFrame is how many of the browser's frames go by for each the film
// keeps: every other one, about 30 a second, smooth for a glide and half the
// WebP of every one.
const everyNthFrame = 2

// mostNudges bounds how a beat clears a line an edge cuts: a few nudges at
// most.
const mostNudges = 4

// Frame is a frame of the film: its file, and when it was drawn.
interface Frame {
  file: string
  at: number
}

// Key is a beat's keyframe: what the step did, when, and the image of the
// page as it stood then.
interface Key {
  caption: string
  at: number
  file: string
}

// Stretch is a span of the film that moves throughout, a glide or typing,
// in which a gap between frames is a frame the browser did not send.
interface Stretch {
  what: string
  from: number
  to: number
}

// Film is the walkthrough's recording: the pace its steps act at, and what
// to do once they are done.
interface Film {
  pace: Pace
  captions: () => string[]
  stalls: () => string[]
  finish: () => Promise<void>
}

// Reel is what a film holds as it runs.
interface Reel {
  dir: string
  frames: Frame[]
  keys: Key[]
  stretches: Stretch[]
}

// stallsIn is every gap between frames longer than stallMs inside a stretch
// that moves throughout, named by its frame and when it fell.
function stallsIn(reel: Reel): string[] {
  return reel.stretches.flatMap((stretch) => {
    const inside = reel.frames.filter((frame) => frame.at >= stretch.from && frame.at <= stretch.to)
    const times = [stretch.from, ...inside.map((frame) => frame.at), stretch.to]

    return times.slice(1).flatMap((at, index) => {
      const gap = at - (times[index] ?? at)

      return gap > stallMs
        ? [
            `${stretch.what}: ${String(Math.round(gap))} ms without a frame at ${String(Math.round(at))} ms`,
          ]
        : []
    })
  })
}

// durations is how long each frame plays: until the next, a still one cut
// to stillMs, plus a beat for each key that landed while it showed, and the
// last for lastMs.
function durations(reel: Reel): number[] {
  return reel.frames.map((frame, index) => {
    const next = reel.frames.at(index + 1)
    const shown = next === undefined ? lastMs : Math.min(next.at - frame.at, stillMs)
    const beats = reel.keys.filter(
      (key) => key.at >= frame.at && (next === undefined || key.at < next.at),
    ).length

    return Math.round(shown) + beats * beatMs
  })
}

// briefMs is the most a frame may play and still be played as long: a
// browser shows a WebP frame of 10 ms or less for 100 ms, as it does a GIF's.
const briefMs = 10

// Played is a frame as the encoding plays it: the frame, and for how long.
interface Played {
  frame: Frame
  ms: number
}

// playedFrames is each frame of the film with how long it plays, a frame
// brief enough that a browser would hold it for 100 ms folded into the frame
// before it, or into the next when it opens the film. A beat's frame holds
// for a beat, so none is ever folded away.
function playedFrames(reel: Reel): Played[] {
  const lasting = durations(reel)
  const played: Played[] = []
  let carried = 0
  reel.frames.forEach((frame, index) => {
    const ms = lasting[index] ?? 0
    const before = played.at(-1)
    if (ms > briefMs) {
      played.push({ frame, ms: ms + carried })
      carried = 0
    } else if (before === undefined) {
      carried += ms
    } else {
      before.ms += ms
    }
  })

  return played
}

// concatOf is the list ffmpeg's concat demuxer reads, each frame for as long
// as it plays. The demuxer reads each PNG at an image's own 25 a second and
// keeps that clock for the whole list, so every start would round to 40 ms
// and two frames drawn in the same 40 ms would start together, leaving a
// frame of 0 ms, which a browser plays for 100 ms; framerate 1000 puts each
// file, and so the film, on a clock of a millisecond.
function concatOf(played: Played[]): string[] {
  const clock = 'option framerate 1000'
  const last = played.at(-1)

  return [
    'ffconcat version 1.0',
    ...played.flatMap(({ frame, ms }) => [
      `file ${frame.file}`,
      clock,
      `duration ${(ms / 1000).toFixed(3)}`,
    ]),
    // The concat demuxer reads a duration only for a file it sees another
    // after, so the last is named twice.
    ...(last === undefined ? [] : [`file ${last.frame.file}`, clock]),
  ]
}

// writeReel writes what ffmpeg reads to encode the film, walkthrough.ffconcat,
// and what docs-media.sh checks the encoding against, walkthrough.txt: one
// line for the film's size and length, then one for each frame as it plays,
// keyframe and stretch that moves throughout.
function writeReel(reel: Reel): void {
  const played = playedFrames(reel)
  const lengthMs = played.reduce((sum, { ms }) => sum + ms, 0)
  const manifest = [
    `size ${String(size.width)} ${String(size.height)}`,
    `length ${String(lengthMs)}`,
    ...played.map(
      ({ frame, ms }) => `frame ${frame.file} ${String(Math.round(frame.at))} ${String(ms)}`,
    ),
    ...reel.keys.map((key) => `key ${key.file} ${String(Math.round(key.at))} ${key.caption}`),
    ...reel.stretches.map(
      (stretch) =>
        `stretch ${String(Math.round(stretch.from))} ${String(Math.round(stretch.to))} ${stretch.what}`,
    ),
  ]
  writeFileSync(join(reel.dir, 'walkthrough.ffconcat'), `${concatOf(played).join('\n')}\n`)
  writeFileSync(join(reel.dir, 'walkthrough.txt'), `${manifest.join('\n')}\n`)
}

// Beat is a step's beat: what the step did, and the element that shows it.
interface Beat {
  caption: string
  shown: Locator
}

// Take is what a step of the film acts through: the page, the reel it adds
// to, the film's clock and the pointer.
interface Take {
  page: Page
  reel: Reel
  clock: () => number
  pointer: Pointer
}

// scrollTops is where every pane on the page stands, as one line to compare.
function scrollTops(page: Page): Promise<string> {
  return page.evaluate(() =>
    [...document.querySelectorAll('*')].map((pane) => pane.scrollTop).join(' '),
  )
}

// settled waits, a frame at a time, until the panes have moved from where
// they stood before a scroll began and then stood still for two frames, and
// says whether they moved; a scroll that never starts, since the page was
// already as far as it goes, gives up after a dozen.
function settled(page: Page, before: string): Promise<boolean> {
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
async function scrollTo(target: Locator, block: 'end' | 'nearest'): Promise<void> {
  await target.evaluate((element, to) => {
    element.scrollIntoView({ block: to, behavior: 'smooth' })
  }, block)
}

// wholeInView says whether shown lies wholly inside the window and every
// pane around it that clips what overflows it.
function wholeInView(shown: Locator): Promise<boolean> {
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

// clearOfPadding says whether target lies clear of the scroll padding of
// every pane around it that scrolls. Playwright's click scrolls a control
// inside that padding clear of it before it presses, as Chromium counts it
// partly hidden, which the film would show as a jump a person's click never
// makes.
function clearOfPadding(target: Locator): Promise<boolean> {
  return target.evaluate((element) => {
    const box = element.getBoundingClientRect()
    let clear = true
    for (let around = element.parentElement; around !== null; around = around.parentElement) {
      const style = getComputedStyle(around)
      if (/^(auto|scroll)$/.test(style.overflowY)) {
        const pane = around.getBoundingClientRect()
        const head = parseFloat(style.scrollPaddingTop) || 0
        const foot = parseFloat(style.scrollPaddingBottom) || 0
        clear &&= box.top >= pane.top + head && box.bottom <= pane.bottom - foot
      }
    }

    return clear
  })
}

// filmedScroll begins a scroll with start and waits until the panes stand
// still, keeping the scroll as a stretch the film moves through when the
// page moved, and says whether it did: one that did not leaves the same
// frames, which the encoding rightly plays as one.
async function filmedScroll(take: Take, start: () => Promise<void>): Promise<boolean> {
  const before = await scrollTops(take.page)
  const from = take.clock()
  await start()
  const moved = await settled(take.page, before)
  if (moved) {
    take.reel.stretches.push({ what: 'a scroll', from, to: take.clock() })
  }

  return moved
}

// bringIntoView scrolls a control the film is about to act on wholly into
// view, and clear of its panes' scroll padding, when it is not, as a person
// scrolls to it before reaching for it; the scroll that brings it into view
// keeps clear of that padding too.
async function bringIntoView(take: Take, target: Locator): Promise<void> {
  if (!(await wholeInView(target)) || !(await clearOfPadding(target))) {
    await filmedScroll(take, () => scrollTo(target, 'nearest'))
  }
}

// scrolledByPress watches the page until its next press and gives each pane
// that, when the press came, stood elsewhere than as it stands now: what a
// click scrolled to reach its control, apart from what the page scrolls
// once pressed.
async function scrolledByPress(page: Page): Promise<() => Promise<string[]>> {
  const watch = await page.evaluateHandle(() => {
    const panes = [...document.querySelectorAll('*')].map((pane) => ({ pane, top: pane.scrollTop }))
    const moved = () =>
      panes
        .filter(({ pane, top }) => pane.scrollTop !== top)
        .map(({ pane, top }) => `${pane.tagName} from ${String(top)} to ${String(pane.scrollTop)}`)

    return {
      moved: new Promise<string[]>((resolve) => {
        addEventListener(
          'pointerdown',
          () => {
            resolve(moved())
          },
          { capture: true, once: true },
        )
      }),
    }
  })

  return async () => {
    const moved = await watch.evaluate((pressed) => pressed.moved)
    await watch.dispose()

    return moved
  }
}

// clickInPlace clicks target, which the film has brought into view, and
// fails when the click itself scrolled any pane to reach it.
async function clickInPlace(page: Page, target: Locator): Promise<void> {
  const scrolled = await scrolledByPress(page)
  await target.click()
  expect(await scrolled(), 'the click scrolled the page').toEqual([])
}

// middleOf is the middle of what of a cut shows, where the pane it lies in
// is found.
function middleOf({ seen }: Cut): Point {
  return { x: (seen.left + seen.right) / 2, y: (seen.top + seen.bottom) / 2 }
}

// Clearing is what scrollClear weighs a pane's scroll by: the elements an
// edge may not cut beside the lines of text, how far past an edge a part may
// overhang before it counts as cut, and the points the cuts show at.
interface Clearing {
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
function scrollClear(shown: Locator, clearing: Clearing): Promise<boolean> {
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

// nudge scrolls the pane a cut lies in, the nearest around it that scrolls,
// the least it can so that nothing in it is cut and what the beat shows
// stays in view, and says whether one could and the pane moved.
async function nudge(take: Take, shown: Locator): Promise<boolean> {
  const points = (await cutAtTheEdge(take.page)).map(middleOf)
  const nudged = { started: false }
  const moved = await filmedScroll(take, async () => {
    nudged.started = await scrollClear(shown, { boxes: cutBoxes, tolerance: cutTolerance, points })
  })

  return nudged.started && moved
}

// reveal brings what a beat shows wholly into view when it is not, or when
// an edge cuts any text or control, then nudges the pane a cut lies in until
// it clears, a few times at most and never so far that what the beat shows
// leaves view, scrolling as the film moves through. What it cannot clear is
// left for the keyframe's checks to name.
async function reveal(take: Take, shown: Locator): Promise<void> {
  if ((await wholeInView(shown)) && (await cutAtTheEdge(take.page)).length === 0) {
    return
  }

  await filmedScroll(take, () => scrollTo(shown, 'end'))
  for (let nudged = 0; nudged < mostNudges; nudged++) {
    if (!(await nudge(take, shown))) {
      break
    }
  }
}

// park moves the pointer, once its pulse is done, off what it covers of the
// page and of what the beat shows, or of the control a click pressed,
// gliding as the film moves through, and waits for what the glide's hover
// left moving to stand still.
async function park(take: Take, shown: Locator): Promise<void> {
  const { page, reel, pointer } = take
  await pointer.still()
  const parked = await pointer.clear(shown)
  if (parked !== undefined) {
    reel.stretches.push({ what: 'a glide', from: parked.began, to: parked.ended })
    await expect.poll(() => stillAnimating(page)).toEqual([])
  }
}

// keyframe holds the page to what every docs image is held to, once what
// the beat shows is in view and the pointer parked clear of it, then keeps
// an image of it as it stands, named by its place in the film, and the
// moment on the film's clock it stood so, from which the beat holds the
// first frame that shows it. The image keeps a field's caret, which a
// screenshot hides unless asked, since the film shows it.
async function keyframe(take: Take, beat: Beat): Promise<void> {
  const { page, reel, clock } = take
  const { caption, shown } = beat
  await expect.poll(() => stillAnimating(page), { message: caption }).toEqual([])
  await reveal(take, shown)
  await park(take, shown)
  expect(await pointerCovers(page, shown), caption).toEqual([])
  expect(await cutAtTheEdge(page), caption).toEqual([])
  await expect(shown, caption).toBeInViewport({ ratio: 1 })
  expect(await fontsLoaded(page), caption).toBe(true)
  const at = clock()
  const file = `keys/${String(reel.keys.length + 1).padStart(2, '0')}.png`
  await page.screenshot({ path: join(reel.dir, file), caret: 'initial' })
  reel.keys.push({ caption, at, file })
}

// screencast sends each frame the page draws to onFrame as a PNG, with when
// it was drawn on the wall clock, until the function it returns is called.
// It asks Chromium's DevTools protocol for PNG, which Playwright's own
// screencast does not offer: a JPEG's noise differs from frame to frame, so
// a lossless WebP of JPEG frames stores every frame whole, and a lossy one
// leaves blocks of earlier frames on screen where it blends a frame over the
// last. In PNG, what did not change is the same pixels in the next frame, so
// the encoding keeps only what moved, exactly.
async function screencast(
  page: Page,
  onFrame: (data: Buffer, drawn: number) => void,
): Promise<() => Promise<void>> {
  const session = await page.context().newCDPSession(page)
  session.on('Page.screencastFrame', ({ data, metadata, sessionId }) => {
    onFrame(Buffer.from(data, 'base64'), (metadata.timestamp ?? 0) * 1000)
    void session.send('Page.screencastFrameAck', { sessionId })
  })
  await session.send('Page.startScreencast', {
    format: 'png',
    maxWidth: size.width,
    maxHeight: size.height,
    everyNthFrame,
  })

  return async () => {
    await session.send('Page.stopScreencast')
    await session.detach()
  }
}

// Camera is a film made ready before the page loads, the pointer drawn on
// it, whose roll starts it once the page is live.
export interface Camera {
  roll: () => Promise<Film>
}

// pointerStart is where the pointer stands as the film opens: the middle of
// its frame.
const pointerStart = { x: size.width / 2, y: size.height / 2 }

// prepareFilm readies a film of the page, which must not have loaded yet,
// into dir, emptied first: it draws the pointer on every document the page
// loads, and records nothing until roll.
export async function prepareFilm(page: Page, dir: string): Promise<Camera> {
  rmSync(dir, { recursive: true, force: true })
  mkdirSync(join(dir, 'frames'), { recursive: true })
  mkdirSync(join(dir, 'keys'), { recursive: true })
  await drawPointer(page)

  return { roll: () => roll(page, dir) }
}

// roll starts the film's clock, places the pointer at its start, so the
// first frame already shows it there, and then starts the screencast.
async function roll(page: Page, dir: string): Promise<Film> {
  const reel: Reel = { dir, frames: [], keys: [], stretches: [] }
  const began = Date.now()
  const clock = () => Date.now() - began
  const pointer = await placePointer(page, pointerStart, clock)
  const stop = await screencast(page, (data, drawn) => {
    const file = `frames/${String(reel.frames.length + 1).padStart(5, '0')}.png`
    writeFileSync(join(dir, file), data)
    reel.frames.push({ file, at: drawn - began })
  })

  return {
    pace: filmedPace({ page, reel, clock, pointer }),
    captions: () => reel.keys.map((key) => key.caption),
    stalls: () => stallsIn(reel),
    finish: async () => {
      await stop()
      reel.keys = await heldKeys({ page, dir, frames: reel.frames }, reel.keys)
      writeReel(reel)
    },
  }
}

// filmedPace is the walkthrough's pace: the pointer glides to each control
// first, scrolling it into view as it goes, and once it has clicked one it
// parks clear of it and of the page, so it rests on nothing while the page
// does what was pressed; a field is clicked and typed into a key at a time,
// and a choice clicked and chosen, the pointer staying where it pressed
// until a beat parks it; and each beat is a keyframe.
function filmedPace(take: Take): Pace {
  const { reel, clock, pointer } = take
  const glideTo = async (target: Locator) => {
    await bringIntoView(take, target)
    const glide = await pointer.glide(target)
    reel.stretches.push({ what: 'a glide', from: glide.began, to: glide.ended })
  }

  return {
    click: async (target) => {
      await glideTo(target)
      await clickInPlace(take.page, target)
      await park(take, target)
      expect(
        await pointerCovers(take.page, target),
        'the pointer rests clear after a click',
      ).toEqual([])
    },
    type: async (target, text) => {
      await glideTo(target)
      await target.click()
      const from = clock()
      await target.pressSequentially(text, { delay: keyDelay })
      reel.stretches.push({ what: `typing "${text}"`, from, to: clock() })
    },
    choose: async (select, value) => {
      await glideTo(select)
      await select.click()
      await select.selectOption(value)
    },
    beat: (caption, shown) => keyframe(take, { caption, shown }),
  }
}
