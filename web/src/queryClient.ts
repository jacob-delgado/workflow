import { QueryClient } from '@tanstack/react-query'

// The app-wide query client, for the reads the event stream does not carry.
// The stream — not polling — keeps the cockpit fresh, writing each snapshot to
// useSnapshotStore (src/api/snapshot.ts) rather than to this cache, so by
// default a query fetches once and neither refetches on window focus nor goes
// stale on its own; a query that must be read again sets its own staleTime. A
// test pins these defaults, because they are the cockpit's freshness policy,
// not an incidental choice.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      staleTime: Infinity,
    },
  },
})
