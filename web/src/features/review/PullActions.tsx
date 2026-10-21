import { useEffect, useRef, useState, type ReactNode } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useForgeWords } from '@/api/health.ts'
import type { Branch, Ci, MergeMethod, PullRequest } from '@/api/generated/types.gen.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { Input, TextArea } from '@/lib/Field.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { Failure, Reading } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { LabeledInput, WriteForm } from '@/lib/WriteForm.tsx'
import {
  editPull,
  finishBranch,
  mergePull,
  readMergeOffer,
  readPullText,
  rerunChecks,
} from './reviewWritesApi.ts'

type Opened = 'none' | 'edit' | 'merge' | 'finish' | 'rerun'

// Moved is the branch a finish left and the one it checked out.
export interface Moved {
  from: string
  to: string
}

// FinishTeller is the panel's line a finish says what it did in, above the
// branch's review: clear, as the finish starts, and say, with where it moved.
export interface FinishTeller {
  clear: () => void
  say: (text: string, moved: Moved) => void
}

// canMerge is loop.CanMerge: open, ready for review, free of conflicts,
// approved with no changes asked for, and its CI passed.
function canMerge(pull: PullRequest, ci: Ci | null): boolean {
  return (
    pull.state === 'open' &&
    !pull.draft &&
    pull.mergeable === 'clean' &&
    pull.approvals > 0 &&
    !pull.changes_requested &&
    ci?.state === 'passed'
  )
}

// canFinish is a merged pull request whose branch nothing is in the way of
// finishing. The server makes loop.RefuseFinish's check, which needs what the
// forge merged, and sends why when something is in the way.
function canFinish(pull: PullRequest, finishRefusal: string | undefined): boolean {
  return pull.state === 'merged' && finishRefusal === undefined
}

// PullActions are the writes on the branch's pull request the terminal's
// Review pane makes: edit it (e), merge it (M), finish its merged branch (F)
// and re-run its failed CI (R), each offered only when it can go, and each
// sent from a form that is its last look. One form is open at a time; closing
// it hands focus back to its button, and what a sent one did is said below.
// A merged branch that cannot be finished says why, as the terminal does; one
// finished says so in finished, the panel's line, since the finish checks out
// another branch, whose review starts afresh.
export function PullActions({
  pull,
  ci,
  branch,
  finishRefusal,
  finished,
}: {
  pull: PullRequest
  ci: Ci | null
  branch: Branch
  finishRefusal: string | undefined
  finished: FinishTeller
}) {
  const [opened, setOpened] = useState<Opened>('none')
  const outcome = useOutcome()
  const { noun, sigil } = useForgeWords()
  const buttons = useRef<Partial<Record<Opened, HTMLButtonElement | null>>>({})
  const returnTo = useRef<Opened>('none')
  const name = `${sigil}${String(pull.number)}`
  const closeForm = () => {
    setOpened('none')
  }
  const teller = closingIn(outcome, closeForm)
  const finishTeller = finishingIn(finished, outcome, closeForm)
  const close = () => {
    returnTo.current = opened
    setOpened('none')
  }

  // A form backed out of hands focus back to the button that opened it.
  useEffect(() => {
    if (opened === 'none' && returnTo.current !== 'none') {
      buttons.current[returnTo.current]?.focus()
      returnTo.current = 'none'
    }
  }, [opened])
  const offers: { opens: Exclude<Opened, 'none'>; label: string; can: boolean }[] = [
    { opens: 'edit', label: `Edit ${noun}`, can: pull.state === 'open' },
    { opens: 'merge', label: 'Merge', can: canMerge(pull, ci) },
    { opens: 'finish', label: 'Finish the branch', can: canFinish(pull, finishRefusal) },
    {
      opens: 'rerun',
      label: 'Re-run failed checks',
      can: pull.state === 'open' && ci?.state === 'failed',
    },
  ]
  const forms: Record<Exclude<Opened, 'none'>, ReactNode> = {
    edit: <EditForm name={name} teller={teller} onCancel={close} />,
    merge: <MergeForm name={name} teller={teller} onCancel={close} />,
    finish: <FinishForm name={name} branch={branch} teller={finishTeller} onCancel={close} />,
    rerun: <RerunForm name={name} title={pull.title} teller={teller} onCancel={close} />,
  }

  return (
    <div className="flex flex-col gap-item">
      <FinishRefusal reason={finishRefusal} />
      {opened === 'none' ? (
        <div role="group" aria-label={`Change ${name}`} className="flex flex-wrap gap-item">
          {offers
            .filter((offer) => offer.can)
            .map((offer) => (
              <OfferButton
                key={offer.opens}
                opens={offer.opens}
                onButton={(button) => {
                  buttons.current[offer.opens] = button
                }}
                onOpen={() => {
                  setOpened(offer.opens)
                }}
              >
                {offer.label}
              </OfferButton>
            ))}
        </div>
      ) : (
        forms[opened]
      )}
      <OutcomeLine said={outcome.said} />
    </div>
  )
}

