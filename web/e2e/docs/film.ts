import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, type Locator, type Page } from '@playwright/test'
import { cutAtTheEdge, fontsLoaded, stillAnimating } from './capture.ts'
import type { Pace } from './loop.ts'
import { drawPointer, placePointer, type Pointer } from './pointer.ts'

// The walkthrough's film: the page's screencast kept frame by frame with the
// time each was drawn, and the beats the loop's steps hold, written for
// scripts/docs-media.sh to encode and check. The browser sends a frame only
// when it draws one, so a frame lasts until the next; a beat holds the frame
// it lands on for a beat's length more, so the film can hold a step without
// the test waiting. A still stretch the page spends waiting on the demo is
// cut to a beat's length, since nothing in it moves.

// size is the film's frame: the window the walkthrough's project opens.
const size = { width: 1280, height: 800 }

// The film's pace: how long a beat holds its frame, the longest a still
// frame plays, how long the last frame holds, the delay between keys typed,
// and the most two frames inside a glide or typed text may lie apart before
// the film counts the stretch as stalled.
const beatMs = 1800
const stillMs = 1000
const lastMs = 3000
const keyDelay = 45
const stallMs = 250

// everyNthFrame is how many of the browser's frames go by for each the film
// keeps: every other one, about 30 a second, smooth for a glide and half the
// WebP of every one.
const everyNthFrame = 2

// mostNudges and nudgeGap bound how a beat clears a line the window's edge
// cuts: a few nudges at most, each leaving the line this far above the edge.
const mostNudges = 4
const nudgeGap = 12

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

// writeReel writes what ffmpeg reads to encode the film, walkthrough.ffconcat,
// and what docs-media.sh checks the encoding against, walkthrough.txt: one
// line for the film's size and length, then one for each frame, keyframe and
// stretch that moves throughout.
function writeReel(reel: Reel): void {
  const played = durations(reel)
  const lengthMs = played.reduce((sum, ms) => sum + ms, 0)
  const last = reel.frames.at(-1)
  const concat = [
    'ffconcat version 1.0',
    ...reel.frames.flatMap((frame, index) => [
      `file ${frame.file}`,
      `duration ${((played[index] ?? 0) / 1000).toFixed(3)}`,
    ]),
    // The concat demuxer reads a duration only for a file it sees another
    // after, so the last is named twice.
    ...(last === undefined ? [] : [`file ${last.file}`]),
  ]
  const manifest = [
    `size ${String(size.width)} ${String(size.height)}`,
    `length ${String(lengthMs)}`,
    ...reel.frames.map(
      (frame, index) =>
        `frame ${frame.file} ${String(Math.round(frame.at))} ${String(played[index] ?? 0)}`,
    ),
    ...reel.keys.map((key) => `key ${key.file} ${String(Math.round(key.at))} ${key.caption}`),
    ...reel.stretches.map(
      (stretch) =>
        `stretch ${String(Math.round(stretch.from))} ${String(Math.round(stretch.to))} ${stretch.what}`,
    ),
  ]
  writeFileSync(join(reel.dir, 'walkthrough.ffconcat'), `${concat.join('\n')}\n`)
  writeFileSync(join(reel.dir, 'walkthrough.txt'), `${manifest.join('\n')}\n`)
}

// Beat is a step's beat: what the step did, and the element that shows it.
interface Beat {
  caption: string
  shown: Locator
}

// Take is what a step of the film acts through: the page, the reel it adds
// to, and the film's clock.
interface Take {
  page: Page
  reel: Reel
  clock: () => number
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

// scrollBy starts a smooth scroll of pane down by a number of pixels.
async function scrollBy(pane: Locator, by: number): Promise<void> {
  await pane.evaluate((element, top) => {
    element.scrollBy({ top, behavior: 'smooth' })
  }, by)
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

// filmedScroll begins a scroll with start and waits until the panes stand
// still, keeping the scroll as a stretch the film moves through when the
// page moved: one that did not leaves the same frames, which the encoding
// rightly plays as one.
async function filmedScroll(take: Take, start: () => Promise<void>): Promise<void> {
  const before = await scrollTops(take.page)
  const from = take.clock()
  await start()
  if (await settled(take.page, before)) {
    take.reel.stretches.push({ what: 'a scroll', from, to: take.clock() })
  }
}

// bringIntoView scrolls a control the film is about to act on wholly into
// view when it is not, as a person scrolls to it before reaching for it.
async function bringIntoView(take: Take, target: Locator): Promise<void> {
  if (!(await wholeInView(target))) {
    await filmedScroll(take, () => scrollTo(target, 'nearest'))
  }
}

// reveal brings what a beat shows wholly into the window when it is not, or
// when the window's edge cuts any text, then nudges the page down past each
// line still cut, a few times at most, scrolling as the film moves through.
// What it cannot clear is left for the keyframe's checks to name.
async function reveal(take: Take, shown: Locator): Promise<void> {
  const { page } = take
  if ((await wholeInView(shown)) && (await cutAtTheEdge(page)).length === 0) {
    return
  }

  await filmedScroll(take, () => scrollTo(shown, 'end'))
  for (let nudge = 0; nudge < mostNudges; nudge++) {
    const cuts = await cutAtTheEdge(page)
    if (cuts.length === 0) {
      break
    }
    const deepest = Math.max(...cuts.map((cut) => cut.bottom))
    await filmedScroll(take, () =>
      scrollBy(page.getByRole('main'), deepest - size.height + nudgeGap),
    )
  }
}

// keyframe holds the page to what every docs image is held to, once what
// the beat shows is in view, then keeps an image of it as it stands, named
// by its place in the film, and the moment on the film's clock it stood so,
// which the beat holds.
async function keyframe(take: Take, beat: Beat): Promise<void> {
  const { page, reel, clock } = take
  const { caption, shown } = beat
  await expect.poll(() => stillAnimating(page), { message: caption }).toEqual([])
  await reveal(take, shown)
  expect(await cutAtTheEdge(page), caption).toEqual([])
  await expect(shown, caption).toBeInViewport({ ratio: 1 })
  expect(await fontsLoaded(page), caption).toBe(true)
  const at = clock()
  const file = `keys/${String(reel.keys.length + 1).padStart(2, '0')}.png`
  await page.screenshot({ path: join(reel.dir, file) })
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
    pace: filmedPace({ page, reel, clock }, pointer),
    captions: () => reel.keys.map((key) => key.caption),
    stalls: () => stallsIn(reel),
    finish: async () => {
      await stop()
      writeReel(reel)
    },
  }
}

// filmedPace is the walkthrough's pace: the pointer glides to each control
// first, scrolling it into view as it goes; a field is clicked and typed into
// a key at a time, and a choice clicked and chosen; and each beat is a
// keyframe.
function filmedPace(take: Take, pointer: Pointer): Pace {
  const { reel, clock } = take
  const glideTo = async (target: Locator) => {
    await bringIntoView(take, target)
    const glide = await pointer.glide(target)
    reel.stretches.push({ what: 'a glide', from: glide.began, to: glide.ended })
  }

  return {
    click: async (target) => {
      await glideTo(target)
      await target.click()
      // A control a press takes away, as Start work, leaves nowhere to
      // park beside, and the pointer stays where it pressed.
      const parked = (await target.isVisible()) ? await pointer.park(target) : undefined
      if (parked !== undefined) {
        reel.stretches.push({ what: 'a glide', from: parked.began, to: parked.ended })
      }
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
