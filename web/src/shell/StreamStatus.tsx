import { useEffect, useState } from 'react'
import { useSnapshotStore, type StreamStatus as Status } from '@/api/snapshot.ts'
import { EmptyState } from './EmptyState.tsx'
import { narrowAgo, writtenMoment } from '@/lib/dates.ts'
import { StateMark, type MarkState } from './StateMark.tsx'

// Each state of the stream, by its words and its mark: nothing yet while it
// first connects, done while live, in flight while it finds its way back, and
// failed once a frame it cannot read has left the page out of date. wait is
// what a section waiting on its first update says meanwhile, so the section
// and the header never disagree.
const meta: Record<Status, { label: string; mark: MarkState; wait: string }> = {
  connecting: { label: 'Connecting', mark: 'not-started', wait: 'Connecting to workflow…' },
  live: { label: 'Live', mark: 'done', wait: 'Waiting for the first update…' },
  reconnecting: { label: 'Reconnecting', mark: 'in-flight', wait: 'Reconnecting to workflow…' },
  stale: {
    label: 'Out of date',
    mark: 'failed',
    wait: 'This page cannot read workflow’s updates. Reload the page.',
  },
}

// StreamStatus says how current the page is, and when the last snapshot
// landed. Why it is out of date is read out with the label and shown on hover,
// rather than drawn beside it, so the header stays one short line.
export function StreamStatus() {
  return (
    <div className="flex items-center gap-item">
      <StreamPill />
      <LastUpdate />
    </div>
  )
}

// StreamPill is the stream's state, live, so a reader hears it change.
function StreamPill() {
  const status = useSnapshotStore((state) => state.status)
  const reason = useSnapshotStore((state) => state.reason)
  const { label, mark } = meta[status]

  return (
    <div
      role="status"
      title={reason === '' ? undefined : reason}
      className="flex items-center gap-1.5 text-xs text-muted-foreground"
    >
      <StateMark state={mark} className="size-3" />
      {label}
      {reason === '' ? null : <span className="sr-only">: {reason}</span>}
    </div>
  )
}

// StreamWait is what a section that reads the stream shows until its first
// update lands, in the words of the state the header shows.
export function StreamWait() {
  const status = useSnapshotStore((state) => state.status)

  return <EmptyState>{meta[status].wait}</EmptyState>
}

// LastUpdate is how long ago the last snapshot landed, counted on each second.
// It stands outside the live region: a time that changes every second would
// otherwise be spoken every second. Before the first snapshot there is none to
// date.
function LastUpdate() {
  const receivedAt = useSnapshotStore((state) => state.receivedAt)
  const now = useSecondClock()
  if (receivedAt === 0) {
    return null
  }

  const landed = new Date(receivedAt)

  return (
    <time
      dateTime={landed.toISOString()}
      title={writtenMoment(landed)}
      className="text-xs text-muted-foreground tabular-nums"
    >
      Updated {narrowAgo(Math.max(0, now - receivedAt))}
    </time>
  )
}

// useSecondClock is the page's clock, in milliseconds, read again each second.
function useSecondClock(): number {
  const [now, setNow] = useState(Date.now)

  useEffect(() => {
    const tick = setInterval(() => {
      setNow(Date.now())
    }, 1_000)

    return () => {
      clearInterval(tick)
    }
  }, [])

  return now
}
