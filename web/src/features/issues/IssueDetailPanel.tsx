import { ExternalLink } from 'lucide-react'
import { useEffect, useRef, useState, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useHealthStore } from '@/api/health.ts'
import type { Comment, Issue, IssueDetail } from '@/api/generated/types.gen.ts'
import { IssueTasks } from '@/features/tasks/IssueTasks.tsx'
import { Button } from '@/lib/Button.tsx'
import { definitionList } from '@/lib/utils.ts'
import { useIssue } from './issueApi.ts'
import { IssueStatus } from './IssueStatus.tsx'
import { WorkStory } from './WorkStory.tsx'
import { CommentComposer } from './CommentComposer.tsx'
import { CommentBody } from './wiki/WikiText.tsx'

const sectionHeading = 'text-base font-semibold'

// IssueDetailPanel shows one issue in full, read on its own from the tracker.
// The heading follows the list's row while the list holds the issue — the
// stream keeps that row current, where the full read is taken once — and falls
// back to the full read otherwise; the people, the link, the description and
// the comments wait for that read, and a read that fails offers to be tried
// again. The work story and the issue's tasks read the stream, so neither
// waits.
export function IssueDetailPanel({ issueKey, listed }: { issueKey: string; listed?: Issue }) {
  const { data, error, errorUpdateCount, isPending, isFetching, refetch } = useIssue(issueKey)
  // A Retry goes once the issue it reads again arrives, so its focus follows to
  // the issue's heading rather than falling to the page — once, so a later read
  // of the issue leaves focus wherever the user has put it since.
  const retried = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)
  const refusal = useShownRefusal(error, isPending)

  useEffect(() => {
    if (retried.current && data && !error) {
      retried.current = false
      heading.current?.focus()
    }
  }, [data, error])

  return (
    <article aria-labelledby="issue-detail-heading" className="flex flex-col gap-block">
      <IssueHeading issueKey={issueKey} issue={listed ?? data} heading={heading} />
      {isPending && refusal === null ? (
        <p className="text-sm text-muted-foreground">Reading {issueKey}…</p>
      ) : null}
      {refusal === null ? null : (
        <IssueUnread
          reason={apiErrorMessage(
            refusal,
            `${issueKey} could not be read. Press Retry to try again.`,
          )}
          refusals={errorUpdateCount}
          retrying={isFetching}
          onRetry={() => {
            retried.current = true
            void refetch()
          }}
        />
      )}
      {data ? <IssuePeople detail={data} /> : null}
      <section aria-labelledby="work-story-heading" className="flex flex-col gap-group">
        <h3 id="work-story-heading" className={sectionHeading}>
          Work story
        </h3>
        <WorkStory issueKey={issueKey} />
      </section>
      <IssueTasks issueKey={issueKey} />
      {data ? <Description text={data.description} /> : null}
      {data ? <Comments detail={data} /> : null}
    </article>
  )
}

// useShownRefusal is the refusal the issue's reads last met. Reading again an
// issue that was never read clears the error its last read met, so the last
// refusal is kept and shown until the read answers: the busy Retry beside it
// keeps its place, and the focus it was pressed with, rather than giving way
// to "Reading…".
function useShownRefusal<Refusal>(error: Refusal | null, isPending: boolean): Refusal | null {
  const [lastRefusal, setLastRefusal] = useState<Refusal | null>(null)
  if (error !== null && error !== lastRefusal) {
    setLastRefusal(error)
  }

  return error ?? (isPending ? lastRefusal : null)
}

// IssueUnread says why the issue could not be read, beside a Retry that reads
// it again: selecting the issue that is already selected reads nothing. While
// it reads, the Retry is marked busy rather than disabled, so it keeps the
// focus it was pressed with through another refusal, and a press while busy
// starts nothing. Each refusal is told in an alert of its own, keyed by how
// many there have been: an alert that keeps its words is not spoken again, and
// the Retry, outside it, keeps its place.
function IssueUnread({
  reason,
  refusals,
  retrying,
  onRetry,
}: {
  reason: string
  refusals: number
  retrying: boolean
  onRetry: () => void
}) {
  return (
    <div className="flex flex-col items-start gap-item">
      <p key={refusals} role="alert" className="text-sm text-destructive">
        {reason}
      </p>
      <Button
        variant="secondary"
        aria-disabled={retrying}
        onClick={() => {
          if (!retrying) {
            onRetry()
          }
        }}
      >
        {retrying ? 'Retrying…' : 'Retry'}
      </Button>
    </div>
  )
}

interface IssueHeadingProps {
  issueKey: string
  issue?: Issue
  heading: RefObject<HTMLHeadingElement | null>
}

// IssueHeading names the issue. Its heading takes focus from a Retry that goes
// once the issue is read, so it can hold focus without being a tab stop.
function IssueHeading({ issueKey, issue, heading }: IssueHeadingProps) {
  return (
    <div className="flex flex-col gap-tight">
      <span className="flex items-center gap-item">
        <span className="text-sm text-muted-foreground tabular-nums">{issueKey}</span>
        {issue ? <IssueStatus category={issue.status_category} label={issue.status} /> : null}
      </span>
      <h2 ref={heading} id="issue-detail-heading" tabIndex={-1} className="text-lg">
        {issue ? issue.summary : issueKey}
      </h2>
      {issue ? (
        <p className="text-sm text-muted-foreground">
          {issue.type}
          {issue.priority ? ` · ${issue.priority} priority` : ''}
        </p>
      ) : null}
    </div>
  )
}

