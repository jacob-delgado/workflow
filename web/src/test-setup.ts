import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'
import { useUiStore } from './shell/uiStore.ts'

const initialUi = useUiStore.getInitialState()

// Unmount and clear the DOM, and reset the shared UI store, so one test cannot
// leak a rendered tree or a selected section into the next. This file is also
// where jsdom polyfills go as components come to need them.
afterEach(() => {
  cleanup()
  useUiStore.setState(initialUi)
})
