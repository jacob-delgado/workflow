import { useState, type ReactNode } from 'react'
import { useForgeWords } from '@/api/health.ts'
import type {
  Branch,
  LinkedIssue,
  OpenedPullRequest,
  Problem,
  PullRequest,
  Review,
} from '@/api/generated/types.gen.ts'
import { useChangedByStream, useLiveSnapshot } from '@/api/snapshot.ts'
import { shownKey } from '@/features/issues/issuePlaces.ts'
import { CopyURL } from '@/lib/CopyURL.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { NewTabLink } from '@/lib/NewTabLink.tsx'
import { ReadFailure } from '@/lib/Status.tsx'
import { cn, contentMeasure, definitionList } from '@/lib/utils.ts'
import { StateMark, type MarkState } from '@/shell/StateMark.tsx'
import { CiChecks } from './Checks.tsx'
import { OpenedOutcome } from './OpenedOutcome.tsx'
import { OpenPullRequest } from './OpenPullRequest.tsx'
import { PullActions } from './PullActions.tsx'

// Whether the pull request merges cleanly, in words and by its mark: clean is
// done, a conflict failed, and a forge that has not worked it out yet has not
// started.
const mergeable: Record<PullRequest['mergeable'], { label: string; mark: MarkState }> = {
  unknown: { label: 'Mergeability unknown', mark: 'not-started' },
  clean: { label: 'No conflicts', mark: 'done' },
  conflicts: { label: 'Has conflicts', mark: 'failed' },
}

// How far the pull request has come, by its mark: open is in flight, whether a
// draft or ready; merged is done; closed unmerged never got there.
const stateMark: Record<PullRequest['state'], MarkState> = {
  open: 'in-flight',
  merged: 'done',
  closed: 'not-started',
}

export function ReviewPanel() {
  const snapshot = useLiveSnapshot()

  // Keyed by the branch, so checking out another starts its review afresh: an
  // open's outcome offers to write to its own branch's issue, and it goes
  // rather than offer a link the server would refuse — or, back on its branch,
  // a second one.
  return (
    <BranchReview
      key={snapshot.branch.name}
      review={snapshot.review}
      branch={snapshot.branch}
      unread={snapshot.problems?.review ?? null}
    />
  )
}

interface BranchReviewProps {
  review: Review
  branch: Branch
  // Why the forge could not be read for the branch, or null when it was.
  unread: Problem | null
}

// BranchReview is the checked-out branch's pull request, or the offer to open
// one, beneath what the last open answered. A forge that could not be read
// says so, above the pull request it last answered with, or in place of the
// offer: a forge that cannot say is not one with no pull request to show.
function BranchReview({ review, branch, unread }: BranchReviewProps) {
  // What the open answered, and the line that says so, held here — above the
  // switch between offering to open and showing the pull request — so the
  // snapshot that brings the new pull request back leaves the outcome and its
  // offers where they were. The offers are keyed by their pull request, so a
  // second open offers its own writes rather than inheriting what the first
  // one's did.
  const [opened, setOpened] = useState<OpenedPullRequest | null>(null)
  const outcome = useOutcome()
  const { noun } = useForgeWords()
  const pull = review.found ? (review.pull ?? null) : null

  return (
    <div className={cn('flex flex-col gap-section', contentMeasure)}>
      {/* The line sits close above the offers it introduces. */}
      <OutcomeLine said={outcome.said} className={opened === null ? undefined : '-mb-block'} />
      {opened === null ? null : <OpenedOutcome key={opened.pull.url} opened={opened} />}
      {unread === null ? null : (
        <ReadFailure
          unread={
            pull === null
              ? `The ${noun} could not be read`
              : `The ${noun} could not be read again; shown as last read`
          }
          notSetUp="The forge"
          problem={unread}
        />
      )}
      {pull === null ? null : <PullRequestSummary pull={pull} review={review} branch={branch} />}
      {pull === null && unread === null ? (
        <OpenPullRequest
          onOpened={(answered, said) => {
            setOpened(answered)
            outcome.say(said)
          }}
        />
      ) : null}
    </div>
  )
}