function IssuePeople({ detail }: { detail: IssueDetail }) {
  const tracker = useTrackerName(detail.tracker)

  return (
    <div className="flex flex-col gap-group">
      <dl className={definitionList}>
        <dt className="text-muted-foreground">Reporter</dt>
        <dd>{detail.reporter === '' ? '—' : detail.reporter}</dd>
        <dt className="text-muted-foreground">Assignee</dt>
        <dd>{detail.assignee ?? 'Unassigned'}</dd>
      </dl>
      {detail.url === '' ? null : (
        <a
          href={detail.url}
          target="_blank"
          rel="noreferrer"
          className="flex items-center gap-1.5 self-start text-sm text-primary underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          Open in {tracker}
          <ExternalLink aria-hidden className="size-3.5" />
          <span className="sr-only"> (opens in a new tab)</span>
        </a>
      )}
    </div>
  )
}

function Description({ text }: { text: string }) {
  return (
    <section aria-labelledby="description-heading" className="flex flex-col gap-group">
      <h3 id="description-heading" className={sectionHeading}>
        Description
      </h3>
      {text === '' ? (
        <p className="text-sm text-muted-foreground">No description.</p>
      ) : (
        <p className="text-sm whitespace-pre-wrap">{text}</p>
      )}
    </section>
  )
}

// Comments is the issue's thread: the comments the tracker sent, oldest
// first, how many more it holds when it sent only some, and — on a Jira
// issue — a composer under it for the next one.
function Comments({ detail }: { detail: IssueDetail }) {
  const { comments, comment_total: total } = detail

  return (
    <section aria-labelledby="comments-heading" className="flex flex-col gap-group">
      <div className="flex items-baseline gap-item">
        <h3 id="comments-heading" className={sectionHeading}>
          Comments
        </h3>
        {total > 0 ? (
          <span className="rounded-sm bg-muted px-1.5 text-xs text-muted-foreground tabular-nums">
            {total}
          </span>
        ) : null}
      </div>
      {total === 0 ? <p className="text-sm text-muted-foreground">No comments yet.</p> : null}
      {comments.length > 0 ? (
        <ol aria-labelledby="comments-heading" className="flex flex-col gap-block">
          {comments.map((comment, index) => (
            <CommentItem key={`${String(index)}-${comment.created}`} comment={comment} />
          ))}
        </ol>
      ) : null}
      {total > comments.length ? (
        <p className="text-sm text-muted-foreground">
          Showing {comments.length} of {total} comments.
        </p>
      ) : null}
      {detail.tracker === 'jira' ? <CommentComposer issueKey={detail.key} /> : null}
    </section>
  )
}

// CommentItem is one comment: who wrote it, beside their initials, when, and
// what, drawn from Jira's wiki markup.
function CommentItem({ comment }: { comment: Comment }) {
  return (
    <li className="flex gap-group">
      <span
        aria-hidden
        className="grid size-8 shrink-0 place-items-center rounded-full bg-accent text-xs font-semibold text-accent-foreground"
      >
        {initialsOf(comment.author)}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-tight">
        <p className="flex flex-wrap items-baseline gap-x-item text-sm">
          <span className="font-medium text-foreground">{comment.author}</span>
          <WrittenAt created={comment.created} />
        </p>
        <div className="rounded-lg border border-border bg-card px-3 py-2 text-sm text-card-foreground">
          <CommentBody body={comment.body} markdown={false} />
        </div>
      </div>
    </li>
  )
}

// initialsOf is the first letters of a name's first and last words.
function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean)
  const first = words.at(0)?.at(0) ?? '?'
  const last = words.length > 1 ? (words.at(-1)?.at(0) ?? '') : ''

  return (first + last).toUpperCase()
}

// WrittenAt says when a comment was written: how long ago within the week,
// and the day after; the exact moment shows on hover. The zero time the server
// sends for a date the tracker could not give says nothing.
function WrittenAt({ created }: { created: string }) {
  const written = new Date(created)
  if (written.getUTCFullYear() <= 1) {
    return null
  }

  return (
    <time
      dateTime={created}
      title={written.toLocaleString(undefined, { dateStyle: 'full', timeStyle: 'short' })}
      className="text-xs text-muted-foreground"
    >
      {sinceWritten(written, new Date())}
    </time>
  )
}

const minute = 60_000
const hour = 60 * minute
const day = 24 * hour

// sinceWritten is how long ago written was, within the week, and its date
// after that.
function sinceWritten(written: Date, now: Date): string {
  const ago = now.getTime() - written.getTime()
  const relative = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

  if (ago < minute) {
    return 'just now'
  }

  if (ago < hour) {
    return relative.format(-Math.floor(ago / minute), 'minute')
  }

  if (ago < day) {
    return relative.format(-Math.floor(ago / hour), 'hour')
  }

  if (ago < 7 * day) {
    return relative.format(-Math.floor(ago / day), 'day')
  }

  return written.toLocaleDateString(undefined, { dateStyle: 'medium' })
}

// useTrackerName names where an issue lives, for its link: Jira, or the forge
// the repository is on — just "the forge" until the server's health says
// which. A merge request is GitLab's word for what GitHub calls a pull
// request.
function useTrackerName(tracker: IssueDetail['tracker']): string {
  const noun = useHealthStore((state) => state.health?.forge_noun)
  if (tracker === 'jira') {
    return 'Jira'
  }

  if (noun === undefined) {
    return 'the forge'
  }

  return noun === 'merge request' ? 'GitLab' : 'GitHub'
}
