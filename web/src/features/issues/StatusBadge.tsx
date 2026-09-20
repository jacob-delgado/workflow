import type { StatusCategory } from '@/api/generated/types.gen.ts'
import { cn } from '@/lib/utils.ts'

// The tracker groups every workflow's statuses into three categories; the badge
// colors by category so "In Progress" and "In Review" read alike without the
// cockpit knowing a given board's status names.
const tone: Record<StatusCategory, string> = {
  new: 'bg-muted text-muted-foreground',
  indeterminate: 'bg-warning/15 text-warning',
  done: 'bg-success/15 text-success',
}

export function StatusBadge({ category, label }: { category: StatusCategory; label: string }) {
  return (
    <span
      className={cn(
        'rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        tone[category],
      )}
    >
      {label}
    </span>
  )
}
