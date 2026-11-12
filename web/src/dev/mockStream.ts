// Imported by its relative path, not `@/`: the e2e project reads this type
// too, and only the app's tsconfig declares that alias.
import type { Snapshot } from '../api/generated/types.gen.ts'

// MockStream is how a spec on the mockup scripts its event stream, as the
// server's stream reports what changed without the page asking: show puts
// the parts given in place of the frame's own, in the frame it pushes down
// every open stream now and in each frame after — a hold armed, waiting, then
// announced; a directory switched. The mockup installs it (mockServer.ts) and
// the specs' helper calls it (e2e/support/fixtures.ts), both through this one
// type.
export interface MockStream {
  show: (parts: Partial<Snapshot>) => void
}

declare global {
  interface Window {
    // mockStream scripts the mockup's event stream; only the mockup's build
    // installs it.
    mockStream?: MockStream
  }
}
