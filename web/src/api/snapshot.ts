import { useEffect, useEffectEvent } from 'react'
import { create } from 'zustand'
import { zSnapshot } from './generated/zod.gen.ts'
import type { Snapshot } from './generated/types.gen.ts'

export type StreamStatus = 'connecting' | 'live' | 'stale'

interface SnapshotState {
  snapshot: Snapshot | null
  // The issue view the snapshot was read for, by name, or null for the
  // server's default. While it differs from the chosen view a switch is under
  // way, and the snapshot's issues are still the last view's.
  view: string | null
  status: StreamStatus
}

// The latest full read state the event stream has pushed, which view it was
// read for, and whether the stream is connected. The panels read from here;
// useEventStream is the only writer.
export const useSnapshotStore = create<SnapshotState>(() => ({
  snapshot: null,
  view: null,
  status: 'connecting',
}))

// useEventStream opens the Server-Sent Events connection and keeps the snapshot
// store current from its pushes — the cockpit's freshness comes from the stream,
// never from polling. Mount it once, near the root. The stream carries the named
// issue view, or the server's default for null, and reconnects when the view
// changes. Each message is validated against the contract's schema; a payload
// that does not match is dropped rather than allowed to corrupt the last good
// snapshot. A named view the server refuses outright is handed back through
// onViewRefused, since the browser never retries a refused stream.
export function useEventStream(view: string | null, onViewRefused: () => void): void {
  const viewRefused = useEffectEvent(onViewRefused)

  useEffect(() => {
    // `task web:mockup` sets VITE_MOCK so the whole cockpit can be navigated
    // against rich fixture data with no backend. The mock is code-split, so it
    // is never pulled into a production build.
    if (import.meta.env.VITE_MOCK === 'true') {
      void import('@/dev/mockSnapshot.ts').then((module) => {
        useSnapshotStore.setState({ snapshot: module.mockSnapshot, view, status: 'live' })
      })

      return
    }

    useSnapshotStore.setState({ status: 'connecting' })
    const source = new EventSource(eventsURL(view))

    source.addEventListener('open', () => {
      useSnapshotStore.setState({ status: 'live' })
    })

    source.addEventListener('snapshot', (event) => {
      const message = event as MessageEvent<string>

      let raw: unknown
      try {
        raw = JSON.parse(message.data)
      } catch {
        return // a frame that is not even JSON is dropped, like a schema mismatch
      }

      const parsed = zSnapshot.safeParse(raw)
      if (parsed.success) {
        useSnapshotStore.setState({ snapshot: parsed.data, view, status: 'live' })
      }
    })

    source.addEventListener('error', () => {
      // A closed source is one the browser will not retry: the server answered
      // with something other than a stream, as it does (404) for a view a
      // restart dropped from the configuration.
      if (view !== null && source.readyState === EventSource.CLOSED) {
        viewRefused()

        return
      }

      useSnapshotStore.setState({ status: 'stale' })
    })

    return () => {
      source.close()
    }
  }, [view])
}

// eventsURL is the stream's address for a view, with no query for the default.
function eventsURL(view: string | null): string {
  if (view === null) {
    return '/api/events'
  }

  return `/api/events?${new URLSearchParams({ view }).toString()}`
}
