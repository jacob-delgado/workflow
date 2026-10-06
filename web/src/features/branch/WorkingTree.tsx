import { useState } from 'react'
import type { Change, FileDiff } from '@/api/generated/types.gen.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Failure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { CommitForm } from './CommitForm.tsx'
import { readDiff, stageEverything, stageFile, unstageFile } from './stagingApi.ts'

// WorkingTree is the changed files, each with the terminal's space — stage it,
// or unstage it once it is wholly staged — then its `a`, Stage all, and the
// commit form, which stays in place and says what it waits for until something
// is staged.
export function WorkingTree({
  changes,
  suggestedScope,
  commitTypes,
}: {
  changes: Change[]
  suggestedScope: string
  commitTypes: string[]
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
      <CommitForm
        blocked={commitBlocker(changes)}
        suggestedScope={suggestedScope}
        commitTypes={commitTypes}
      />
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
  const busy = stage ? 'Staging…' : 'Unstaging…'
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
      <div className="flex flex-wrap items-center gap-x-item gap-y-tight">
        <span className="shrink-0 text-muted-foreground sm:w-20">{change.kind}</span>
        <code className="grow basis-full sm:basis-0">{change.path}</code>
        <span className={change.staged ? 'text-xs text-success' : 'text-xs text-muted-foreground'}>
          {stagedTag(change)}
        </span>
        <Button
          variant="secondary"
          size="sm"
          aria-label={`${state === 'running' ? busy : verb} ${change.path}`}
          disabled={state === 'running'}
          onClick={() => {
            void run()
          }}
        >
          {state === 'running' ? busy : verb}
        </Button>
      </div>
      <OutcomeLine said={outcome.said} />
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
      <ChangeDiff path={change.path} />
    </li>
  )
}

// ChangeDiff reads a changed file's diff when asked, never before, as the
// terminal shows the selected file's: the diff takes the focus as it opens,
// and Hide diff hands it back to the control that asked.
function ChangeDiff({ path }: { path: string }) {
  const [shown, setShown] = useState(false)
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const read = useAsyncAction(readDiff, {
    fallback: `The diff of ${path} could not be read. Try again, or run git diff from a terminal.`,
    onDone: () => {
      setShown(true)
    },
  })

  if (shown && read.result !== undefined) {
    return (
      <div className="flex flex-col gap-tight">
        <Button
          variant="secondary"
          size="sm"
          aria-label={`Hide diff of ${path}`}
          onClick={() => {
            handBack()
            setShown(false)
          }}
          className="self-start"
        >
          Hide diff
        </Button>
        <DiffText diff={read.result} />
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-tight">
      <Button
        variant="secondary"
        size="sm"
        ref={opener}
        aria-label={
          read.state === 'running' ? `Reading the diff of ${path}…` : `Show diff of ${path}`
        }
        aria-disabled={read.state === 'running'}
        onClick={() => {
          if (read.state !== 'running') {
            void read.run(path)
          }
        }}
        className="self-start"
      >
        {read.state === 'running' ? 'Reading the diff…' : 'Show diff'}
      </Button>
      {read.state === 'error' ? <Failure>{read.error}</Failure> : null}
    </div>
  )
}

// DiffText is a diff, scrolled by the keyboard once Tab reaches it. Each
// line keeps git's +, - or space, which carries what it is; color repeats it.
function DiffText({ diff }: { diff: FileDiff }) {
  const region = useFocusOnMount<HTMLPreElement>()

  return (
    <pre
      ref={region}
      role="region"
      aria-label={`Diff of ${diff.path}`}
      // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- a diff that scrolls must be reachable by Tab to be scrolled by keys (WCAG 2.1.1)
      tabIndex={0}
      className="max-h-80 overflow-auto rounded-md border border-border p-3 text-xs [overflow-wrap:anywhere] whitespace-pre-wrap focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      {diff.lines.length === 0
        ? 'git reports no difference.'
        : diff.lines.map((line, index) => (
            // A diff's lines repeat, so a line is known by where it is: the diff is
            // read once and never reordered.
            <span key={index} className={lineTone(line)}>
              {line}
              {'\n'}
            </span>
          ))}
    </pre>
  )
}

// lineTone colors an added line and a removed one, never their headers.
function lineTone(line: string): string | undefined {
  if (line.startsWith('+') && !line.startsWith('+++')) {
    return 'text-success'
  }

  if (line.startsWith('-') && !line.startsWith('---')) {
    return 'text-destructive'
  }

  return undefined
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
      <Button
        variant="secondary"
        disabled={!anythingToStage || state === 'running'}
        onClick={() => {
          void run()
        }}
        className="self-start"
      >
        {state === 'running' ? 'Staging…' : 'Stage all'}
      </Button>
      <OutcomeLine said={outcome.said} />
      {state === 'error' ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
