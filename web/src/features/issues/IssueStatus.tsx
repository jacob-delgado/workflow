import type { StatusCategory } from '@/api/generated/types.gen.ts'
import { StateMark, type MarkState } from '@/shell/StateMark.tsx'

// The tracker groups every workflow's statuses into three categories, and the
// mark follows the category, as the interface's issue list does, so "In
// Progress" and "In Review" read alike without the cockpit knowing a given
// board's status names.
const mark: Record<StatusCategory, MarkState> = {
  new: 'not-started',
  indeterminate: 'in-flight',
  done: 'done',
}

// IssueStatus is an issue's status in the tracker's words, after the mark of
// its category.
export function IssueStatus({ category, label }: { category: StatusCategory; label: string }) {
  return (
    <span className="flex items-center gap-1 text-xs whitespace-nowrap text-muted-foreground">
      <StateMark state={mark[category]} className="size-3" />
      {label}
    </span>
  )
}
