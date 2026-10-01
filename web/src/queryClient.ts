import { QueryClient } from '@tanstack/react-query'

// The app-wide query client, for the reads the event stream does not carry.
// The stream — not polling — keeps the cockpit fresh, writing each snapshot to
// useSnapshotStore (src/api/snapshot.ts) rather than to this cache, so by
// default a query fetches once and neither refetches on window focus nor goes
// stale on its own; a query that must be read again sets its own staleTime. A
// test pins these defaults, because they are the cockpit's freshness policy,
// not an incidental choice.
//
// Trade-off TRADE-4: no read here refetches on its own; the stream alone keeps
// the cockpit fresh.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      staleTime: Infinity,
    },
  },
})

// freshFor is how long a read the stream does not carry is taken as current, as
// the terminal takes a pane: opening its section after this reads it again, and
// opening it sooner spends none of the forge's or Jira's requests.
export const freshFor = 30_000
