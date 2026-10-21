import { useState } from 'react'
import type { Change, FileDiff, Problem } from '@/api/generated/types.gen.ts'
import { useChangedByStream } from '@/api/snapshot.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { useFocusHandback, useFocusOnMount } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { Failure, ReadFailure } from '@/lib/Status.tsx'
import { StateMark, type MarkState } from '@/lib/StateMark.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, keyedByText } from '@/lib/utils.ts'
import { LastLook } from '@/lib/LastLook.tsx'
import { CommitForm, type CommitConvention } from './CommitForm.tsx'
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
// which stays in place and says what it waits for until something is staged;
// a clean tree says so under the heading, before the form.
// What a discard did is said below the list, which outlives the file's row.
export function WorkingTree({
  changes,
  unread,
  suggestedScope,
  convention,
}: {
  changes: Change[]
  // Why the changes could not be read, or null when they were.
  unread: Problem | null
  suggestedScope: string
  convention: CommitConvention
}) {
  const discards = useOutcome()
  const anythingStaged = changes.some((change) => change.staged)

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
      {changes.length === 0 ? (
        <p className="text-sm text-muted-foreground">Clean — nothing to commit.</p>
      ) : (
        <>
          <ul className="flex flex-col gap-item">
            {changes.map((change) => (
              <ChangeRow key={change.path} change={change} discards={discards} />
            ))}
          </ul>
          <BulkStaging
            anythingToStage={changes.some(offersStage)}
            anythingStaged={anythingStaged}
          />
        </>
      )}
      <OutcomeLine said={discards.said} />
      <CommitForm
        canCommit={anythingStaged}
        waiting={
          changes.length > 0 && !anythingStaged
            ? 'Nothing staged yet — stage a file above.'
            : undefined
        }
        suggestedScope={suggestedScope}
        convention={convention}
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

// stagedTag is how much of a file the index holds, in words and by the mark
// the terminal's stageGlyph draws for it: a conflict, which no stage can
// half-hold, is failed until staging marks it resolved.
function stagedTag(change: Change): { word: string; mark: MarkState } {
  if (change.conflicted) {
    return { word: 'conflicted', mark: 'failed' }
  }

  if (!change.staged) {
    return { word: 'unstaged', mark: 'not-started' }
  }

  return change.has_unstaged
    ? { word: 'partly staged', mark: 'in-flight' }
    : { word: 'staged', mark: 'done' }
}

// ChangeRow is one changed file with its stage or unstage button, the live
// line that says what the last press did, and why when it was refused. The row
// outlives the snapshot that shows the file moved — it is keyed by the path —
// so what it said stays, and says what was done then, not what the button
// offers now; a snapshot that moves its stage plays the row's brief highlight.
function ChangeRow({ change, discards }: { change: Change; discards: Teller }) {
  const [asking, setAsking] = useState(false)
  const [discardOpener, handBack] = useFocusHandback<HTMLButtonElement>()
  const stage = offersStage(change)
  const verb = stage ? 'Stage' : 'Unstage'
  const busy = stage ? 'Staging…' : 'Unstaging…'
  const outcome = useOutcome()
  const staging = useChangedByStream(stagedTag(change).word)
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
    <li
      className={cn('flex flex-col gap-tight text-sm', staging.changed && 'stream-changed')}
      onAnimationEnd={staging.settle}
    >
      <div className="flex flex-wrap items-center gap-x-item gap-y-tight">
        <span className="shrink-0 text-muted-foreground sm:w-20">{change.kind}</span>
        <ChangePath change={change} />
        <StagedTag change={change} />
        <Button
          variant="secondary"
          size="sm"
          aria-label={`${state === 'running' ? busy : verb} ${change.path}`}
          held={state === 'running'}
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
      {state === 'error' ? <Failure>{error}</Failure> : null}
      <ChangeDiff path={change.path} />
    </li>
  )
}

// ChangePath is where a changed file is: for a rename or a copy, the path it
// had, then an arrow, then the path it has, as the terminal draws it. The arrow
// is read as "to", and is held to the new path by a no-break space, so a wrap
// leaves it leading the path it points at.
function ChangePath({ change }: { change: Change }) {
  const from = change.original_path ?? ''

  return (
    <code className="grow basis-full sm:basis-0">
      {from === '' ? null : (
        <>
          {from} <span aria-hidden>→</span>
          <span className="sr-only">to</span>
          {'\u00a0'}
        </>
      )}
      {change.path}
    </code>
  )
}