// PullRequestSummary is the branch's pull request — its number in the forge's
// own mark, its title and the control that copies its URL, which says so just
// under them; its state and, while it is open, its reviews — and its CI checks,
// which the server sends only for an open one, or why they could not be read.
function PullRequestSummary({
  pull,
  review,
  branch,
}: {
  pull: PullRequest
  review: Review
  branch: Branch
}) {
  const { sigil } = useForgeWords()
  const mark = `${sigil}${String(pull.number)}`
  const copied = useOutcome()
  const ci = review.ci ?? null
  const ciUnread = review.ci_error ?? null
  const issue = review.issue ?? null

  return (
    <>
      <section aria-labelledby="pr-heading" className="flex flex-col gap-group">
        <div className="flex flex-wrap items-center gap-x-group gap-y-item">
          <h2 id="pr-heading" className="flex items-baseline gap-2 text-lg">
            <span className="text-muted-foreground">{mark}</span>
            <NewTabLink href={pull.url}>{pull.title}</NewTabLink>
          </h2>
          <CopyURL url={pull.url} mark={mark} teller={copied} />
        </div>
        <OutcomeLine said={copied.said} className="-mt-tight" />
        <dl className={definitionList}>
          {issue ? <IssueRow issue={issue} /> : null}
          <dt className="text-muted-foreground">State</dt>
          <RowValue mark={stateMark[pull.state]}>{stateLabel(pull)}</RowValue>
          {pull.state === 'open' ? <ReviewRows pull={pull} /> : null}
        </dl>
        <PullActions pull={pull} ci={ci} branch={branch} />
      </section>

      {ciUnread === null ? null : <ReadFailure unread="CI could not be read" problem={ciUnread} />}
      {ci ? <CiChecks ci={ci} /> : null}
    </>
  )
}

// stateLabel is what the State row says: an open pull request is a draft or
// ready for review, and one that is no longer open says how it ended.
function stateLabel(pull: PullRequest): string {
  if (pull.state === 'open') {
    return pull.draft ? 'Draft' : 'Ready for review'
  }

  return endedLabel[pull.state]
}

const endedLabel: Record<Exclude<PullRequest['state'], 'open'>, string> = {
  merged: 'Merged',
  closed: 'Closed',
}

// ReviewRows are how an open pull request's review stands. One that is no
// longer open waits on no review, and the forge stops reporting it, so these
// rows would only show stale or unknown values.
function ReviewRows({ pull }: { pull: PullRequest }) {
  return (
    <>
      <dt className="text-muted-foreground">Mergeable</dt>
      <RowValue mark={mergeable[pull.mergeable].mark}>{mergeable[pull.mergeable].label}</RowValue>
      <dt className="text-muted-foreground">Approvals</dt>
      <RowValue>{String(pull.approvals)}</RowValue>
      <dt className="text-muted-foreground">Changes requested</dt>
      {pull.changes_requested ? <RowValue mark="failed">Yes</RowValue> : <RowValue>No</RowValue>}
    </>
  )
}

// RowValue is a row's value, after the mark of the state it is, when it is one:
// the mark carries the status light, so the word stays in the plain
// foreground. A value a snapshot changes plays the row's brief highlight.
function RowValue({ mark, children }: { mark?: MarkState; children: string }) {
  const { changed, settle } = useChangedByStream(children)

  return (
    <Value mark={mark} className={changed ? 'stream-changed' : undefined} onAnimationEnd={settle}>
      {children}
    </Value>
  )
}

// Value is a row's value cell, which always holds a mark's slot before its
// words: the state's mark, or an empty slot of the same size for a value that
// is no state, so every value in the list starts at one left edge.
function Value({
  mark,
  className,
  onAnimationEnd,
  children,
}: {
  mark?: MarkState
  className?: string
  onAnimationEnd?: () => void
  children: ReactNode
}) {
  return (
    <dd className={cn('flex items-center gap-tight', className)} onAnimationEnd={onAnimationEnd}>
      {mark === undefined ? (
        <span aria-hidden className="size-3.5 shrink-0" />
      ) : (
        <StateMark state={mark} />
      )}
      {children}
    </dd>
  )
}

// IssueRow names the issue the pull request is for, by its key as its tracker
// writes it, linked to its page when the tracker gives one.
function IssueRow({ issue }: { issue: LinkedIssue }) {
  const shown = shownKey(issue)

  return (
    <>
      <dt className="text-muted-foreground">Issue</dt>
      <Value className="font-mono">
        <NewTabLink href={issue.url}>{shown}</NewTabLink>
      </Value>
    </>
  )
}
