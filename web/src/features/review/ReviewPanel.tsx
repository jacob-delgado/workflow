import type { CiState } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'

const ciDot: Record<CiState, string> = {
  none: 'bg-muted-foreground',
  running: 'bg-warning',
  passed: 'bg-success',
  failed: 'bg-destructive',
}

const mergeableLabel: Record<'unknown' | 'clean' | 'conflicts', string> = {
  unknown: 'Mergeability unknown',
  clean: 'No conflicts',
  conflicts: 'Has conflicts',
}

export function ReviewPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  if (!snapshot) {
    return <EmptyState>Connecting to the forge…</EmptyState>
  }

  const { review } = snapshot

  if (!review.found || !review.pull) {
    return <EmptyState>No open pull request for this branch yet.</EmptyState>
  }

  const { pull, ci } = review

  return (
    <div className="mt-4 flex max-w-2xl flex-col gap-8">
      <section aria-labelledby="pr-heading" className="flex flex-col gap-3">
        <h2 id="pr-heading" className="flex items-baseline gap-2 text-lg font-medium">
          <span className="text-muted-foreground">#{pull.number}</span>
          <a
            href={pull.url}
            target="_blank"
            rel="noreferrer"
            className="underline-offset-4 hover:underline"
          >
            {pull.title}
          </a>
        </h2>
        <dl className="grid grid-cols-[9rem_1fr] gap-x-4 gap-y-1.5 text-sm">
          <dt className="text-muted-foreground">State</dt>
          <dd>{pull.draft ? 'Draft' : 'Ready for review'}</dd>
          <dt className="text-muted-foreground">Mergeable</dt>
          <dd>{mergeableLabel[pull.mergeable]}</dd>
          <dt className="text-muted-foreground">Approvals</dt>
          <dd>{pull.approvals}</dd>
          <dt className="text-muted-foreground">Changes requested</dt>
          <dd>{pull.changes_requested ? 'Yes' : 'No'}</dd>
        </dl>
      </section>

      {ci ? (
        <section aria-labelledby="ci-heading" className="flex flex-col gap-3">
          <h3 id="ci-heading" className="text-sm font-semibold text-muted-foreground uppercase">
            CI — {ci.done}/{ci.total} done{ci.failed > 0 ? `, ${String(ci.failed)} failed` : ''}
          </h3>
          <ul className="flex flex-col gap-1.5">
            {ci.checks.map((check) => (
              <li key={check.name} className="flex items-center gap-3 text-sm">
                <span
                  aria-hidden
                  className={cn('size-2 shrink-0 rounded-full', ciDot[check.state])}
                />
                {check.url === '' ? (
                  <span>{check.name}</span>
                ) : (
                  <a
                    href={check.url}
                    target="_blank"
                    rel="noreferrer"
                    className="underline-offset-4 hover:underline"
                  >
                    {check.name}
                  </a>
                )}
                <span className="text-muted-foreground">{check.state}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </div>
  )
}
