import { useEffect, useState } from 'react'
import { useForm, type UseFormRegister } from 'react-hook-form'
import { useForgeWords } from '@/api/health.ts'
import type {
  Ci,
  OpenedPullRequest,
  OpenPullRequestRequest,
  PullRequest,
  PullRequestDraft,
  Review,
} from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { useFocusHandback } from '@/lib/focus.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { splitList } from '@/lib/utils.ts'
import { ciMark, StateMark } from '@/shell/StateMark.tsx'
import { OpenedOutcome } from './OpenedOutcome.tsx'
import { openPr, previewPullRequest } from './openPrApi.ts'

const mergeableLabel: Record<'unknown' | 'clean' | 'conflicts', string> = {
  unknown: 'Mergeability unknown',
  clean: 'No conflicts',
  conflicts: 'Has conflicts',
}

export function ReviewPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  // The shell says it is connecting until the first snapshot lands.
  if (!snapshot) {
    return null
  }

  // Keyed by the branch, so checking out another starts its review afresh: an
  // open's outcome offers to write to its own branch's issue, and it goes
  // rather than offer a link the server would refuse — or, back on its branch,
  // a second one.
  return <BranchReview key={snapshot.branch.name} review={snapshot.review} />
}

// BranchReview is the checked-out branch's pull request, or the offer to open
// one, beneath what the last open answered.
function BranchReview({ review }: { review: Review }) {
  // What the open answered, and the line that says so, held here — above the
  // switch between offering to open and showing the pull request — so the
  // snapshot that brings the new pull request back leaves the outcome and its
  // offers where they were. The offers are keyed by their pull request, so a
  // second open offers its own writes rather than inheriting what the first
  // one's did.
  const [opened, setOpened] = useState<OpenedPullRequest | null>(null)
  const outcome = useOutcome()

  return (
    <div className="flex max-w-2xl flex-col gap-section">
      {/* The line sits close above the offers it introduces. */}
      <OutcomeLine said={outcome.said} className={opened === null ? undefined : '-mb-block'} />
      {opened === null ? null : <OpenedOutcome key={opened.pull.url} opened={opened} />}
      {review.found && review.pull ? (
        <PullRequestSummary pull={review.pull} ci={review.ci ?? null} />
      ) : (
        <OpenPullRequest
          onOpened={(answered, said) => {
            setOpened(answered)
            outcome.say(said)
          }}
        />
      )}
    </div>
  )
}

