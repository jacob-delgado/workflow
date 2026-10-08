import type * as Zustand from 'zustand'
import type { StateCreator, StoreApi } from 'zustand'

// Every store the app has made, as how to put it back as it was made. The test
// setup swaps zustand's create for one that records here, so a store added
// later is reset after every test without anyone listing it.
const resets = new Set<() => void>()

// remember records store and hands it back unchanged.
function remember<Store extends StoreApi<unknown>>(store: Store): Store {
  resets.add(() => {
    store.setState(store.getInitialState(), true)
  })

  return store
}

// recordingZustand is zustand with a create that records each store it makes,
// in both of create's forms: create(initializer) and create<T>()(initializer).
export function recordingZustand(actual: typeof Zustand): typeof Zustand {
  const create = (initializer?: StateCreator<unknown>) =>
    initializer === undefined
      ? (later: StateCreator<unknown>) => remember(actual.create(later))
      : remember(actual.create(initializer))

  return { ...actual, create: create as typeof actual.create }
}

// resetStores puts every store made so far back to its initial state.
export function resetStores(): void {
  for (const reset of resets) {
    reset()
  }
}
