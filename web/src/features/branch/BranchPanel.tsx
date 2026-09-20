import { GitBranch } from 'lucide-react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { CommitForm } from './CommitForm.tsx'

export function BranchPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  if (!snapshot) {
    return <EmptyState>Connecting to the workspace…</EmptyState>
  }

  const { branch, changes } = snapshot

  // An empty name means either no repository or a detached HEAD — the latter
  // still points at a real commit, so only the former is "not a repository".
  if (branch.name === '' && !branch.detached) {
    return <EmptyState>This directory is not a Git repository.</EmptyState>
  }

  const heading = branch.name === '' ? `Detached HEAD at ${branch.head.slice(0, 7)}` : branch.name
  const anythingStaged = changes.changes.some((change) => change.staged)

  return (
    <div className="mt-4 flex max-w-2xl flex-col gap-8">
      <section aria-labelledby="branch-heading" className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <GitBranch aria-hidden className="size-4 text-muted-foreground" />
          <h2 id="branch-heading" className="font-mono text-lg font-medium">
            {heading}
          </h2>
        </div>
        <dl className="grid grid-cols-[6rem_1fr] gap-x-4 gap-y-1.5 text-sm">
          <dt className="text-muted-foreground">Base</dt>
          <dd className="font-mono">{branch.base === '' ? '—' : branch.base}</dd>
          <dt className="text-muted-foreground">Upstream</dt>
          <dd className="font-mono">{branch.upstream === '' ? 'none' : branch.upstream}</dd>
          <dt className="text-muted-foreground">Tracking</dt>
          <dd>
            {branch.ahead} ahead, {branch.behind} behind
          </dd>
        </dl>
      </section>

      <section aria-labelledby="commits-heading" className="flex flex-col gap-3">
        <h3 id="commits-heading" className="text-sm font-semibold text-muted-foreground uppercase">
          Commits
        </h3>
        {branch.commits.length === 0 ? (
          <p className="text-sm text-muted-foreground">No commits yet on this branch.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {branch.commits.map((commit) => (
              <li key={commit.hash} className="flex gap-3 text-sm">
                <code className="text-muted-foreground">{commit.hash.slice(0, 7)}</code>
                <span>{commit.subject}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="changes-heading" className="flex flex-col gap-3">
        <h3 id="changes-heading" className="text-sm font-semibold text-muted-foreground uppercase">
          Working tree
        </h3>
        {changes.changes.length === 0 ? (
          <p className="text-sm text-muted-foreground">Clean — nothing to commit.</p>
        ) : (
          <ul className="flex flex-col gap-1.5">
            {changes.changes.map((change) => (
              <li key={change.path} className="flex items-center gap-3 text-sm">
                <span className="w-20 shrink-0 text-muted-foreground">{change.kind}</span>
                <code className="flex-1">{change.path}</code>
                <span
                  className={
                    change.staged ? 'text-xs text-success' : 'text-xs text-muted-foreground'
                  }
                >
                  {change.staged ? 'staged' : 'unstaged'}
                </span>
              </li>
            ))}
          </ul>
        )}
        {anythingStaged ? <CommitForm /> : null}
      </section>
    </div>
  )
}
