import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'
import { useSnapshotStore } from './api/snapshot.ts'
import { FakeEventSource } from './test/fakeEventSource.ts'
import { installMatchMedia, resetMatchMedia } from './test/matchMedia.ts'
import { useThemeStore } from './shell/themeStore.ts'
import { useUiStore } from './shell/uiStore.ts'

const initialUi = useUiStore.getInitialState()
const initialSnapshot = useSnapshotStore.getInitialState()

// jsdom has no EventSource, and the stream hook opens one on mount. Install the
// controllable fake as the global so components that open the stream render,
// and tests can push events through FakeEventSource.latest().
globalThis.EventSource = FakeEventSource as unknown as typeof EventSource

// jsdom has no matchMedia either, and the theme hook queries it on mount.
installMatchMedia()

// Unmount and clear the DOM, reset the shared stores, and drop opened streams,
// so one test cannot leak a rendered tree, a section, or a snapshot into the
// next. This file is also where jsdom polyfills go as components need them.
afterEach(() => {
  cleanup()
  useUiStore.setState(initialUi)
  useSnapshotStore.setState(initialSnapshot)
  useThemeStore.setState({ choice: 'system' })
  localStorage.removeItem('workflow-theme')
  FakeEventSource.reset()
  resetMatchMedia()
  vi.unstubAllEnvs()
})
