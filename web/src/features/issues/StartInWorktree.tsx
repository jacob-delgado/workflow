import { useEffect, useRef } from 'react'
import type { CreatedWorktree } from '@/api/generated/types.gen.ts'
import { useSwitchTo } from '@/features/repositories/repositoriesApi.ts'
import { Button } from '@/lib/Button.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { startWorkInWorktree } from './startWorkApi.ts'

// useSwitchToWorktree switches the server to a worktree's directory, saying
// where once it has: every section is read again there.
function useSwitchToWorktree(dir: string, outcome: Teller) {
  const switchTo = useSwitchTo()

  return useAsyncAction(() => switchTo(dir), {
    fallback: 'The worktree could not be switched to.',
    done: (switched) => `Switched to ${switched.here.shown}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })
}

interface StartInWorktreeProps {
  issueKey: string
  outcome: Teller
  onMade: (worktree: CreatedWorktree) => void
}

// StartInWorktreeButton starts work on a not-started issue in a new worktree
// beside the repository, leaving the checkout here as it is, and hands the
// worktree made to onMade, for the switch to it to be offered.
export function StartInWorktreeButton({ issueKey, outcome, onMade }: StartInWorktreeProps) {
  const start = useAsyncAction(() => startWorkInWorktree(issueKey), {
    fallback: `No worktree was made for ${issueKey}. Try again, or make it from the terminal with ctrl+w.`,
    done: () => '',
    onStart: outcome.clear,
    onDone: (_, worktree) => {
      onMade(worktree)
    },
  })

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="secondary"
        disabled={start.state === 'running'}
        onClick={() => {
          void start.run()
        }}
        className="self-start"
      >
        {start.state === 'running' ? 'Making the worktree…' : 'Start in a new worktree'}
      </Button>
      {start.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {start.error}
        </p>
      ) : null}
    </div>
  )
}

interface WorktreeMadeOfferProps {
  issueKey: string
  worktree: CreatedWorktree
  outcome: Teller
}

// WorktreeMadeOffer says where the worktree was made and offers to switch to
// it. It takes the focus as it appears, so the offer is where the button was.
export function WorktreeMadeOffer({ issueKey, worktree, outcome }: WorktreeMadeOfferProps) {
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    heading.current?.focus()
  }, [])
  const go = useSwitchToWorktree(worktree.dir, outcome)

  return (
    <section
      aria-label={`Started ${issueKey} in ${worktree.shown}`}
      className="flex flex-col gap-item rounded-md border border-border p-4"
    >
      <h3 ref={heading} tabIndex={-1} className="font-semibold focus-visible:outline-none">
        Started {issueKey} in <span className="font-mono">{worktree.shown}</span>
      </h3>
      <p className="text-sm text-muted-foreground">
        On <span className="font-mono">{worktree.branch}</span>. Switch to it to work there; every
        section is read again.
      </p>
      {go.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {go.error}
        </p>
      ) : null}
      <div>
        <Button
          variant="primary"
          aria-disabled={go.state === 'running'}
          onClick={() => {
            if (go.state !== 'running') {
              void go.run()
            }
          }}
        >
          {go.state === 'running' ? 'Switching…' : 'Switch to it'}
        </Button>
      </div>
    </section>
  )
}

interface SwitchToWorktreeProps {
  dir: string
  shown: string
  outcome: Teller
}

// SwitchToWorktreeButton switches to the worktree that has an issue's branch
// checked out: git will not check that branch out here as well.
export function SwitchToWorktreeButton({ dir, shown, outcome }: SwitchToWorktreeProps) {
  const go = useSwitchToWorktree(dir, outcome)

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="secondary"
        disabled={go.state === 'running'}
        aria-label={`Switch to its worktree, ${shown}`}
        onClick={() => {
          void go.run()
        }}
        className="self-start"
      >
        {go.state === 'running' ? 'Switching…' : 'Switch to its worktree'}
      </Button>
      {go.state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {go.error}
        </p>
      ) : null}
    </div>
  )
}