// closingIn is a teller that says in line, closing the form that said it.
function closingIn(line: Teller, closeForm: () => void): Teller {
  return {
    clear: line.clear,
    say: (said) => {
      closeForm()
      line.say(said)
    },
  }
}

// finishingIn is the teller a finish says through: the panel's line, which
// outlives the review a finish starts afresh, cleared with this section's own
// as it starts, so it never stands beside what an earlier write said here.
function finishingIn(finished: FinishTeller, own: Teller, closeForm: () => void): FinishTeller {
  return {
    clear: () => {
      own.clear()
      finished.clear()
    },
    say: (said, moved) => {
      closeForm()
      finished.say(said, moved)
    },
  }
}

// FinishRefusal says why a merged branch cannot be finished, where the offer
// to finish it would be, as the terminal's Review pane does.
function FinishRefusal({ reason }: { reason: string | undefined }) {
  if (reason === undefined) {
    return null
  }

  return (
    <p className="text-sm text-muted-foreground">The branch cannot be finished yet: {reason}.</p>
  )
}

// The terminal's action each offer answers to. Edit answers none this page
// lists: the terminal's e edits in a preview as well as here.
const offerActions: Record<Exclude<Opened, 'none'>, string | undefined> = {
  edit: undefined,
  merge: 'merge',
  finish: 'finish-branch',
  rerun: 'rerun-checks',
}

interface OfferButtonProps {
  opens: Exclude<Opened, 'none'>
  onButton: (button: HTMLButtonElement | null) => void
  onOpen: () => void
  children: ReactNode
}

// OfferButton is one offer on the pull request, answering the terminal's key
// for it, and handing its button up so a form backed out of can focus it.
function OfferButton({ opens, onButton, onOpen, children }: OfferButtonProps) {
  const button = useRef<HTMLButtonElement>(null)
  const shortcut = useShortcut(offerActions[opens], button)

  return (
    <Button
      variant="secondary"
      aria-keyshortcuts={shortcut}
      ref={(drawn) => {
        button.current = drawn
        onButton(drawn)
      }}
      onClick={onOpen}
    >
      {children}
    </Button>
  )
}

interface FormProps {
  name: string
  teller: Teller
  onCancel: () => void
}

// useReadOnOpen reads what a form starts from once, as it opens, and says
// why when it could not.
function useReadOnOpen<T>(read: () => Promise<T>, fallback: string) {
  const [state, setState] = useState<{ value?: T; error: string }>({ error: '' })

  useEffect(() => {
    let current = true
    read().then(
      (value) => {
        if (current) {
          setState({ value, error: '' })
        }
      },
      (caught: unknown) => {
        if (current) {
          setState({ error: apiErrorMessage(caught, fallback) })
        }
      },
    )

    return () => {
      current = false
    }
  }, [read, fallback])

  return state
}

// EditForm edits the title and description, read afresh as it opens.
function EditForm({ name, teller, onCancel }: FormProps) {
  const read = useReadOnOpen(readPullText, `${name} could not be read. Try again.`)

  if (read.error !== '') {
    return <Unopened error={read.error} onCancel={onCancel} />
  }

  if (read.value === undefined) {
    return <Reading>Reading {name}…</Reading>
  }

  return <EditFields name={name} start={read.value} teller={teller} onCancel={onCancel} />
}

