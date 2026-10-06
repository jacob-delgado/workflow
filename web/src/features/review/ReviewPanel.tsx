import { useState } from 'react'
import { useForgeWords } from '@/api/health.ts'
import type {
  Branch,
  Ci,
  LinkedIssue,
  OpenedPullRequest,
  PullRequest,
  Review,
} from '@/api/generated/types.gen.ts'
import { useLiveSnapshot } from '@/api/snapshot.ts'
import { shownKey } from '@/features/issues/issuePlaces.ts'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { definitionList } from '@/lib/utils.ts'
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
    <BranchReview key={snapshot.branch.name} review={snapshot.review} branch={snapshot.branch} />
  )
}

// BranchReview is the checked-out branch's pull request, or the offer to open
// one, beneath what the last open answered.
function BranchReview({ review, branch }: { review: Review; branch: Branch }) {
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
        <PullRequestSummary
          pull={review.pull}
          ci={review.ci ?? null}
          issue={review.issue ?? null}
          branch={branch}
        />
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
// own mark, its title, state and, while it is open, its reviews — and its CI
// checks, which the server sends only for an open one.
function PullRequestSummary({
  pull,
  ci,
  issue,
  branch,
}: {
  pull: PullRequest
  ci: Ci | null
  issue: LinkedIssue | null
  branch: Branch
}) {
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
        <dl className={definitionList}>
          {issue ? <IssueRow issue={issue} /> : null}
          <dt className="text-muted-foreground">State</dt>
          <dd>{stateLabel(pull)}</dd>
          {pull.state === 'open' ? <ReviewRows pull={pull} /> : null}
        </dl>
        <PullActions pull={pull} ci={ci} branch={branch} />
      </section>

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
        {issue.url === '' ? (
          shown
        ) : (
          <a
            href={issue.url}
            target="_blank"
            rel="noreferrer"
            className="underline-offset-4 hover:underline"
          >
            {shown}
          </a>
        )}
      </dd>
    </>
  )
}
