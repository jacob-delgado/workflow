import { useSnapshotStore, type StreamStatus as Status } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'

const meta: Record<Status, { label: string; dot: string }> = {
  connecting: { label: 'Connecting', dot: 'bg-warning' },
  live: { label: 'Live', dot: 'bg-success' },
  stale: { label: 'Reconnecting', dot: 'bg-destructive' },
}

export function StreamStatus() {
  const status = useSnapshotStore((state) => state.status)
  const { label, dot } = meta[status]

  return (
    <div role="status" className="flex items-center gap-2 text-xs text-muted-foreground">
      <span aria-hidden className={cn('size-2 rounded-full', dot)} />
      {label}
    </div>
  )
}
