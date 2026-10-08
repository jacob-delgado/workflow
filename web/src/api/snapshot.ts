import { useEffect, useEffectEvent, useState } from 'react'
import { create } from 'zustand'
import { zSnapshot } from './generated/zod.gen.ts'
import type { Snapshot } from './generated/types.gen.ts'

// StreamStatus is how current the snapshot is: connecting before the first
// frame, live while frames land, reconnecting while the connection is down,
// stale while it is up but its frames cannot be read, and closed once the
// browser has given it up for good — the snapshot on screen is the last good
// one in each.
export type StreamStatus = 'connecting' | 'live' | 'reconnecting' | 'stale' | 'closed'

// unreadable is why a frame is dropped: a server newer or older than this page
// reads the snapshot differently, and a reload fetches the page it serves.
const unreadable = "the server's last update did not match what this page reads; reload the page"

// givenUp is why the default stream is closed: what answered at the page's
// address was no stream, and the browser does not ask again, so only a reload
// reaches the server there now.
const givenUp = 'workflow stopped sending this page updates; reload the page'

interface SnapshotState {
  snapshot: Snapshot | null
  // The issue view the snapshot was read for, by name, or null for the
  // server's default. While it differs from the chosen view a switch is under
  // way, and the snapshot's issues are still the last view's.
  view: string | null
  status: StreamStatus
  // Why the stream is stale, to show beside it; empty otherwise.
  reason: string
  // When the snapshot landed, by the page's clock in milliseconds — 0 before
  // the first — so a panel can tell what a write answered since from what the
  // stream pushed since.
  receivedAt: number
}

// The latest full read state the event stream has pushed, which view it was
// read for, and whether the stream is connected. The panels read from here;
// useEventStream is the only writer.
export const useSnapshotStore = create<SnapshotState>(() => ({
  snapshot: null,
  view: null,
  status: 'connecting',
  reason: '',
  receivedAt: 0,
}))

// useLiveSnapshot is the snapshot for a section that renders only once there is
// one: SectionPanel says it is connecting until the first frame lands, and no
// frame ever takes the snapshot back to null, so the section never checks for
// null itself. Rendering one before the first frame is a mistake in the shell,
// and throws rather than draw an empty section.
export function useLiveSnapshot(): Snapshot {
  const snapshot = useSnapshotStore((state) => state.snapshot)
  if (snapshot === null) {
    throw new Error('a section rendered before the first snapshot landed')
  }

  return snapshot
}

// useChangedByStream marks a row's value — a pull request's state, a check's,
// a file's stage — changed when a later snapshot changes it under the mounted
// row, so the row can play its brief highlight; settle, once that has played,
// takes the mark away. A value first drawn, as a section opens, is not a
// change. The highlight only points: what changed is said in words elsewhere,
// so the mark carries nothing a reader must see.
export function useChangedByStream(value: string): { changed: boolean; settle: () => void } {
  const [seen, setSeen] = useState(value)
  const [changed, setChanged] = useState(false)
  // Set while rendering, as React has state follow what it reads, so the frame
  // that changes the value is the one drawn highlighted.
  if (seen !== value) {
    setSeen(value)
    setChanged(true)
  }

  return {
    changed,
    settle: () => {
      setChanged(false)
    },
  }
}

// useEventStream opens the Server-Sent Events connection and keeps the snapshot
// store current from its pushes — the cockpit's freshness comes from the stream,
// never from polling. Mount it once, near the root. The stream carries the named
// issue view, or the server's default for null, and reconnects when the view
// changes. Each message is validated against the contract's schema; a payload
// that does not match is dropped rather than allowed to corrupt the last good
// snapshot, and the stream marked stale with why, so the page never looks live
// while it is not updating. A named view the server refuses outright is handed
// back through onViewRefused, since the browser never retries a refused stream.
export function useEventStream(view: string | null, onViewRefused: () => void): void {
  const viewRefused = useEffectEvent(onViewRefused)

  useEffect(() => {
    // `task web:mockup` sets VITE_MOCK so the whole cockpit can be navigated
    // against rich fixture data with no backend. The mock is code-split, so it
    // is never pulled into a production build.
    if (import.meta.env.VITE_MOCK === 'true') {
      void import('@/dev/mockSnapshot.ts').then((module) => {
        useSnapshotStore.setState({
          snapshot: module.mockSnapshot,
          view,
          status: 'live',
          receivedAt: Date.now(),
        })
      })

      return
    }

    useSnapshotStore.setState({ status: 'connecting', reason: '' })
    const source = new EventSource(eventsURL(view))

    source.addEventListener('open', () => {
      useSnapshotStore.setState({ status: 'live', reason: '' })
    })

    source.addEventListener('snapshot', (event) => {
      const parsed = zSnapshot.safeParse(parseJSON((event as MessageEvent<string>).data))
      if (parsed.success) {
        useSnapshotStore.setState({
          snapshot: parsed.data,
          view,
          status: 'live',
          reason: '',
          receivedAt: Date.now(),
        })
      } else {
        useSnapshotStore.setState({ status: 'stale', reason: unreadable })
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

      // The default stream has no view to fall back from: a closed one stays
      // closed until the page is loaded again.
      if (source.readyState === EventSource.CLOSED) {
        useSnapshotStore.setState({ status: 'closed', reason: givenUp })

        return
      }

      useSnapshotStore.setState({ status: 'reconnecting', reason: '' })
    })

    return () => {
      source.close()
    }
  }, [view])
}

// parseJSON reads a frame's JSON, or undefined for a frame that is not JSON —
// which the schema then refuses, like any other frame it cannot read.
function parseJSON(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return undefined
  }
}

// eventsURL is the stream's address for a view, with no query for the default.
function eventsURL(view: string | null): string {
  if (view === null) {
    return '/api/events'
  }

  return `/api/events?${new URLSearchParams({ view }).toString()}`
}
