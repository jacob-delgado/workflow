import { spawn, type ChildProcess } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, rm } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { createInterface } from 'node:readline'
import { expect, test as playwright, type Page, type TestInfo } from '@playwright/test'
import { sessionStorageKey } from '../../src/api/session.ts'

// The docs' media are taken over the real --web server, served by cmd/demo
// over its made-up world (internal/demo), a fresh one for each test, so a
// write one screen makes never shows in another's. A test takes this test,
// not e2e/support/fixtures.ts's, whose page refuses every API call: here the
// server answers them all.

// Demo is a demo server a test's page is served by: where it serves, the
// session it prints, and every other line it said on stderr, which a test
// fails on, since the server says a line only for a failure it answered as
// internal.
export interface Demo {
  origin: string
  session: string
  said: string[]
}

// started is how long a demo has to say where it serves, and stopped how long
// it has to end once told to: the web server drains for up to five seconds.
const started = 15_000
const stopped = 10_000

// metadataOf is the project's metadata entry named, which the docs config
// sets: a string, or the run is misconfigured.
export function metadataOf(testInfo: TestInfo, name: string): string {
  const value: unknown = testInfo.project.metadata[name]
  if (typeof value !== 'string') {
    throw new Error(`the docs project names no ${name}`)
  }

  return value
}

// served is the address a demo says it serves, read from its stderr, with
// every line after it kept in said.
function served(child: ChildProcess, said: string[]): Promise<string> {
  const lines = createInterface({ input: child.stderr ?? process.stdin })

  return new Promise((resolve, reject) => {
    const late = setTimeout(() => {
      reject(new Error(`the demo did not say where it serves within ${String(started)} ms`))
    }, started)
    lines.on('line', (line) => {
      const address = /serving (\S+)/.exec(line)?.[1]
      if (address === undefined) {
        said.push(line)

        return
      }
      clearTimeout(late)
      resolve(address)
    })
    child.on('exit', (code) => {
      clearTimeout(late)
      reject(new Error(`the demo exited ${String(code)} before it served: ${said.join('; ')}`))
    })
  })
}

// closed is when a demo has gone and its stderr has been read to the end,
// every line it said among said: Node may fire a child's 'exit' with its
// output still in the pipe, but 'close' only once every stream has closed.
function closed(child: ChildProcess): Promise<void> {
  return new Promise((resolve) => {
    child.once('close', () => {
      resolve()
    })
  })
}

// stop ends a demo as Ctrl+C would, unless it has exited already, by its own
// hand or by a signal from outside, and waits until gone holds that it closed.
async function stop(child: ChildProcess, gone: Promise<void>): Promise<void> {
  if (child.exitCode === null && child.signalCode === null) {
    child.kill('SIGTERM')
  }
  const late = setTimeout(() => child.kill('SIGKILL'), stopped)
  await gone
  clearTimeout(late)
}

// World is how a test's demo departs from the demo as it begins: holdCI
// keeps its end-to-end check running, and autoAnnounce, when false, arms no
// announcement as a pull request opens.
interface World {
  holdCI: boolean
  autoAnnounce: boolean
}

// demoArgs are the demo's arguments for a world: the web server on a port
// the system chooses, naming the build version, and a flag for each way the
// world departs from the demo as it begins.
function demoArgs(version: string, world: World): string[] {
  return [
    '-web',
    '-port',
    '0',
    '-version',
    version,
    ...(world.holdCI ? ['-hold-ci'] : []),
    ...(world.autoAnnounce ? [] : ['-no-auto-announce']),
  ]
}

// launch starts the demo binary the docs config names over a world, in a
// home of its own with nothing of the machine's but PATH, and in UTC, as the
// screenshots read the time; gone is when it has closed, watched from its
// start so a demo that closes early is not waited on for good.
async function launch(
  testInfo: TestInfo,
  world: World,
): Promise<{ child: ChildProcess; gone: Promise<void>; home: string }> {
  const binary = metadataOf(testInfo, 'demo')
  if (!existsSync(binary)) {
    throw new Error(`no demo at ${binary}: build it with go build -o tmp/demo ./cmd/demo`)
  }

  const home = await mkdtemp(join(tmpdir(), 'workflow-demo-'))
  const args = demoArgs(metadataOf(testInfo, 'version'), world)
  const child = spawn(binary, args, {
    env: { PATH: process.env.PATH, HOME: home, TZ: 'UTC' },
    stdio: ['ignore', 'ignore', 'pipe'],
  })

  return { child, gone: closed(child), home }
}

// pinSession keeps the demo's session where the page reads it, before the
// page loads, so every goto('/') the specs make presents it.
async function pinSession(page: Page, session: string): Promise<void> {
  await page.addInitScript(
    ({ key, value }) => {
      window.localStorage.setItem(key, value)
    },
    { key: sessionStorageKey, value: session },
  )
}

// test is Playwright's, with a demo served for each test, the page's base
// address at it and its session pinned. holdCI keeps the demo's end-to-end
// check running, for a screen that shows CI part way, and autoAnnounce false
// arms no announcement as #128 opens, for a screen of the composer alone.
// Each fixture hands its value on through provide, Playwright's use, named
// apart from React's.
export const test = playwright.extend<World & { demo: Demo }>({
  holdCI: [false, { option: true }],
  autoAnnounce: [true, { option: true }],
  demo: async ({ holdCI, autoAnnounce }, provide, testInfo) => {
    const { child, gone, home } = await launch(testInfo, { holdCI, autoAnnounce })
    const said: string[] = []
    try {
      const address = await served(child, said)
      const [origin = '', session = ''] = address.split('/#session=')
      await provide({ origin, session, said })
    } finally {
      await stop(child, gone)
      await rm(home, { recursive: true, force: true })
    }
    expect(said, 'the demo said only where it serves').toEqual([])
  },
  baseURL: async ({ demo }, provide) => {
    await provide(demo.origin)
  },
  page: async ({ page, demo }, provide) => {
    await pinSession(page, demo.session)
    await provide(page)
  },
})

export { expect } from '@playwright/test'
