// A controllable window.matchMedia for jsdom, which has none. It reports "not
// dark" (the unit tests do not depend on the OS scheme — the e2e drives that
// with Playwright's emulateMedia) and tracks the change listeners the theme hook
// registers, so a test can prove the hook adds one while following the OS and
// removes it when it stops.

const listeners = new Set<(event: MediaQueryListEvent) => void>()

// installMatchMedia puts the stub on the global, so a component that queries the
// color scheme renders instead of throwing.
export function installMatchMedia(): void {
  globalThis.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.add(listener)
    },
    removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      listeners.delete(listener)
    },
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => true,
  })) as unknown as typeof window.matchMedia
}

// listenerCount is how many change listeners are currently registered, so a
// test can prove the theme hook registers one while following the OS and removes
// it when it stops — the leak the hook's cleanup exists to prevent.
export function listenerCount(): number {
  return listeners.size
}

// resetMatchMedia clears the tracked listeners between tests.
export function resetMatchMedia(): void {
  listeners.clear()
}
