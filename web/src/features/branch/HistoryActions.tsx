import { useState } from 'react'
import type { Branch, Change, Commit } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { WriteForm } from '@/lib/WriteForm.tsx'
import type { GitRunner } from './GitRun.tsx'

// baseName is the base branch without its remote, main for origin/main.
export function baseName(base: string): string {
  const slash = base.indexOf('/')

  return slash < 0 ? base : base.slice(slash + 1)
}

// onFeatureBranch is loop.OnFeatureBranch: a branch of its own, not a detached
// HEAD and not the base itself.
export function onFeatureBranch(branch: Branch): boolean {
  return branch.name !== '' && branch.name !== baseName(branch.base)
}

// canRebase is loop.CanRebase: a branch of its own with a base to replay it
// onto.
function canRebase(branch: Branch): boolean {
  return onFeatureBranch(branch) && branch.base !== ''
}

// foldable is loop.Foldable: the commits not yet pushed, oldest first, while
// something is staged to fold into them.
function foldable(branch: Branch, changes: Change[]): Commit[] {
  if (!changes.some((change) => change.staged)) {
    return []
  }

  return branch.commits.filter((commit) => commit.unpushed)
}

// RebaseAction rebases the branch onto its base, as the terminal's u does,
// after a last look: a rebase rewrites the branch's history.
export function RebaseAction({ branch, runner }: { branch: Branch; runner: GitRunner }) {
  const [asking, setAsking] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const onto = baseName(branch.base)

  if (!canRebase(branch)) {
    return null
  }

  if (asking) {
    return (
      <div className="basis-full">
        <WriteForm
          label={`Rebase ${branch.name} onto ${onto}`}
          act="Rebase"
          busy={null}
          error=""
          onSend={() => {
            setAsking(false)
            runner.start({ kind: 'rebase' })
          }}
          onCancel={() => {
            handBack()
            setAsking(false)
          }}
        >
          <p className="text-sm">
            Replay <span className="font-mono">{branch.name}</span> onto{' '}
            <span className="font-mono">{branch.base}</span>? Its commits are rewritten; one already
            pushed then needs a force push. A conflict stops it midway, for a terminal to finish.
          </p>
        </WriteForm>
      </div>
    )
  }

  return (
    <Button
      variant="secondary"
      ref={opener}
      disabled={runner.going}
      onClick={() => {
        setAsking(true)
      }}
    >
      Rebase onto {onto}
    </Button>
  )
}

type Asked = 'none' | 'amend' | 'fixup'

// HistoryActions are the Commits section's runs, as the terminal's Commits
// pane offers them: Run pre-commit on what is staged (h), which changes
// nothing that leaves the machine and so runs at once; and, while something
// is staged and a commit is not yet pushed, Amend last commit (A) and Fix up
// a commit (f), each after a last look at the commit it rewrites.
export function HistoryActions({
  branch,
  changes,
  runner,
}: {
  branch: Branch
  changes: Change[]
  runner: GitRunner
}) {
  const [asked, setAsked] = useState<Asked>('none')
  const [amendButton, amendBack] = useFocusHandback<HTMLButtonElement>()
  const [fixupButton, fixupBack] = useFocusHandback<HTMLButtonElement>()
  const commits = foldable(branch, changes)
  const last = commits.at(-1)
  const cancel = () => {
    if (asked === 'amend') {
      amendBack()
    } else {
      fixupBack()
    }
    setAsked('none')
  }
  const look = {
    runner,
    onCancel: cancel,
    onSent: () => {
      setAsked('none')
    },
  }

  if (asked === 'amend' && last !== undefined) {
    return <AmendLook last={last} {...look} />
  }

  if (asked === 'fixup' && commits.length > 0) {
    return <FixupLook commits={commits} {...look} />
  }

  return (
    <div role="group" aria-label="Commit runs" className="flex flex-wrap gap-item">
      <Button
        variant="secondary"
        disabled={runner.going}
        onClick={() => {
          runner.start({ kind: 'pre_commit' })
        }}
      >
        Run pre-commit
      </Button>
      {last === undefined ? null : (
        <>
          <Button
            variant="secondary"
            ref={amendButton}
            disabled={runner.going}
            onClick={() => {
              setAsked('amend')
            }}
          >
            Amend last commit
          </Button>
          <Button
            variant="secondary"
            ref={fixupButton}
            disabled={runner.going}
            onClick={() => {
              setAsked('fixup')
            }}
          >
            Fix up a commit
          </Button>
        </>
      )}
    </div>
  )
}

interface LookProps {
  runner: GitRunner
  onCancel: () => void
  onSent: () => void
}

// AmendLook is the last look at an amend: the commit the staged changes go
// into, which is not pushed yet.
function AmendLook({ last, runner, onCancel, onSent }: LookProps & { last: Commit }) {
  return (
    <WriteForm
      label={`Amend ${last.subject}`}
      act="Amend"
      busy={null}
      error=""
      onSend={() => {
        onSent()
        runner.start({ kind: 'amend' })
      }}
      onCancel={onCancel}
    >
      <p className="text-sm">
        Fold the staged changes into <span className="font-mono">{last.hash.slice(0, 7)}</span>{' '}
        {last.subject}, keeping its message? It is not pushed yet, so only this machine&apos;s
        history changes.
      </p>
    </WriteForm>
  )
}

// FixupLook chooses the commit to record a fixup! of, newest first, among
// those not yet pushed; the choice is the last look.
function FixupLook({ commits, runner, onCancel, onSent }: LookProps & { commits: Commit[] }) {
  const newestFirst = [...commits].reverse()
  const [chosen, setChosen] = useState(newestFirst[0]?.hash ?? '')

  return (
    <WriteForm
      label="Fix up a commit"
      act="Fix up"
      busy={null}
      error=""
      disabled={chosen === ''}
      onSend={() => {
        onSent()
        runner.start({ kind: 'fixup', commit: chosen })
      }}
      onCancel={onCancel}
    >
      <fieldset className="flex flex-col gap-tight">
        <legend className="mb-tight text-sm">
          Record the staged changes as a fixup! of which commit? Only those not yet pushed are
          offered.
        </legend>
        {newestFirst.map((commit) => (
          <label key={commit.hash} className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="fixup-commit"
              value={commit.hash}
              checked={chosen === commit.hash}
              onChange={() => {
                setChosen(commit.hash)
              }}
              className="size-4"
            />
            <span className="font-mono text-muted-foreground">{commit.hash.slice(0, 7)}</span>{' '}
            {commit.subject}
          </label>
        ))}
      </fieldset>
    </WriteForm>
  )
}
