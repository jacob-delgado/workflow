import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { StatusBadge } from './StatusBadge.tsx'

export function IssuesPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)
  const selected = useUiStore((state) => state.selectedIssue)
  const selectIssue = useUiStore((state) => state.selectIssue)

  if (!snapshot) {
    return <EmptyState>Connecting to the tracker…</EmptyState>
  }

  const issues = snapshot.issues.issues

  if (issues.length === 0) {
    return <EmptyState>No issues match this view.</EmptyState>
  }

  const current = issues.find((issue) => issue.key === selected) ?? null

  return (
    <div className="mt-4 flex gap-6">
      <ul aria-label="Issues" className="flex w-80 shrink-0 flex-col gap-1">
        {issues.map((issue) => (
          <li key={issue.key}>
            <button
              type="button"
              aria-current={issue.key === selected ? true : undefined}
              onClick={() => {
                selectIssue(issue.key)
              }}
              className={cn(
                'flex w-full flex-col gap-1 rounded-md border border-transparent px-3 py-2 text-left transition-colors',
                'hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                issue.key === selected && 'border-border bg-accent',
              )}
            >
              <span className="flex items-center gap-2">
                <span className="font-mono text-xs text-muted-foreground">{issue.key}</span>
                <StatusBadge category={issue.status_category} label={issue.status} />
              </span>
              <span className="text-sm">{issue.summary}</span>
            </button>
          </li>
        ))}
      </ul>

      <div className="flex-1">
        {current ? (
          <IssueDetail issue={current} />
        ) : (
          <EmptyState>Select an issue to see its detail.</EmptyState>
        )}
      </div>
    </div>
  )
}

function IssueDetail({ issue }: { issue: Issue }) {
  return (
    <article aria-labelledby="issue-detail-heading" className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <span className="flex items-center gap-2">
          <span className="font-mono text-sm text-muted-foreground">{issue.key}</span>
          <StatusBadge category={issue.status_category} label={issue.status} />
        </span>
        <h2 id="issue-detail-heading" className="text-lg font-medium">
          {issue.summary}
        </h2>
      </div>
      <dl className="grid grid-cols-[5rem_1fr] gap-x-4 gap-y-1.5 text-sm">
        <dt className="text-muted-foreground">Type</dt>
        <dd>{issue.type}</dd>
        <dt className="text-muted-foreground">Priority</dt>
        <dd>{issue.priority ?? '—'}</dd>
      </dl>
    </article>
  )
}