// StagedTag is how much of a file is staged: its mark carries the status
// light, so the word stays in the plain foreground.
function StagedTag({ change }: { change: Change }) {
  const { word, mark } = stagedTag(change)

  return (
    <span className="inline-flex items-center gap-tight text-xs">
      <StateMark state={mark} />
      {word}
    </span>
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
        held={read.state === 'running'}
        onClick={() => {
          void read.run(path)
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
        : keyedByText(diff.lines, (line) => line).map(({ key, item: line }) => (
            <span key={key} className={lineTone(line)}>
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

// BulkWords is what Stage all or Unstage all says: its name, its name while it
// runs, the line it ends on, its fallback, and the terminal's key it answers.
interface BulkWords {
  act: string
  acting: string
  done: string
  fallback: string
  shortcut: string
}

// useBulkWrite is Stage all's or Unstage all's write and the outcome it says.
function useBulkWrite(action: () => Promise<void>, words: BulkWords) {
  const outcome = useOutcome()
  const write = useAsyncAction(action, {
    fallback: words.fallback,
    done: () => words.done,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return { outcome, write, words }
}

type BulkWrite = ReturnType<typeof useBulkWrite>

// BulkStaging is Stage all, which stages every change the index does not hold
// yet as the terminal's `a` does, beside Unstage all, which takes every staged
// change out of the index at once as its `U` does, staging again undoing it.
// What each said goes below the pair, so a line wider than its button never
// pushes the other away.
function BulkStaging({
  anythingToStage,
  anythingStaged,
}: {
  anythingToStage: boolean
  anythingStaged: boolean
}) {
  const stageAll = useBulkWrite(stageEverything, {
    act: 'Stage all',
    acting: 'Staging…',
    done: 'Staged every change.',
    fallback: 'Nothing was staged. Try again, or stage from a terminal to see why.',
    shortcut: 'stage-all',
  })
  const unstageAll = useBulkWrite(unstageEverything, {
    act: 'Unstage all',
    acting: 'Unstaging…',
    done: 'Unstaged every change.',
    fallback: 'Nothing was unstaged. Try again, or unstage from a terminal to see why.',
    shortcut: 'unstage-all',
  })

  return (
    <div className="flex flex-col gap-tight">
      <div className="flex flex-wrap items-center gap-item">
        <BulkButton bulk={stageAll} offered={anythingToStage} />
        <BulkButton bulk={unstageAll} offered={anythingStaged} />
      </div>
      <BulkSaid bulk={stageAll} />
      <BulkSaid bulk={unstageAll} />
    </div>
  )
}

// BulkButton starts a bulk write, offered only while there is something for it
// to do, and held while it runs.
function BulkButton({ bulk, offered }: { bulk: BulkWrite; offered: boolean }) {
  const shortcut = useShortcutProps<HTMLButtonElement>(bulk.words.shortcut)
  const running = bulk.write.state === 'running'

  return (
    <Button
      {...shortcut}
      variant="secondary"
      disabled={!offered}
      held={running}
      onClick={() => {
        void bulk.write.run()
      }}
    >
      {running ? bulk.words.acting : bulk.words.act}
    </Button>
  )
}

// BulkSaid is how a bulk write went: what it did, or why it could not.
function BulkSaid({ bulk }: { bulk: BulkWrite }) {
  return (
    <>
      <OutcomeLine said={bulk.outcome.said} />
      {bulk.write.state === 'error' ? <Failure>{bulk.write.error}</Failure> : null}
    </>
  )
}

interface DiscardConfirmProps {
  change: Change
  teller: Teller
  onClose: (discarded: boolean) => void
}

// DiscardConfirm asks before a file's changes are dropped, since a discard
// cannot be undone — the terminal's last look on x — and drops them.
function DiscardConfirm({ change, teller, onClose }: DiscardConfirmProps) {
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
    <LastLook
      question={
        <>
          Discard the changes to <code>{change.path}</code>?
        </>
      }
      cost={discardCost(change)}
      act="Discard"
      acting="Discarding…"
      write={discarding}
      className="rounded-lg border border-border p-3"
      onAct={() => void discarding.run()}
      onCancel={() => {
        onClose(false)
      }}
    />
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
