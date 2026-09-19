import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// Unmount and clear the DOM between tests so one render cannot leak into the
// next. This file is also where jsdom polyfills go as components come to need
// them (a native <dialog>, IntersectionObserver, and the like).
afterEach(() => {
  cleanup()
})
