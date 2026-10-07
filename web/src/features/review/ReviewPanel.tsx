import { useState } from 'react'
import { useForgeWords } from '@/api/health.ts'
import type {
  Branch,
  LinkedIssue,
  OpenedPullRequest,
  Problem,
  PullRequest,
  Review,
} from '@/api/generated/types.gen.ts'
import { useLiveSnapshot } from '@/api/snapshot.ts'
import { shownKey } from '@/features/issues/issuePlaces.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { NewTabLink } from '@/lib/NewTabLink.tsx'
import { ReadFailure } from '@/lib/Status.tsx'
import { cn, contentMeasure, definitionList } from '@/lib/utils.ts'
import { CiChecks } from './Checks.tsx'
import { OpenedOutcome } from './OpenedOutcome.tsx'
import { OpenPullRequest } from './OpenPullRequest.tsx'
import { PullActions } from './PullActions.tsx'

const mergeableLabel: Record<'unknown' | 'clean' | 'conflicts', string> = {
  unknown: 'Mergeability unknown',
  clean: 'No conflicts',
  conflicts: 'Has conflicts',
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
// own mark, its title, state and, while it is open, its reviews — and its CI
// checks, which the server sends only for an open one, or why they could not
// be read.
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
  const ci = review.ci ?? null
  const ciUnread = review.ci_error ?? null
  const issue = review.issue ?? null

  return (
    <>
      <section aria-labelledby="pr-heading" className="flex flex-col gap-group">
        <h2 id="pr-heading" className="flex items-baseline gap-2 text-lg">
          <span className="text-muted-foreground">
            {sigil}
            {pull.number}
          </span>
          <NewTabLink href={pull.url}>{pull.title}</NewTabLink>
        </h2>
        <dl className={definitionList}>
          {issue ? <IssueRow issue={issue} /> : null}
          <dt className="text-muted-foreground">State</dt>
          <dd>{stateLabel(pull)}</dd>
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
      <dd>{mergeableLabel[pull.mergeable]}</dd>
      <dt className="text-muted-foreground">Approvals</dt>
      <dd>{pull.approvals}</dd>
      <dt className="text-muted-foreground">Changes requested</dt>
      <dd>{pull.changes_requested ? 'Yes' : 'No'}</dd>
    </>
  )
}

// IssueRow names the issue the pull request is for, by its key as its tracker
// writes it, linked to its page when the tracker gives one.
function IssueRow({ issue }: { issue: LinkedIssue }) {
  const shown = shownKey(issue)

  return (
    <>
      <dt className="text-muted-foreground">Issue</dt>
      <dd className="font-mono">
        {issue.url === '' ? shown : <NewTabLink href={issue.url}>{shown}</NewTabLink>}
      </dd>
    </>
  )
}
