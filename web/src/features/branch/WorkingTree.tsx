import { useEffect, useRef, useState } from 'react'
import type { Change } from '@/api/generated/types.gen.ts'
import { useAsyncAction, type AsyncState } from '@/lib/useAsyncAction.ts'
import { cn } from '@/lib/utils.ts'
import { CommitForm } from './CommitForm.tsx'
import { stageEverything, stageFile, unstageFile } from './stagingApi.ts'

// WorkingTree is the changed files, each with the terminal's space — stage it,
// or unstage it once it is wholly staged — then its `a`, Stage all, and the
// commit form, which stays in place and says what it waits for until something
// is staged.
export function WorkingTree({
  changes,
  suggestedScope,
}: {
  changes: Change[]
  suggestedScope: string
}) {
  return (
    <section aria-labelledby="changes-heading" className="flex flex-col gap-3">
      <h3 id="changes-heading" className="text-sm font-semibold text-muted-foreground uppercase">
        Working tree
      </h3>
      {changes.length === 0 ? null : (
        <>
          <ul className="flex flex-col gap-1.5">
            {changes.map((change) => (
              <ChangeRow key={change.path} change={change} />
            ))}
          </ul>
          <StageAll anythingToStage={changes.some(offersStage)} />
        </>
      )}
      <CommitForm blocked={commitBlocker(changes)} suggestedScope={suggestedScope} />
    </section>
  )
}

// offersStage is the terminal's rule for space: a file is unstaged only once
// the index holds all of it, and staged otherwise — an edit, an untracked
// file, the rest of a partly staged one, or a conflict, which staging marks
// resolved. What it offers to stage is what Stage all stages.
function offersStage(change: Change): boolean {
  return !change.staged || change.has_unstaged
}

// commitBlocker says why there is nothing to commit yet, or null once
// something is staged.
function commitBlocker(changes: Change[]): string | null {
  if (changes.some((change) => change.staged)) {
    return null
  }

  return changes.length === 0
    ? 'Clean — nothing to commit.'
    : 'Nothing staged yet — stage a file above.'
}

// stagedTag is how much of a file the index holds, in words.
function stagedTag(change: Change): string {
  if (!change.staged) {
    return 'unstaged'
  }

  return change.has_unstaged ? 'partly staged' : 'staged'
}

// ChangeRow is one changed file with its stage or unstage button and a live
// line that says what the last press did. The row outlives the snapshot that
// shows the file moved — it is keyed by the path — so what it said stays, and
// says what was done then, not what the button offers now.
function ChangeRow({ change }: { change: Change }) {
  const stage = offersStage(change)
  const verb = stage ? 'Stage' : 'Unstage'
  const [said, setSaid] = useState('')
  const { state, error, run } = useAsyncAction(
    async () => {
      await (stage ? stageFile(change.path) : unstageFile(change.path))
      setSaid(`${stage ? 'Staged' : 'Unstaged'} ${change.path}.`)
    },
    { fallback: `${change.path} could not be ${stage ? 'staged' : 'unstaged'}.` },
  )
  const outcome = useFocusOnDone(state)

  return (
    <li className="flex flex-col gap-1 text-sm">
      <div className="flex items-center gap-3">
        <span className="w-20 shrink-0 text-muted-foreground">{change.kind}</span>
        <code className="flex-1">{change.path}</code>
        <span className={change.staged ? 'text-xs text-success' : 'text-xs text-muted-foreground'}>
          {stagedTag(change)}
        </span>
        <button
          type="button"
          aria-label={`${verb} ${change.path}`}
          disabled={state === 'running'}
          onClick={() => {
            void run()
          }}
          className={stagingButtonClass}
        >
          {verb}
        </button>
      </div>
      <Outcome ref={outcome} state={state} text={state === 'error' ? error : said} />
    </li>
  )
}

// StageAll stages every change the index does not hold yet, as the terminal's
// `a` does, and says how it went.
function StageAll({ anythingToStage }: { anythingToStage: boolean }) {
  const { state, error, run } = useAsyncAction(stageEverything, {
    fallback: 'The changes could not be staged.',
  })
  const outcome = useFocusOnDone(state)

  return (
    <div className="flex flex-col gap-1">
      <button
        type="button"
        disabled={!anythingToStage || state === 'running'}
        onClick={() => {
          void run()
        }}
        className={cn('self-start', stagingButtonClass)}
      >
        {state === 'running' ? 'Staging…' : 'Stage all'}
      </button>
      <Outcome
        ref={outcome}
        state={state}
        text={state === 'error' ? error : 'Staged every change.'}
      />
    </div>
  )
}

// useFocusOnDone hands focus to an outcome line once its write is done: the
// button is off while the write runs, which can drop its focus to the page.
// Focus the user has moved elsewhere since — into the commit message, say —
// stays where they put it.
function useFocusOnDone(state: AsyncState) {
  const outcome = useRef<HTMLParagraphElement>(null)

  useEffect(() => {
    if (state !== 'done') {
      return
    }
    const focused = document.activeElement
    const group = outcome.current?.parentElement
    if (focused === document.body || group?.contains(focused)) {
      outcome.current?.focus()
    }
  }, [state])

  return outcome
}

interface OutcomeProps {
  ref: React.Ref<HTMLParagraphElement>
  state: AsyncState
  text: string
}

// Outcome is a write's live line: what was done, or why not, once there is
// something to say.
function Outcome({ ref, state, text }: OutcomeProps) {
  const settled = state === 'done' || state === 'error'

  return (
    <p
      ref={ref}
      role="status"
      tabIndex={-1}
      className={cn('text-sm', state === 'error' ? 'text-destructive' : 'text-success')}
    >
      {settled ? text : ''}
    </p>
  )
}

const stagingButtonClass =
  'rounded-md border border-input px-2 py-1 text-xs hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60'