// PullRequestSummary is the branch's pull request — its number in the forge's
// own mark, its title, state and reviews — and its CI checks.
function PullRequestSummary({ pull, ci }: { pull: PullRequest; ci: Ci | null }) {
  const { sigil } = useForgeWords()

  return (
    <>
      <section aria-labelledby="pr-heading" className="flex flex-col gap-group">
        <h2 id="pr-heading" className="flex items-baseline gap-2 text-lg">
          <span className="text-muted-foreground">
            {sigil}
            {pull.number}
          </span>
          <a
            href={pull.url}
            target="_blank"
            rel="noreferrer"
            className="underline-offset-4 hover:underline"
          >
            {pull.title}
          </a>
        </h2>
        <dl className="grid grid-cols-[9rem_1fr] gap-x-group gap-y-tight text-sm">
          <dt className="text-muted-foreground">State</dt>
          <dd>{pull.draft ? 'Draft' : 'Ready for review'}</dd>
          <dt className="text-muted-foreground">Mergeable</dt>
          <dd>{mergeableLabel[pull.mergeable]}</dd>
          <dt className="text-muted-foreground">Approvals</dt>
          <dd>{pull.approvals}</dd>
          <dt className="text-muted-foreground">Changes requested</dt>
          <dd>{pull.changes_requested ? 'Yes' : 'No'}</dd>
        </dl>
      </section>

      {ci ? (
        <section aria-labelledby="ci-heading" className="flex flex-col gap-group">
          <h3 id="ci-heading" className="text-base font-semibold">
            CI checks{' '}
            <span className="font-normal text-muted-foreground">
              · {ci.done} of {ci.total} done
              {ci.failed > 0 ? `, ${String(ci.failed)} failed` : ''}
            </span>
          </h3>
          <ul className="flex flex-col gap-item">
            {ci.checks.map((check) => (
              <li key={check.name} className="flex items-center gap-item text-sm">
                <StateMark state={ciMark[check.state]} />
                {check.url === '' ? (
                  <span>{check.name}</span>
                ) : (
                  <a
                    href={check.url}
                    target="_blank"
                    rel="noreferrer"
                    className="underline-offset-4 hover:underline"
                  >
                    {check.name}
                  </a>
                )}
                <span className="text-muted-foreground">{check.state}</span>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </>
  )
}

// OpenPullRequest opens a pull request for a branch that has none yet, behind a
// preview: it composes the proposal, shows it as an editable form, and opens on
// confirm — pushing the branch first when it is not yet published. Composing and
// opening are two steps, each its own action. On success it hands what the open
// answered, and what to say of it, to the panel, which says so above it, and
// steps aside until the event stream brings back the new pull request.
function OpenPullRequest({
  onOpened,
}: {
  onOpened: (opened: OpenedPullRequest, said: string) => void
}) {
  const { noun, sigil } = useForgeWords()
  const [opener, handBack] = useFocusHandback<HTMLButtonElement>()
  const compose = useAsyncAction(previewPullRequest, {
    fallback: `The ${noun} could not be composed. Try again, or run workflow pr from a terminal.`,
  })
  const open = useAsyncAction(openPr, {
    fallback: `The ${noun} was not opened. Try again — your edits are still in the form.`,
    done: (opened) => `Opened ${noun} ${sigil}${String(opened.pull.number)}.`,
    onDone: (said, opened) => {
      onOpened(opened, said)
    },
  })

  if (open.state === 'done') {
    return null
  }

  // A failed open keeps the form up with its reason, so the edits are not lost;
  // a failed compose has no draft to edit, so it falls through to the retry
  // button below.
  if (compose.state === 'done' && compose.result !== undefined) {
    return (
      <PullRequestForm
        draft={compose.result}
        opening={open.state === 'running'}
        error={open.state === 'error' ? open.error : ''}
        onCancel={() => {
          handBack()
          open.reset()
          compose.reset()
        }}
        onSubmit={(request) => {
          void open.run(request)
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-item">
      <p className="text-sm text-muted-foreground">No open {noun} for this branch yet.</p>
      <button
        ref={opener}
        type="button"
        disabled={compose.state === 'running'}
        onClick={() => {
          void compose.run()
        }}
        className="self-start rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
      >
        {compose.state === 'running' ? 'Preparing…' : `Open a ${noun}`}
      </button>
      {compose.state === 'error' ? (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {compose.error}
        </p>
      ) : null}
    </div>
  )
}

interface PullRequestFields {
  title: string
  base: string
  body: string
  draft: boolean
  reviewers: string
  assignees: string
  labels: string
}

// PullRequestForm is the composed pull request, editable, opening on its title,
// with a confirm that opens it and a cancel. It is disabled while the open is in
// flight, so a second click cannot open a second pull request.
function PullRequestForm({
  draft,
  opening,
  error,
  onCancel,
  onSubmit,
}: {
  draft: PullRequestDraft
  opening: boolean
  error: string
  onCancel: () => void
  onSubmit: (request: OpenPullRequestRequest) => void
}) {
  const { noun } = useForgeWords()
  const { register, handleSubmit, setFocus } = useForm<PullRequestFields>({
    defaultValues: {
      title: draft.title,
      base: draft.base,
      body: draft.body,
      draft: draft.draft,
      reviewers: '',
      assignees: '',
      labels: '',
    },
  })

  // The form appears only because it was asked for, so focus goes with the
  // person asking to its first field.
  useEffect(() => {
    setFocus('title')
  }, [setFocus])

  const submit = handleSubmit((fields) => {
    onSubmit(proposal(fields))
  })

  return (
    <form
      aria-label={`Open a ${noun}`}
      onSubmit={(event) => {
        void submit(event)
      }}
      className="flex flex-col gap-group rounded-lg border border-border p-4"
    >
      <ProposalFields register={register} />

      {draft.needs_push ? (
        <p className="text-xs text-muted-foreground">
          The branch is not pushed yet; opening will push it first.
        </p>
      ) : null}

      <div className="flex items-center gap-item">
        <button
          type="button"
          disabled={opening}
          onClick={onCancel}
          className="rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={opening}
          className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
        >
          {opening ? 'Opening…' : `Open ${noun}`}
        </button>
      </div>

      {error === '' ? null : (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {error}
        </p>
      )}
    </form>
  )
}

// proposal is the request the form's fields make, each comma-separated list
// read into its trimmed entries.
function proposal(fields: PullRequestFields): OpenPullRequestRequest {
  return {
    title: fields.title,
    base: fields.base,
    body: fields.body,
    draft: fields.draft,
    reviewers: splitList(fields.reviewers),
    assignees: splitList(fields.assignees),
    labels: splitList(fields.labels),
  }
}

// ProposalFields are what a pull request opens with, each editable: its title
// and base, the people and labels it asks for, its description, and whether
// it opens as a draft.
function ProposalFields({ register }: { register: UseFormRegister<PullRequestFields> }) {
  return (
    <>
      <label className={labelClass}>
        Title
        <input {...register('title')} required className={prInputClass} />
      </label>

      <label className={labelClass}>
        Base branch
        <input {...register('base')} required className={prInputClass} />
      </label>

      <label className={labelClass}>
        Reviewers
        <input
          {...register('reviewers')}
          placeholder="comma-separated usernames"
          className={prInputClass}
        />
      </label>

      <label className={labelClass}>
        Assignees
        <input
          {...register('assignees')}
          placeholder="comma-separated usernames"
          className={prInputClass}
        />
      </label>

      <label className={labelClass}>
        Labels
        <input
          {...register('labels')}
          placeholder="comma-separated labels"
          className={prInputClass}
        />
      </label>

      <label className={labelClass}>
        Description
        <textarea {...register('body')} rows={6} className={prInputClass} />
      </label>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" {...register('draft')} className="size-4" />
        Open as a draft
      </label>
    </>
  )
}

const labelClass = 'flex flex-col gap-tight text-sm text-muted-foreground'

const prInputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
