import { useEffect, useRef, useState } from 'react'
import type { CreatedWorktree } from '@/api/generated/types.gen.ts'
import { ConfirmSwitch } from '@/features/repositories/ConfirmSwitch.tsx'
import { useSwitchTo } from '@/features/repositories/repositoriesApi.ts'
import { Button } from '@/lib/Button.tsx'
import type { Teller } from '@/lib/Outcome.tsx'
import { useFocusHandback } from '@/lib/focus.ts'
import { type AsyncState, useAsyncAction } from '@/lib/useAsyncAction.ts'
import { startWorkInWorktree } from './startWorkApi.ts'

interface StartInWorktreeProps {
  issueKey: string
  outcome: Teller
  onMade: (worktree: CreatedWorktree) => void
}

// StartInWorktreeButton starts work on a not-started issue in a new worktree
// beside the repository, leaving the checkout here as it is, and hands the
// worktree made to onMade, for the switch to it to be offered.
export function StartInWorktreeButton({ issueKey, outcome, onMade }: StartInWorktreeProps) {
  const start = useAsyncAction((fetch: boolean) => startWorkInWorktree(issueKey, fetch), {
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
        held={start.state === 'running'}
        onClick={() => {
          void start.run(true)
        }}
        className="self-start"
      >
        {start.state === 'running' ? 'Starting work…' : 'Start work in a new worktree'}
      </Button>
      <StartRefusal start={start} />
    </div>
  )
}

interface StartAttempt {
  state: AsyncState
  error: string
  code: string
  run: (fetch: boolean) => Promise<void>
}

// StartRefusal says why a start of work made nothing, and when it was the
// fetch of origin that failed, offers to branch from what you have instead,
// as the terminal's branch creator does.
export function StartRefusal({ start }: { start: StartAttempt }) {
  if (start.state !== 'error') {
    return null
  }

  return (
    <div className="flex flex-col gap-tight">
      <p role="alert" className="text-sm text-destructive">
        {start.error}
      </p>
      {start.code === 'fetch_failed' ? (
        <Button
          variant="secondary"
          onClick={() => {
            void start.run(false)
          }}
          className="self-start"
        >
          Branch from what you have
        </Button>
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
// it, through the same last look every switch takes. It takes the focus as it
// appears, so the offer is where the button was.
export function WorktreeMadeOffer({ issueKey, worktree, outcome }: WorktreeMadeOfferProps) {
  const heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    heading.current?.focus()
  }, [])

  return (
    <section
      aria-label={`Started ${issueKey} in ${worktree.shown}`}
      className="flex flex-col gap-item rounded-lg border border-border p-4"
    >
      <h3 ref={heading} tabIndex={-1} className="font-semibold focus-visible:outline-none">
        Started {issueKey} in <span className="font-mono">{worktree.shown}</span>
      </h3>
      <p className="text-sm text-muted-foreground">
        On <span className="font-mono">{worktree.branch}</span>. Switch to it to work there; every
        section is read again.
      </p>
      <SwitchAsked dir={worktree.dir} shown={worktree.shown} outcome={outcome}>
        Switch to it
      </SwitchAsked>
    </section>
  )
}

interface SwitchToWorktreeProps {
  dir: string
  shown: string
  outcome: Teller
}

// SwitchToWorktreeButton switches to the worktree that has an issue's branch
// checked out, once the switch is confirmed: git will not check that branch
// out here as well.
export function SwitchToWorktreeButton({ dir, shown, outcome }: SwitchToWorktreeProps) {
  return (
    <SwitchAsked
      dir={dir}
      shown={shown}
      outcome={outcome}
      label={`Switch to its worktree, ${shown}`}
    >
      Switch to its worktree
    </SwitchAsked>
  )
}

interface SwitchAskedProps extends SwitchToWorktreeProps {
  // label is the opener's accessible name, where its words alone are not.
  label?: string
  children: string
}

// SwitchAsked is a switch's opener, or the confirmation it opens, as the
// Repositories section asks before each switch; Cancel hands focus back.
function SwitchAsked({ dir, shown, outcome, label, children }: SwitchAskedProps) {
  const [asking, setAsking] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const switchTo = useSwitchTo()

  if (asking) {
    return (
      <ConfirmSwitch
        destination={{ dir, shown }}
        switchTo={switchTo}
        teller={outcome}
        onCancel={() => {
          handBack()
          setAsking(false)
        }}
      />
    )
  }

  return (
    <Button
      ref={opener}
      variant="secondary"
      aria-label={label}
      onClick={() => {
        setAsking(true)
      }}
      className="self-start"
    >
      {children}
    </Button>
  )
}
