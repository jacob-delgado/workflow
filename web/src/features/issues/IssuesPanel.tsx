import type { Issue, TaskBranch } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { StatusBadge } from './StatusBadge.tsx'
import { useAsyncAction } from './useAsyncAction.ts'
import { WorkStory } from './WorkStory.tsx'

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

  const branchesByKey = groupBranchesByKey(snapshot.branches)
  const current = issues.find((issue) => issue.key === selected) ?? null

  return (
    <div className="mt-4 flex gap-6">
      <ul aria-label="Issues" className="flex w-80 shrink-0 flex-col gap-1">
        {issues.map((issue) => {
          // An issue can have more than one local branch; it is on HEAD when any
          // of them is, and check-out targets the most recent one (branches
          // arrive most-recently-committed first).
          const issueBranches = branchesByKey.get(issue.key) ?? []
          const newest = issueBranches[0]
          const onHead = issueBranches.some((branch) => branch.current)

          return (
            <li key={issue.key} className="flex flex-wrap items-center gap-1">
              <button
                type="button"
                aria-current={issue.key === selected ? true : undefined}
                onClick={() => {
                  selectIssue(issue.key)
                }}
                className={cn(
                  'flex flex-1 flex-col gap-1 rounded-md border border-transparent px-3 py-2 text-left transition-colors',
                  'hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                  issue.key === selected && 'border-border bg-accent',
                )}
              >
                <span className="flex items-center gap-2">
                  <span className="font-mono text-xs text-muted-foreground">{issue.key}</span>
                  <StatusBadge category={issue.status_category} label={issue.status} />
                  {newest ? (
                    <span className="ml-auto flex items-center gap-1 text-xs text-primary">
                      <span aria-hidden className="size-1.5 rounded-full bg-primary" />
                      <span className="sr-only">in flight</span>
                    </span>
                  ) : null}
                </span>
                <span className="text-sm">{issue.summary}</span>
              </button>
              {newest && !onHead ? <RowCheckout branch={newest.name} issueKey={issue.key} /> : null}
            </li>
          )
        })}
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
    <article aria-labelledby="issue-detail-heading" className="flex flex-col gap-6">
      <div className="flex flex-col gap-2">
        <span className="flex items-center gap-2">
          <span className="font-mono text-sm text-muted-foreground">{issue.key}</span>
          <StatusBadge category={issue.status_category} label={issue.status} />
        </span>
        <h2 id="issue-detail-heading" className="text-lg font-medium">
          {issue.summary}
        </h2>
        <p className="text-sm text-muted-foreground">
          {issue.type}
          {issue.priority ? ` · ${issue.priority} priority` : ''}
        </p>
      </div>
      <section aria-labelledby="work-story-heading" className="flex flex-col gap-4">
        <h3
          id="work-story-heading"
          className="text-sm font-semibold text-muted-foreground uppercase"
        >
          Work story
        </h3>
        <WorkStory issueKey={issue.key} />
      </section>
    </article>
  )
}

// groupBranchesByKey groups the local branches by the issue they name, keeping
// their order (most-recently-committed first) so the first is the most recent.
function groupBranchesByKey(branches: TaskBranch[]): Map<string, TaskBranch[]> {
  const byKey = new Map<string, TaskBranch[]>()
  for (const branch of branches) {
    const existing = byKey.get(branch.issue_key)
    if (existing) {
      existing.push(branch)
    } else {
      byKey.set(branch.issue_key, [branch])
    }
  }

  return byKey
}

// RowCheckout switches to an in-flight issue's branch from the list, so moving
// between tasks does not need the detail panel first. On success the event
// stream reflects the switch; a refusal — a dirty tree — is shown inline.
function RowCheckout({ branch, issueKey }: { branch: string; issueKey: string }) {
  const { state, error, run } = useAsyncAction(
    () => checkoutBranch(branch),
    'The branch could not be checked out.',
  )

  return (
    <>
      <button
        type="button"
        aria-label={`Check out ${issueKey}`}
        disabled={state === 'running'}
        onClick={() => {
          void run()
        }}
        className="shrink-0 rounded-md border border-input px-2 py-1 text-xs hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
      >
        {state === 'running' ? 'Switching…' : 'Check out'}
      </button>
      {state === 'error' ? (
        <p role="alert" className="basis-full text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </>
  )
}
