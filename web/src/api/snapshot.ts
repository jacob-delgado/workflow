import { useEffect, useEffectEvent, useState } from 'react'
import { create } from 'zustand'
import { getHealth } from './generated/sdk.gen.ts'
import { zSeed, zSnapshot } from './generated/zod.gen.ts'
import type { IssuesPage, Snapshot } from './generated/types.gen.ts'
import { useSessionStore } from './session.ts'

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

// SeededIssues is a view's issues as the store last kept them, which the
// stream sends ahead of its first snapshot: the view they were sent for, by
// name, or null for the server's default, and the page.
interface SeededIssues {
  view: string | null
  page: IssuesPage
}

interface SnapshotState {
  snapshot: Snapshot | null
  // The view's kept issues the stream sent before its first snapshot, until a
  // snapshot takes their place; null when none was sent, or once one has.
  seed: SeededIssues | null
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
  seed: null,
  view: null,
  status: 'connecting',
  reason: '',
  receivedAt: 0,
}))

// nextFrame resolves once the stream holds a frame that shows the write the
// server numbered write: the first whose writes is at least it, read wholly
// after that write landed, the frame already on the page included. With no
// number, for an answer the server did not number, it resolves on the next
// frame the stream pushes. It resolves at once when there is no stream to
// agree with — no snapshot yet, or a stream gone stale or closed — and as soon
// as the stream goes either way meanwhile. A stream reconnecting is waited
// out, since a frame it pushes once it is back is the one that agrees, as
// after a directory switch, whose count goes on.
export function nextFrame(write?: number): Promise<void> {
  const { snapshot, status } = useSnapshotStore.getState()
  if (snapshot === null || endsTheWait(status) || shows(snapshot, write)) {
    return Promise.resolve()
  }

  return new Promise((resolve) => {
    const unsubscribe = useSnapshotStore.subscribe((state) => {
      if (endsTheWait(state.status) || agrees(state.snapshot, snapshot, write)) {
        unsubscribe()
        resolve()
      }
    })
  })
}

// shows reports a frame read after the write numbered write landed; with no
// number, none is known to.
function shows(frame: Snapshot, write: number | undefined): boolean {
  return write !== undefined && frame.writes >= write
}

// agrees reports a frame that ends the wait for the write numbered write,
// begun while before was on the page: one that shows it, or, with no number,
// any frame pushed since.
function agrees(frame: Snapshot | null, before: Snapshot, write: number | undefined): boolean {
  if (frame === null || frame === before) {
    return false
  }

  return write === undefined || shows(frame, write)
}

// endsTheWait reports a stream that will push no frame to wait for.
function endsTheWait(status: StreamStatus): boolean {
  return status === 'stale' || status === 'closed'
}

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
    useSnapshotStore.setState({ status: 'connecting', reason: '', seed: null })
    const source = new EventSource(eventsURL(view))

    source.addEventListener('open', () => {
      useSnapshotStore.setState({ status: 'live', reason: '' })
    })

    source.addEventListener('seed', (event) => {
      onSeed(view, event as MessageEvent<string>)
    })

    source.addEventListener('snapshot', (event) => {
      onSnapshot(view, event as MessageEvent<string>)
    })

    source.addEventListener('error', () => {
      onError(source, view, viewRefused)
    })

    return () => {
      source.close()
    }
  }, [view])
}

// onSnapshot keeps a snapshot the stream pushed for view, or drops one the
// page cannot read and marks the stream stale with why.
function onSnapshot(view: string | null, event: MessageEvent<string>): void {
  const parsed = zSnapshot.safeParse(parseJSON(event.data))
  if (parsed.success) {
    useSnapshotStore.setState({
      snapshot: parsed.data,
      seed: null,
      view,
      status: 'live',
      reason: '',
      receivedAt: Date.now(),
    })
  } else {
    useSnapshotStore.setState({ status: 'stale', reason: unreadable, seed: null })
  }
}

// onSeed keeps the view's kept issues the stream sent ahead of its first
// snapshot. A seed is not an update, so the stream's state and when the last
// snapshot landed stay as they were; one the page cannot read marks the
// stream stale, as a snapshot it cannot read does.
function onSeed(view: string | null, event: MessageEvent<string>): void {
  const parsed = zSeed.safeParse(parseJSON(event.data))
  if (parsed.success) {
    useSnapshotStore.setState({ seed: { view, page: parsed.data.issues } })
  } else {
    useSnapshotStore.setState({ status: 'stale', reason: unreadable, seed: null })
  }
}

// onError says how the stream for view stands once its connection failed:
// refused, given up, or reconnecting.
function onError(source: EventSource, view: string | null, viewRefused: () => void): void {
  // A closed source is one the browser will not retry: the server answered
  // with something other than a stream, as it does (404) for a view a
  // restart dropped from the configuration.
  if (view !== null && source.readyState === EventSource.CLOSED) {
    viewRefused()

    return
  }

  // The default stream has no view to fall back from: a closed one stays
  // closed until the page is loaded again. EventSource says nothing of why
  // it was refused; a read of the health does, and a refused session marks
  // itself (client.ts).
  if (source.readyState === EventSource.CLOSED) {
    void getHealth()
    useSnapshotStore.setState({ status: 'closed', reason: givenUp, seed: null })

    return
  }

  useSnapshotStore.setState({ status: 'reconnecting', reason: '' })
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

// eventsURL is the stream's address for a view, or the default for null,
// presenting the page's session in the query: an EventSource sends no header
// of its own, and the stream alone takes the session there.
function eventsURL(view: string | null): string {
  const query = new URLSearchParams()
  if (view !== null) {
    query.set('view', view)
  }

  const { token } = useSessionStore.getState()
  if (token !== '') {
    query.set('session', token)
  }

  const asked = query.toString()

  return asked === '' ? '/api/events' : `/api/events?${asked}`
}
