import { GitBranch } from 'lucide-react'
import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Branch } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { pushBranch } from './pushApi.ts'
import { WorkingTree } from './WorkingTree.tsx'

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

  return (
    <div className="mt-4 flex max-w-2xl flex-col gap-8">
      <BranchSummary branch={branch} />
      <Commits commits={branch.commits} />
      <WorkingTree changes={changes.changes} suggestedScope={snapshot.suggested_scope} />
    </div>
  )
}

function BranchSummary({ branch }: { branch: Branch }) {
  const heading = branch.name === '' ? `Detached HEAD at ${branch.head.slice(0, 7)}` : branch.name
  // There is something to push on a real branch (not a detached HEAD) that has no
  // upstream yet, or that is ahead of the one it has. Commit count is not used —
  // an undiscoverable base can leave it unknowable even for an ahead branch.
  const canPush = branch.name !== '' && (branch.upstream === '' || branch.ahead > 0)

  return (
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
      {canPush ? <PushButton branch={branch} /> : null}
    </section>
  )
}

function Commits({ commits }: { commits: Branch['commits'] }) {
  return (
    <section aria-labelledby="commits-heading" className="flex flex-col gap-3">
      <h3 id="commits-heading" className="text-sm font-semibold text-muted-foreground uppercase">
        Commits
      </h3>
      {commits.length === 0 ? (
        <p className="text-sm text-muted-foreground">No commits yet on this branch.</p>
      ) : (
        <ul className="flex flex-col gap-2">
          {commits.map((commit) => (
            <li key={commit.hash} className="flex gap-3 text-sm">
              <code className="text-muted-foreground">{commit.hash.slice(0, 7)}</code>
              <span>{commit.subject}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

type PushStatus = 'idle' | 'confirming' | 'pushing' | 'error'

// PushButton publishes the branch, behind a confirm step: pushing is outward and
// not undone with a click, so it asks first. The event stream reflects the
// published branch on success, and a failed push is shown inline.
function PushButton({ branch }: { branch: Branch }) {
  const [status, setStatus] = useState<PushStatus>('idle')
  const [error, setError] = useState('')

  const onPush = async () => {
    setStatus('pushing')
    try {
      await pushBranch()
      setError('')
      setStatus('idle')
    } catch (caught) {
      setError(apiErrorMessage(caught, 'The push failed.'))
      setStatus('error')
    }
  }

  return (
    <div className="flex flex-col gap-2">
      {status === 'confirming' ? (
        <div className="flex items-center gap-2 text-sm">
          <span>Push {branch.commits.length} commit(s) to the remote?</span>
          <button
            type="button"
            onClick={() => {
              setStatus('idle')
            }}
            className="rounded-md border border-input px-2 py-1 hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => {
              void onPush()
            }}
            className="rounded-md bg-primary px-2 py-1 text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
          >
            Push
          </button>
        </div>
      ) : (
        <button
          type="button"
          disabled={status === 'pushing'}
          onClick={() => {
            setStatus('confirming')
          }}
          className="self-start rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          {status === 'pushing' ? 'Pushing…' : 'Push branch'}
        </button>
      )}
      {status === 'error' ? (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
