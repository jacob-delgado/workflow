import { useSnapshotStore, type StreamStatus as Status } from '@/api/snapshot.ts'
import { EmptyState } from './EmptyState.tsx'
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

// StreamStatus says how current the page is. Why it is out of date is read out
// with the label and shown on hover, rather than drawn beside it, so the header
// stays one short line.
export function StreamStatus() {
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
