import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'
import './api/client.ts'
import { sessionStorageKey } from './api/session.ts'
import { FakeEventSource } from './test/fakeEventSource.ts'
import { installMatchMedia, resetMatchMedia } from './test/matchMedia.ts'
import { resetStores } from './test/stores.ts'
import { themeStorageKey } from './shell/themeKey.ts'

// Every zustand store the app makes is recorded as it is made, so each one is
// put back as it was made after every test, a store added later included.
vi.mock('zustand', async (importOriginal) => {
  const { recordingZustand } = await import('./test/stores.ts')

  return recordingZustand(await importOriginal())
})

// jsdom has no EventSource, and the stream hook opens one on mount. Install the
// controllable fake as the global so components that open the stream render,
// and tests can push events through FakeEventSource.latest().
globalThis.EventSource = FakeEventSource as unknown as typeof EventSource

// jsdom has no matchMedia either, and the theme hook queries it on mount.
installMatchMedia()

// jsdom draws a dialog but cannot open one as a modal: showModal and close
// only set and clear its open attribute here. The page behind is not made
// inert, so a test cannot see Tab kept inside; the e2e specs, in a browser,
// do.
HTMLDialogElement.prototype.showModal = function showModal(this: HTMLDialogElement) {
  this.setAttribute('open', '')
}
HTMLDialogElement.prototype.close = function close(this: HTMLDialogElement) {
  this.removeAttribute('open')
}
// A browser asks an open modal dialog to close, with a cancel event, when
// Escape is pressed in it; jsdom does not, so this does.
document.addEventListener('keydown', (event) => {
  const shown = document.querySelector('dialog[open]')
  if (event.key === 'Escape' && shown !== null) {
    shown.dispatchEvent(new Event('cancel', { cancelable: true }))
  }
})
// jsdom lays nothing out, so it has nothing to scroll into view.
Element.prototype.scrollIntoView = function scrollIntoView() {}
// jsdom has no ReadableStream, and the vmThreads pool, unlike a plain worker,
// leaks none of Node's into the page. A Response here is Node's, its body
// Node's stream, so the page borrows that constructor, and a test can build
// the streamed answer the client reads.
globalThis.ReadableStream = new Response('').body?.constructor as typeof ReadableStream

// jsdom leaves Node's Request in place, which cannot resolve a relative URL; a
// browser resolves one against the page. The API client builds every request
// from a relative URL (the SPA is same-origin), so resolve against the page too.
class PageRequest extends Request {
  constructor(input: RequestInfo | URL, init?: RequestInit) {
    super(typeof input === 'string' ? new URL(input, window.location.href) : input, init)
  }
}
globalThis.Request = PageRequest

// No server answers a unit test, and the API goes through the same configured
// client as the app (imported above, dry-run hold and all). A read the test did
// not stub — the health read the shell makes on mount — is refused rather than
// sent to the network; a test that needs an answer stubs fetch itself.
beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn(() => Promise.reject(new TypeError('no server answers a unit test'))),
  )
})

// Unmount and clear the DOM, reset the shared stores, and drop opened streams,
// so one test cannot leak a rendered tree, a section, or a snapshot into the
// next. This file is also where jsdom polyfills go as components need them.
afterEach(() => {
  cleanup()
  resetStores()
  localStorage.removeItem(themeStorageKey)
  localStorage.removeItem(sessionStorageKey)
  FakeEventSource.reset()
  resetMatchMedia()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  vi.useRealTimers()
})
