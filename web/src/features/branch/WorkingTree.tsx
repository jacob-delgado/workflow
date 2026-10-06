import { useState } from 'react'
import type { Change, FileDiff, Problem } from '@/api/generated/types.gen.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { Failure, ReadFailure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { CommitForm } from './CommitForm.tsx'
import {
  discardFile,
  readDiff,
  stageEverything,
  stageFile,
  unstageEverything,
  unstageFile,
} from './stagingApi.ts'

// WorkingTree is the changed files, each with the terminal's space — stage it,
// or unstage it once it is wholly staged — and its x, Discard…, which asks
// first; then its `a` and `U`, Stage all and Unstage all, and the commit form,
// which stays in place and says what it waits for until something is staged.
// What a discard did is said below the list, which outlives the file's row.
export function WorkingTree({
  changes,
  unread,
  suggestedScope,
  commitTypes,
}: {
  changes: Change[]
  // Why the changes could not be read, or null when they were.
  unread: Problem | null
  suggestedScope: string
  commitTypes: string[]
}) {
  const discards = useOutcome()

  // Changes that could not be read are that failure, not a clean tree.
  if (unread !== null) {
    return (
      <section aria-labelledby="changes-heading" className="flex flex-col gap-group">
        <WorkingTreeHeading />
        <ReadFailure unread="The working tree could not be read" problem={unread} />
      </section>
    )
  }

  return (
    <section aria-labelledby="changes-heading" className="flex flex-col gap-group">
      <WorkingTreeHeading />
      {changes.length === 0 ? null : (
        <>
          <ul className="flex flex-col gap-item">
            {changes.map((change) => (
              <ChangeRow key={change.path} change={change} discards={discards} />
            ))}
          </ul>
          <div className="flex flex-wrap items-start gap-item">
            <StageAll anythingToStage={changes.some(offersStage)} />
            <UnstageAll anythingStaged={changes.some((change) => change.staged)} />
          </div>
        </>
      )}
      <OutcomeLine said={discards.said} />
      <CommitForm
        blocked={commitBlocker(changes)}
        suggestedScope={suggestedScope}
        commitTypes={commitTypes}
      />
    </section>
  )
}

function WorkingTreeHeading() {
  return (
    <h3 id="changes-heading" className="text-base font-semibold">
      Working tree
    </h3>
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
function ChangeRow({ change, discards }: { change: Change; discards: Teller }) {
  const [asking, setAsking] = useState(false)
  const [discardOpener, handBack] = useFocusHandback<HTMLButtonElement>()
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
        <Button
          variant="secondary"
          size="sm"
          ref={discardOpener}
          aria-label={`Discard ${change.path}…`}
          aria-expanded={asking}
          onClick={() => {
            setAsking(true)
          }}
        >
          Discard…
        </Button>
      </div>
      {asking ? (
        <DiscardConfirm
          change={change}
          teller={discards}
          onClose={(discarded) => {
            if (!discarded) {
              handBack()
            }
            setAsking(false)
          }}
        />
      ) : null}
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
  const shortcut = useShortcutProps<HTMLButtonElement>('stage-all')

  return (
    <div className="flex flex-col gap-tight">
      <Button
        {...shortcut}
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

// UnstageAll takes every staged change out of the index, as the terminal's `U`
// does, at once: staging again undoes it. It says how it went.
function UnstageAll({ anythingStaged }: { anythingStaged: boolean }) {
  const outcome = useOutcome()
  const { state, error, run } = useAsyncAction(unstageEverything, {
    fallback: 'Nothing was unstaged. Try again, or unstage from a terminal to see why.',
    done: () => 'Unstaged every change.',
    onStart: outcome.clear,
    onDone: outcome.say,
  })
  const shortcut = useShortcutProps<HTMLButtonElement>('unstage-all')

  return (
    <div className="flex flex-col gap-tight">
      <Button
        {...shortcut}
        variant="secondary"
        disabled={!anythingStaged || state === 'running'}
        onClick={() => {
          void run()
        }}
        className="self-start"
      >
        {state === 'running' ? 'Unstaging…' : 'Unstage all'}
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

interface DiscardConfirmProps {
  change: Change
  teller: Teller
  onClose: (discarded: boolean) => void
}

// DiscardConfirm asks before a file's changes are dropped, since a discard
// cannot be undone — the terminal's last look on x — and takes the focus as it
// opens, so a screen reader hears the question.
function DiscardConfirm({ change, teller, onClose }: DiscardConfirmProps) {
  const question = useFocusOnMount<HTMLDivElement>()
  const discarding = useAsyncAction(() => discardFile(change.path), {
    fallback: `${change.path} was not discarded. Try again, or discard it from a terminal to see why.`,
    done: () => `Discarded ${change.path}.`,
    onStart: teller.clear,
    onDone: (said) => {
      teller.say(said)
      onClose(true)
    },
  })

  return (
    <div
      ref={question}
      role="group"
      aria-label={`Discard the changes to ${change.path}?`}
      tabIndex={-1}
      className="flex flex-col items-start gap-item rounded-lg border border-border p-3 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
    >
      <p>
        Discard the changes to <code>{change.path}</code>?
      </p>
      <p className="text-muted-foreground">{discardCost(change)}</p>
      {discarding.state === 'error' ? (
        <p role="alert" className="text-destructive">
          {discarding.error}
        </p>
      ) : null}
      <div className="flex items-center gap-item">
        <Button
          variant="secondary"
          disabled={discarding.state === 'running'}
          onClick={() => {
            onClose(false)
          }}
        >
          Cancel
        </Button>
        <Button
          variant="primary"
          disabled={discarding.state === 'running'}
          onClick={() => void discarding.run()}
        >
          {discarding.state === 'running' ? 'Discarding…' : 'Discard'}
        </Button>
      </div>
    </div>
  )
}

// discardCost says what a discard loses: a file the last commit does not hold
// is deleted, and any other goes back to how that commit has it.
function discardCost(change: Change): string {
  if (change.kind === 'untracked' || change.kind === 'new') {
    return 'The last commit does not have it, so the file is deleted. This cannot be undone.'
  }

  return 'Every change to it, staged and not, is lost: it goes back to the last commit. This cannot be undone.'
}
