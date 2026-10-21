import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test, type Page, type TestInfo } from '@playwright/test'

// The walkthrough's WebP, held to the film it encodes: each frame the film
// holds, a step's beat or its last, is decoded from the WebP as Chromium
// shows it, halfway through its hold, and compared with the film's own
// frame. scripts/docs-media.sh runs this once it has encoded the film, so a
// WebP that shows anything but the film where a viewer looks longest, such
// as blocks of earlier frames a lossy encoding left on screen, fails the
// run, naming the moment and the frame.

// heldMs is how long a frame plays before it counts as held: a beat holds
// its frame 1.2 s more than it showed, and a still frame plays 0.6 s at
// most.
const heldMs = 1000

// A held frame is the film's when none of its pixels differs from the
// film's by more than tolerance in any channel: the lossless encoding, as
// Chromium decodes it, differs in none at all, so tolerance allows only the
// rounding a decoder may do. Any more is a wrong frame: the one before a
// beat, held in its place, differs in a few hundred pixels, a status line or
// the pointer's last step, and a lossy encoding's blocks of earlier frames
// left on screen differed in four to six hundredths of them.
const tolerance = 2

// Held is a frame the film holds: its file in the film, and the moment
// halfway through its hold, in film milliseconds.
interface Held {
  file: string
  midway: number
}

// metadataPath is a path the docs config names for the project.
function metadataPath(testInfo: TestInfo, name: string): string {
  const value: unknown = testInfo.project.metadata[name]
  if (typeof value !== 'string') {
    throw new Error(`the docs project names no ${name}`)
  }

  return value
}

// filmArea is how many pixels a frame of the film has, from its manifest's
// size line, which a frame's differing pixels are a share of.
function filmArea(manifest: string): number {
  const [, width = '0', height = '0'] = /^size (\d+) (\d+)$/m.exec(manifest) ?? []

  return Number(width) * Number(height)
}

// heldFrames is every frame the film's manifest holds, in order.
function heldFrames(manifest: string): Held[] {
  let at = 0

  return manifest
    .split('\n')
    .filter((line) => line.startsWith('frame '))
    .flatMap((line) => {
      const [, file = '', , plays = '0'] = line.split(' ')
      const ms = Number(plays)
      const held = ms >= heldMs ? [{ file, midway: at + ms / 2 }] : []
      at += ms

      return held
    })
}

// differing decodes, in the page, the WebP frame showing at each held
// frame's midway moment, and counts its pixels that differ from the film's
// frame by more than tolerance in a channel.
function differing(page: Page, webp: Buffer, held: Held[], film: string): Promise<number[]> {
  const sources = held.map((frame) => readFileSync(join(film, frame.file)).toString('base64'))

  return page.evaluate(
    async ({ webp, midways, sources, tolerance }) => {
      const bytes = (base64: string) => Uint8Array.from(atob(base64), (c) => c.charCodeAt(0))
      const pixels = (source: CanvasImageSource, width: number, height: number) => {
        const canvas = new OffscreenCanvas(width, height)
        const context = canvas.getContext('2d')
        if (context === null) {
          throw new Error('no 2d context to read pixels with')
        }
        context.drawImage(source, 0, 0)

        return context.getImageData(0, 0, width, height).data
      }
      const decoder = new ImageDecoder({ data: bytes(webp), type: 'image/webp' })
      await decoder.tracks.ready
      const count = decoder.tracks.selectedTrack?.frameCount ?? 0
      const starts: number[] = []
      let at = 0
      for (let index = 0; index < count; index++) {
        const { image } = await decoder.decode({ frameIndex: index })
        starts.push(at)
        at += (image.duration ?? 0) / 1000
        image.close()
      }

      return Promise.all(
        midways.map(async (midway, held) => {
          const index = starts.findLastIndex((start) => start <= midway)
          const { image } = await decoder.decode({ frameIndex: Math.max(index, 0) })
          const width = image.displayWidth
          const height = image.displayHeight
          const shown = pixels(image, width, height)
          const filmed = await createImageBitmap(
            new Blob([bytes(sources[held] ?? '')], { type: 'image/png' }),
          )
          const own = pixels(filmed, width, height)
          let differ = 0
          for (let byte = 0; byte < shown.length; byte += 4) {
            const off = [0, 1, 2].some(
              (channel) =>
                Math.abs((shown[byte + channel] ?? 0) - (own[byte + channel] ?? 0)) > tolerance,
            )
            differ += off ? 1 : 0
          }

          return differ
        }),
      )
    },
    {
      webp: webp.toString('base64'),
      midways: held.map((frame) => frame.midway),
      sources,
      tolerance,
    },
  )
}

test('the walkthrough WebP shows the film itself wherever it holds a frame', async ({
  page,
}, testInfo) => {
  // Arrange
  const film = metadataPath(testInfo, 'walkthroughDir')
  const webp = readFileSync(metadataPath(testInfo, 'webp'))
  const manifest = readFileSync(join(film, 'walkthrough.txt'), 'utf8')
  const held = heldFrames(manifest)
  // ImageDecoder is offered only to a secure page, which an address the
  // browser is answered for counts as.
  await page.route('https://decode.test/', (route) =>
    route.fulfill({ contentType: 'text/html', body: '<!doctype html><title>decode</title>' }),
  )
  await page.goto('https://decode.test/')

  // Act
  const counts = await differing(page, webp, held, film)

  // Assert
  const area = filmArea(manifest)
  const unlike = held.flatMap((frame, index) => {
    const differ = counts[index] ?? 0
    const share = ((differ / area) * 100).toFixed(2)

    return differ > 0
      ? [
          `at ${(frame.midway / 1000).toFixed(2)} s it shows not the film's ${frame.file}: ${String(differ)} pixels differ, ${share}% of the frame`,
        ]
      : []
  })
  expect(held.length, 'the film holds a frame').toBeGreaterThan(0)
  expect(unlike).toEqual([])
})
