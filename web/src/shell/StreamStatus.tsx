import { useSnapshotStore, type StreamStatus as Status } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'

const meta: Record<Status, { label: string; dot: string }> = {
  connecting: { label: 'Connecting', dot: 'bg-warning' },
  live: { label: 'Live', dot: 'bg-success' },
  reconnecting: { label: 'Reconnecting', dot: 'bg-destructive' },
  stale: { label: 'Out of date', dot: 'bg-destructive' },
}

// StreamStatus says how current the page is. Why it is out of date is read out
// with the label and shown on hover, rather than drawn beside it, so the header
// stays one short line.
export function StreamStatus() {
  const status = useSnapshotStore((state) => state.status)
  const reason = useSnapshotStore((state) => state.reason)
  const { label, dot } = meta[status]

  return (
    <div
      role="status"
      title={reason === '' ? undefined : reason}
      className="flex items-center gap-2 text-xs text-muted-foreground"
    >
      <span aria-hidden className={cn('size-2 rounded-full', dot)} />
      {label}
      {reason === '' ? null : <span className="sr-only">: {reason}</span>}
    </div>
  )
}
