import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import type { Page } from '@playwright/test'

// The frame a beat holds, held to the beat's keyframe. The checks a keyframe
// passes run on the live page and on its own screenshot, never on the film,
// and the film can lag them: the browser sends only every other frame it
// draws, sends none while the page stands still, and stamps a frame with
// when it was drawn, which can fall after the keyframe's moment. So the
// frame showing at that moment can be the one before the beat's last
// change, such as the pointer a step short of where it parked. The beat
// holds instead the first frame from that moment on that shows what its
// keyframe does, and a beat whose keyframe no frame shows fails by name.

// A frame shows a keyframe when none of its pixels differs from the
// keyframe's by more than tolerance in a channel: the screencast and a
// screenshot of the same page round its text's edges apart by one.
const tolerance = 2

// Shot is an image the film keeps: its file, and its moment on the film's
// clock.
interface Shot {
  file: string
  at: number
}

// Keyed is a beat's keyframe: an image of the page and what the beat shows.
interface Keyed extends Shot {
  caption: string
}

// base64Of is the image file a shot names, in the film's directory, as the
// page reads it.
function base64Of(dir: string, shot: Shot): string {
  return readFileSync(join(dir, shot.file)).toString('base64')
}

// shows says, decoding both in the page, whether a frame of the film shows
// what a keyframe does.
function shows(page: Page, frame: string, key: string): Promise<boolean> {
  return page.evaluate(
    async ({ frame, key, tolerance }) => {
      const pixels = async (base64: string) => {
        const bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
        const image = await createImageBitmap(new Blob([bytes], { type: 'image/png' }))
        const canvas = new OffscreenCanvas(image.width, image.height)
        const context = canvas.getContext('2d')
        if (context === null) {
          throw new Error('no 2d context to read pixels with')
        }
        context.drawImage(image, 0, 0)

        return context.getImageData(0, 0, image.width, image.height).data
      }
      const [drawn, kept] = await Promise.all([pixels(frame), pixels(key)])

      return (
        drawn.length === kept.length &&
        drawn.every(
          (value, byte) => byte % 4 === 3 || Math.abs(value - (kept[byte] ?? 0)) <= tolerance,
        )
      )
    },
    { frame, key, tolerance },
  )
}

// Reel is the film a beat's frame is found in: the page that decodes it,
// the directory it was written to, and its frames in order.
interface Reel {
  page: Page
  dir: string
  frames: Shot[]
}

// heldAt is the moment the beat of key holds from: that of the first frame,
// from the one showing at the keyframe's moment and before until, the next
// beat's, that shows the keyframe.
async function heldAt({ page, dir, frames }: Reel, key: Keyed, until: number): Promise<number> {
  const from = Math.max(
    frames.findLastIndex((frame) => frame.at <= key.at),
    0,
  )
  const image = base64Of(dir, key)
  for (const frame of frames.slice(from).filter((frame) => frame.at < until)) {
    if (await shows(page, base64Of(dir, frame), image)) {
      return frame.at
    }
  }

  throw new Error(`no frame of the film shows the keyframe of "${key.caption}", ${key.file}`)
}

// heldKeys is each key moved to the moment of the first frame that shows
// it, so the beat holds that frame, in order; it fails on a keyframe the film
// never shows.
export async function heldKeys<K extends Keyed>(reel: Reel, keys: K[]): Promise<K[]> {
  const held: K[] = []
  for (const [index, key] of keys.entries()) {
    const until = keys.at(index + 1)?.at ?? Infinity
    held.push({ ...key, at: await heldAt(reel, key, until) })
  }

  return held
}
