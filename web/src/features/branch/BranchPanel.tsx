import { GitBranch } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { Branch } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { pushBranch } from './pushApi.ts'
import { WorkingTree } from './WorkingTree.tsx'

export function BranchPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  // The shell says it is connecting until the first snapshot lands.
  if (!snapshot) {
    return null
  }

  const { branch, changes } = snapshot

  // An empty name means either no repository or a detached HEAD — the latter
  // still points at a real commit, so only the former is "not a repository".
  if (branch.name === '' && !branch.detached) {
    return (
      <EmptyState>
        This directory is not a Git repository. Start workflow --web inside one to see its branch,
        commits and working tree here.
      </EmptyState>
    )
  }

  return (
    <div className="mt-4 flex max-w-2xl flex-col gap-8">
      <BranchSummary branch={branch} />
      <Commits commits={branch.commits} />
      <WorkingTree changes={changes.changes} suggestedScope={snapshot.suggested_scope} />
    </div>
  )
}

// BranchSummary is the checked-out branch, with the push that publishes it and
// the line that says what the push did — which stays when the snapshot showing
// the branch published takes the push away.
function BranchSummary({ branch }: { branch: Branch }) {
  const outcome = useOutcome()
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
      {canPush ? <PushButton branch={branch} outcome={outcome} /> : null}
      <OutcomeLine said={outcome.said} />
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

// PushButton publishes the branch, behind a confirm step: pushing is outward and
// not undone with a click, so it asks first. What it pushed is said in the
// panel's outcome, and a failed push is shown inline. Focus goes to the confirm
// as it opens, and back to the button on Cancel or a refusal.
function PushButton({ branch, outcome }: { branch: Branch; outcome: Teller }) {
  const [confirming, setConfirming] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const push = useAsyncAction(pushBranch, {
    fallback: 'The branch was not pushed. Try again, or push from a terminal to see why.',
    done: (published) => `Pushed ${published.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  // A refused push comes back to the button, beside its reason: the confirm that
  // had focus is gone. Focus the user has moved on to stays where they put it.
  useEffect(() => {
    if (push.state === 'error' && document.activeElement === document.body) {
      opener.current?.focus()
    }
  }, [push.state, opener])

  return (
    <div className="flex flex-col gap-2">
      {confirming ? (
        <PushConfirm
          commits={branch.commits.length}
          onCancel={() => {
            handBack()
            setConfirming(false)
          }}
          onPush={() => {
            setConfirming(false)
            void push.run()
          }}
        />
      ) : (
        <button
          ref={opener}
          type="button"
          disabled={push.state === 'running'}
          onClick={() => {
            setConfirming(true)
          }}
          className="self-start rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          {push.state === 'running' ? 'Pushing…' : 'Push branch'}
        </button>
      )}
      {push.state === 'error' && !confirming ? (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {push.error}
        </p>
      ) : null}
    </div>
  )
}

interface PushConfirmProps {
  commits: number
  onCancel: () => void
  onPush: () => void
}

// PushConfirm asks before the push, and takes focus as it opens, so a screen
// reader hears the question.
function PushConfirm({ commits, onCancel, onPush }: PushConfirmProps) {
  const question = useFocusOnMount<HTMLDivElement>()

  return (
    <div
      ref={question}
      role="group"
      aria-labelledby="push-question"
      tabIndex={-1}
      className="flex items-center gap-2 text-sm"
    >
      <span id="push-question">Push {commits} commit(s) to the remote?</span>
      <button
        type="button"
        onClick={onCancel}
        className="rounded-md border border-input px-2 py-1 hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        Cancel
      </button>
      <button
        type="button"
        onClick={onPush}
        className="rounded-md bg-primary px-2 py-1 text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
      >
        Push
      </button>
    </div>
  )
}