// EditFields are the title and description as they will be saved.
function EditFields({
  name,
  start,
  teller,
  onCancel,
}: FormProps & { start: { title: string; body: string } }) {
  const [title, setTitle] = useState(start.title)
  const [body, setBody] = useState(start.body)
  const save = useAsyncAction(editPull, {
    fallback: `${name} was not saved. Try again — your edits are still in the form.`,
    done: () => `Saved ${name}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <WriteForm
      label={`Edit ${name}`}
      act="Save"
      busy={save.state === 'running' ? 'Saving…' : null}
      error={save.state === 'error' ? save.error : ''}
      onSend={() => void save.run(title, body)}
      onCancel={onCancel}
    >
      <LabeledInput label="Title">
        {(id) => (
          <Input
            id={id}
            value={title}
            required
            onChange={(event) => {
              setTitle(event.target.value)
            }}
          />
        )}
      </LabeledInput>
      <LabeledInput label="Description">
        {(id) => (
          <TextArea
            id={id}
            rows={6}
            value={body}
            onChange={(event) => {
              setBody(event.target.value)
            }}
          />
        )}
      </LabeledInput>
    </WriteForm>
  )
}

// methodLabel names a merge method as the forge's own button does.
const methodLabel: Record<MergeMethod, string> = {
  merge: 'Merge commit',
  squash: 'Squash and merge',
  rebase: 'Rebase and merge',
}

// MergeForm is the merge's preview: the methods the repository permits, read
// as it opens, one chosen, and the merge sent only from here.
function MergeForm({ name, teller, onCancel }: FormProps) {
  const read = useReadOnOpen(
    readMergeOffer,
    `How ${name} may be merged could not be read. Try again.`,
  )
  const [chosen, setChosen] = useState<MergeMethod | null>(null)
  const merge = useAsyncAction(mergePull, {
    fallback: `${name} was not merged. Try again, or merge it on the forge.`,
    done: () => `Merged ${name}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  if (read.error !== '') {
    return <Unopened error={read.error} onCancel={onCancel} />
  }

  if (read.value === undefined) {
    return <Reading>Reading how {name} may be merged…</Reading>
  }

  const method = chosen ?? read.value.methods[0] ?? 'merge'

  return (
    <WriteForm
      label={`Merge ${name}`}
      act="Merge"
      busy={merge.state === 'running' ? 'Merging…' : null}
      error={merge.state === 'error' ? merge.error : ''}
      onSend={() => void merge.run(method)}
      onCancel={onCancel}
    >
      <fieldset className="flex flex-col gap-tight">
        <legend className="mb-tight text-sm">
          Merge {name} {read.value.pull.title} by:
        </legend>
        {read.value.methods.map((offered) => (
          <label key={offered} className="flex items-center gap-2 text-sm">
            <input
              type="radio"
              name="merge-method"
              value={offered}
              checked={method === offered}
              onChange={() => {
                setChosen(offered)
              }}
              className="size-4"
            />
            {methodLabel[offered]}
          </label>
        ))}
      </fieldset>
    </WriteForm>
  )
}

// FinishForm is the finish's last look: the git commands it runs, as the
// server runs them.
function FinishForm({
  name,
  branch,
  teller,
  onCancel,
}: Omit<FormProps, 'teller'> & { branch: Branch; teller: FinishTeller }) {
  const finish = useAsyncAction(finishBranch, {
    fallback: `${branch.name} was not finished. Try again, or finish it with F in the terminal.`,
    done: (now) => `Finished ${branch.name}; now on ${now.name}.`,
    onStart: teller.clear,
    onDone: (said, now) => {
      teller.say(said, { from: branch.name, to: now.name })
    },
  })

  return (
    <WriteForm
      label={`Finish ${branch.name}`}
      act="Finish"
      busy={finish.state === 'running' ? 'Finishing…' : null}
      error={finish.state === 'error' ? finish.error : ''}
      onSend={() => void finish.run()}
      onCancel={onCancel}
    >
      <p className="text-sm">
        {name} merged; finish <span className="font-mono">{branch.name}</span> by running:
      </p>
      <pre className="text-xs [overflow-wrap:anywhere] whitespace-pre-wrap">
        {(branch.finish_commands ?? []).join('\n')}
      </pre>
    </WriteForm>
  )
}

// RerunForm is the re-run's last look: the pull request whose failed checks
// it restarts, a write to the forge.
function RerunForm({ name, title, teller, onCancel }: FormProps & { title: string }) {
  const rerun = useAsyncAction(rerunChecks, {
    fallback: `The checks on ${name} were not re-run. Try again, or re-run them on the forge.`,
    done: (answer) =>
      answer.reran
        ? `Re-ran the failed checks on ${name}.`
        : 'Nothing was re-run: this failure has no job to restart.',
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <WriteForm
      label={`Re-run the failed checks on ${name}`}
      act="Re-run"
      busy={rerun.state === 'running' ? 'Re-running…' : null}
      error={rerun.state === 'error' ? rerun.error : ''}
      onSend={() => void rerun.run()}
      onCancel={onCancel}
    >
      <p className="text-sm">
        Re-run the failed checks on {name} {title}?
      </p>
    </WriteForm>
  )
}

// Unopened is a form whose start could not be read: why, and the way back.
function Unopened({ error, onCancel }: { error: string; onCancel: () => void }) {
  return (
    <div className="flex flex-col items-start gap-item">
      <Failure>{error}</Failure>
      <Button variant="secondary" onClick={onCancel}>
        Cancel
      </Button>
    </div>
  )
}
