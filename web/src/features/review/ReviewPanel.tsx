import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useForgeWords } from '@/api/health.ts'
import type {
  Ci,
  CiState,
  OpenedPullRequest,
  OpenPullRequestRequest,
  PullRequest,
  PullRequestDraft,
  Review,
} from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { cn } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { OpenedOutcome } from './OpenedOutcome.tsx'
import { openPr, previewPullRequest } from './openPrApi.ts'

const ciDot: Record<CiState, string> = {
  none: 'bg-muted-foreground',
  running: 'bg-warning',
  passed: 'bg-success',
  failed: 'bg-destructive',
}

const mergeableLabel: Record<'unknown' | 'clean' | 'conflicts', string> = {
  unknown: 'Mergeability unknown',
  clean: 'No conflicts',
  conflicts: 'Has conflicts',
}

export function ReviewPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  if (!snapshot) {
    return <EmptyState>Connecting to the forge…</EmptyState>
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
  // What the open answered, held here — above the switch between offering to
  // open and showing the pull request — so the snapshot that brings the new pull
  // request back leaves the outcome and its offers where they were. The outcome
  // is keyed by its pull request, so a second open offers its own writes rather
  // than inheriting what the first one's did.
  const [opened, setOpened] = useState<OpenedPullRequest | null>(null)

  return (
    <div className="mt-4 flex max-w-2xl flex-col gap-8">
      {opened === null ? null : <OpenedOutcome key={opened.pull.url} opened={opened} />}
      {review.found && review.pull ? (
        <PullRequestSummary pull={review.pull} ci={review.ci ?? null} />
      ) : (
        <OpenPullRequest onOpened={setOpened} />
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
      <section aria-labelledby="pr-heading" className="flex flex-col gap-3">
        <h2 id="pr-heading" className="flex items-baseline gap-2 text-lg font-medium">
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
        <dl className="grid grid-cols-[9rem_1fr] gap-x-4 gap-y-1.5 text-sm">
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
        <section aria-labelledby="ci-heading" className="flex flex-col gap-3">
          <h3 id="ci-heading" className="text-sm font-semibold text-muted-foreground uppercase">
            CI — {ci.done}/{ci.total} done{ci.failed > 0 ? `, ${String(ci.failed)} failed` : ''}
          </h3>
          <ul className="flex flex-col gap-1.5">
            {ci.checks.map((check) => (
              <li key={check.name} className="flex items-center gap-3 text-sm">
                <span
                  aria-hidden
                  className={cn('size-2 shrink-0 rounded-full', ciDot[check.state])}
                />
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

type OpenState = 'idle' | 'loading' | 'form' | 'opening' | 'done' | 'error'

// OpenPullRequest opens a pull request for a branch that has none yet, behind a
// preview: it composes the proposal, shows it as an editable form, and opens on
// confirm — pushing the branch first when it is not yet published. On success it
// hands what the open answered to the panel, which says so above it, and steps
// aside until the event stream brings back the new pull request.
function OpenPullRequest({ onOpened }: { onOpened: (opened: OpenedPullRequest) => void }) {
  const { noun } = useForgeWords()
  const [state, setState] = useState<OpenState>('idle')
  const [draft, setDraft] = useState<PullRequestDraft | null>(null)
  const [error, setError] = useState('')

  const openForm = async () => {
    setState('loading')
    try {
      setDraft(await previewPullRequest())
      setError('')
      setState('form')
    } catch (caught) {
      setError(apiErrorMessage(caught, `A ${noun} could not be composed.`))
      setState('error')
    }
  }

  const submit = async (request: OpenPullRequestRequest) => {
    setState('opening')
    try {
      onOpened(await openPr(request))
      setError('')
      setState('done')
    } catch (caught) {
      setError(apiErrorMessage(caught, `The ${noun} could not be opened.`))
      setState('error')
    }
  }

  if (state === 'done') {
    return null
  }

  // A failed open keeps the form up with its reason, so the edits are not lost;
  // a failed compose has no draft to edit, so it falls through to the retry
  // button below.
  if (draft && (state === 'form' || state === 'opening' || state === 'error')) {
    return (
      <PullRequestForm
        draft={draft}
        opening={state === 'opening'}
        error={state === 'error' ? error : ''}
        onCancel={() => {
          setState('idle')
        }}
        onSubmit={(request) => {
          void submit(request)
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-sm text-muted-foreground">No open {noun} for this branch yet.</p>
      <button
        type="button"
        disabled={state === 'loading'}
        onClick={() => {
          void openForm()
        }}
        className="self-start rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
      >
        {state === 'loading' ? 'Preparing…' : `Open a ${noun}`}
      </button>
      {state === 'error' ? (
        <p role="alert" className="text-sm whitespace-pre-line text-destructive">
          {error}
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

// splitList reads a comma-separated field into its trimmed, non-empty entries,
// so a stray comma never sends the forge a blank reviewer, assignee or label.
function splitList(text: string): string[] {
  return text
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry !== '')
}

// PullRequestForm is the composed pull request, editable, with a confirm that
// opens it and a cancel. It is disabled while the open is in flight, so a second
// click cannot open a second pull request.
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
  const { register, handleSubmit } = useForm<PullRequestFields>({
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

  const submit = handleSubmit((fields) => {
    onSubmit({
      title: fields.title,
      base: fields.base,
      body: fields.body,
      draft: fields.draft,
      reviewers: splitList(fields.reviewers),
      assignees: splitList(fields.assignees),
      labels: splitList(fields.labels),
    })
  })

  return (
    <form
      aria-label={`Open a ${noun}`}
      onSubmit={(event) => {
        void submit(event)
      }}
      className="flex flex-col gap-3 rounded-md border border-border p-4"
    >
      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Title
        <input {...register('title')} required className={prInputClass} />
      </label>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Base branch
        <input {...register('base')} required className={prInputClass} />
      </label>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Reviewers
        <input
          {...register('reviewers')}
          placeholder="comma-separated usernames"
          className={prInputClass}
        />
      </label>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Assignees
        <input
          {...register('assignees')}
          placeholder="comma-separated usernames"
          className={prInputClass}
        />
      </label>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Labels
        <input
          {...register('labels')}
          placeholder="comma-separated labels"
          className={prInputClass}
        />
      </label>

      <label className="flex flex-col gap-1 text-xs text-muted-foreground">
        Description
        <textarea {...register('body')} rows={6} className={prInputClass} />
      </label>

      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" {...register('draft')} className="size-4" />
        Open as a draft
      </label>

      {draft.needs_push ? (
        <p className="text-xs text-muted-foreground">
          The branch is not pushed yet; opening will push it first.
        </p>
      ) : null}

      <div className="flex items-center gap-2">
        <button
          type="button"
          disabled={opening}
          onClick={onCancel}
          className="rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={opening}
          className="rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-primary-foreground hover:bg-primary/90 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:opacity-60"
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

const prInputClass =
  'rounded-md border border-input bg-transparent px-3 py-2 text-sm text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'
