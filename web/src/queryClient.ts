import { QueryClient } from '@tanstack/react-query'

// The app-wide query client. The event stream — not polling — keeps the cache
// fresh: a query fetches once and the stream's snapshots update it through
// setQueryData, so queries neither refetch on window focus nor go stale on their
// own. A test pins these defaults, because they are the cockpit's freshness
// policy, not an incidental choice.
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      staleTime: Infinity,
    },
  },
})
