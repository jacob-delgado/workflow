import { useSnapshotStore, type StreamStatus as Status } from '@/api/snapshot.ts'
import { StateMark, type MarkState } from './StateMark.tsx'

// Each state of the stream, by its words and its mark: nothing yet while it
// first connects, done while live, in flight while it finds its way back, and
// failed once a frame it cannot read has left the page out of date.
const meta: Record<Status, { label: string; mark: MarkState }> = {
  connecting: { label: 'Connecting', mark: 'not-started' },
  live: { label: 'Live', mark: 'done' },
  reconnecting: { label: 'Reconnecting', mark: 'in-flight' },
  stale: { label: 'Out of date', mark: 'failed' },
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
