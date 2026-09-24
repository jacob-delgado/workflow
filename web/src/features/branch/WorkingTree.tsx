import type { Change } from '@/api/generated/types.gen.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
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
    <section aria-labelledby="changes-heading" className="flex flex-col gap-group">
      <h3 id="changes-heading" className="text-base font-semibold">
        Working tree
      </h3>
      {changes.length === 0 ? null : (
        <>
          <ul className="flex flex-col gap-item">
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

// ChangeRow is one changed file with its stage or unstage button, the live
// line that says what the last press did, and why when it was refused. The row
// outlives the snapshot that shows the file moved — it is keyed by the path —
// so what it said stays, and says what was done then, not what the button
// offers now.
function ChangeRow({ change }: { change: Change }) {
  const stage = offersStage(change)
  const verb = stage ? 'Stage' : 'Unstage'
  const outcome = useOutcome()
  const { state, error, run } = useAsyncAction(
    () => (stage ? stageFile(change.path) : unstageFile(change.path)),
    {
      fallback: `${change.path} was not ${stage ? 'staged' : 'unstaged'}. Try again, or ${verb.toLowerCase()} it from a terminal to see why.`,
      done: () => `${stage ? 'Staged' : 'Unstaged'} ${change.path}.`,
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )

  return (
    <li className="flex flex-col gap-tight text-sm">
      <div className="flex items-center gap-item">
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
      <OutcomeLine said={outcome.said} />
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </li>
  )
}

// StageAll stages every change the index does not hold yet, as the terminal's
// `a` does, and says how it went.
function StageAll({ anythingToStage }: { anythingToStage: boolean }) {
  const outcome = useOutcome()
  const { state, error, run } = useAsyncAction(stageEverything, {
    fallback: 'Nothing was staged. Try again, or stage from a terminal to see why.',
    done: () => 'Staged every change.',
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col gap-tight">
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
      <OutcomeLine said={outcome.said} />
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}

const stagingButtonClass =
  'rounded-sm border border-input px-2 py-1 text-xs hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground'
