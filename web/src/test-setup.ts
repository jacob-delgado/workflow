import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'
import './api/client.ts'
import { useHealthStore } from './api/health.ts'
import { useSnapshotStore } from './api/snapshot.ts'
import { FakeEventSource } from './test/fakeEventSource.ts'
import { installMatchMedia, resetMatchMedia } from './test/matchMedia.ts'
import { useThemeStore } from './shell/themeStore.ts'
import { useUiStore } from './shell/uiStore.ts'

const initialUi = useUiStore.getInitialState()
const initialSnapshot = useSnapshotStore.getInitialState()
const initialHealth = useHealthStore.getInitialState()

// jsdom has no EventSource, and the stream hook opens one on mount. Install the
// controllable fake as the global so components that open the stream render,
// and tests can push events through FakeEventSource.latest().
globalThis.EventSource = FakeEventSource as unknown as typeof EventSource

// jsdom has no matchMedia either, and the theme hook queries it on mount.
installMatchMedia()

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
  useUiStore.setState(initialUi)
  useSnapshotStore.setState(initialSnapshot)
  useHealthStore.setState(initialHealth)
  useThemeStore.setState({ choice: 'system' })
  localStorage.removeItem('workflow-theme')
  FakeEventSource.reset()
  resetMatchMedia()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})
