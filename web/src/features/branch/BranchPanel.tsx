import { GitBranch } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { Branch, Change } from '@/api/generated/types.gen.ts'
import { useLiveSnapshot } from '@/api/snapshot.ts'
import { useHoldShortcuts, useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { CodeValue } from '@/lib/Meta.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { ReadFailure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure, definitionList } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { RunOutput, useGitRun, type GitRunner } from './GitRun.tsx'
import { HistoryActions, onFeatureBranch, RebaseAction } from './HistoryActions.tsx'
import { HookSetup } from './HookSetup.tsx'
import { IssueLink } from './IssueLink.tsx'
import { pushBranch } from './pushApi.ts'
import { WorkingTree } from './WorkingTree.tsx'

export function BranchPanel() {
  const snapshot = useLiveSnapshot()
  const runner = useGitRun(snapshot.run)

  const { branch, changes } = snapshot
  const unread = snapshot.problems ?? {}

  // A branch that could not be read is that failure, not the empty branch the
  // server left in its place, which would read as no repository.
  if (unread.branch) {
    return (
      <div className={cn('flex flex-col gap-section', contentMeasure)}>
        <ReadFailure unread="The branch could not be read" problem={unread.branch} />
      </div>
    )
  }

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
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      <BranchSummary branch={branch} runner={runner} />
      <RunOutput runner={runner} />
      <Commits
        branch={branch}
        changes={changes.changes}
        runner={runner}
        unmanagedHooks={snapshot.hooks_unmanaged}
      />
      <WorkingTree
        changes={changes.changes}
        unread={unread.changes ?? null}
        suggestedScope={snapshot.suggested_scope}
        convention={{ types: snapshot.commit_types, subjectLimit: snapshot.subject_limit }}
      />
    </div>
  )
}

// BranchSummary is the checked-out branch, with the push that publishes it and
// the line that says what the push did — which stays when the snapshot showing
// the branch published takes the push away. A detached HEAD has no branch to
// push, link or rebase, so it says where one is to be had instead.
function BranchSummary({ branch, runner }: { branch: Branch; runner: GitRunner }) {
  const outcome = useOutcome()
  const heading = branch.name === '' ? `Detached HEAD at ${branch.head.slice(0, 7)}` : branch.name
  // There is something to push on a branch of its own (not a detached HEAD, not
  // the base) that has no upstream on the remote its push goes to, or that is
  // ahead of the one it has: nothingToPush, the other way round. Commit count is
  // not used — an undiscoverable base can leave it unknowable even for an ahead
  // branch.
  const published = onPushRemote(branch)
  const canPush = onFeatureBranch(branch) && (!published || branch.ahead > 0)

  return (
    <section aria-labelledby="branch-heading" className="flex flex-col gap-group">
      <div className="flex items-center gap-2">
        <GitBranch aria-hidden className="size-4 text-muted-foreground" />
        <h2 id="branch-heading" className="font-mono text-lg">
          {heading}
        </h2>
      </div>
      {branch.detached ? (
        <p className="text-sm text-muted-foreground">
          To work on a branch, go to Issues, where Switch branch and Start work live.
        </p>
      ) : null}
      <dl className={definitionList}>
        <dt className="text-muted-foreground">Base</dt>
        <CodeValue value={branch.base} />
        <dt className="text-muted-foreground">Upstream</dt>
        <CodeValue value={branch.upstream} />
        <dt className="text-muted-foreground">Tracking</dt>
        <dd>
          {published
            ? `${String(branch.ahead)} ahead, ${String(branch.behind)} behind`
            : 'not pushed yet'}
        </dd>
      </dl>
      <div className="flex flex-wrap items-start gap-item">
        {canPush ? <PushButton branch={branch} outcome={outcome} /> : null}
        {branch.name === '' ? null : <IssueLink branch={branch} outcome={outcome} />}
        <RebaseAction branch={branch} runner={runner} />
      </div>
      <OutcomeLine said={outcome.said} />
    </section>
  )
}

// onPushRemote reports an upstream on the remote the branch's push goes to: the
// one its ahead and behind are counted against, and the one a push updates.
function onPushRemote(branch: Branch): boolean {
  return branch.upstream.startsWith(`${branch.push_remote}/`)
}

// Commits are the branch's commits since its base, and the runs that work on
// them and on what is staged.
function Commits({
  branch,
  changes,
  runner,
  unmanagedHooks,
}: {
  branch: Branch
  changes: Change[]
  runner: GitRunner
  unmanagedHooks: number
}) {
  const { commits } = branch
  const outcome = useOutcome()

  return (
    <section aria-labelledby="commits-heading" className="flex flex-col gap-group">
      <h3 id="commits-heading" className="text-base font-semibold">
        Commits
      </h3>
      {commits.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {branch.base === ''
            ? 'The base branch is unknown, so the commits since it cannot be listed.'
            : 'No commits yet on this branch.'}
        </p>
      ) : (
        <ul className="flex flex-col gap-item">
          {commits.map((commit) => (
            <li key={commit.hash} className="flex gap-item text-sm">
              <code className="text-muted-foreground">{commit.hash.slice(0, 7)}</code>
              <span>{commit.subject}</span>
            </li>
          ))}
        </ul>
      )}
      <HistoryActions branch={branch} changes={changes} runner={runner} />
      <HookSetup unmanaged={unmanagedHooks} outcome={outcome} />
      <OutcomeLine said={outcome.said} />
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
  const pushKeys = useShortcut('push', opener)
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

  // The confirm step is a box of its own, so it takes the row's whole width.
  return (
    <div className={cn('flex flex-col gap-item', confirming && 'basis-full')}>
      {confirming ? (
        <PushConfirm
          branch={branch}
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
        <Button
          variant="secondary"
          ref={opener}
          aria-keyshortcuts={pushKeys}
          held={push.state === 'running'}
          onClick={() => {
            setConfirming(true)
          }}
        >
          {push.state === 'running' ? 'Pushing…' : 'Push branch'}
        </Button>
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
  branch: Branch
  onCancel: () => void
  onPush: () => void
}

// PushConfirm asks before the push, naming what goes where as the terminal's
// last look does, and takes focus as it opens, so a screen reader hears the
// question. It counts no commits: the count since the base is not what the push
// sends, and without a base it is not known at all.
function PushConfirm({ branch, onCancel, onPush }: PushConfirmProps) {
  const question = useFocusOnMount<HTMLDivElement>()
  useHoldShortcuts()

  return (
    <div
      ref={question}
      role="group"
      aria-labelledby="push-question"
      tabIndex={-1}
      className="flex flex-wrap items-center gap-item text-sm"
    >
      <span id="push-question">
        Push <span className="font-mono">{branch.name}</span> to{' '}
        <span className="font-mono">{branch.push_remote}</span>?
      </span>
      {/* The buttons keep their words whole: a long branch name wraps them
          under the question rather than squeeze them. */}
      <span className="flex shrink-0 items-center gap-item">
        <Button variant="secondary" onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" onClick={onPush}>
          Push
        </Button>
      </span>
    </div>
  )
}
