import { ExternalLink } from 'lucide-react'
import { useEffect, useRef, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Comment, Issue, IssueDetail } from '@/api/generated/types.gen.ts'
import { definitionList } from '@/lib/utils.ts'
import { useIssue } from './issueApi.ts'
import { IssueStatus } from './IssueStatus.tsx'
import { WorkStory } from './WorkStory.tsx'

const sectionHeading = 'text-base font-semibold'

// IssueDetailPanel shows one issue in full, read on its own from the tracker.
// The heading follows the list's row while the list holds the issue — the
// stream keeps that row current, where the full read is taken once — and falls
// back to the full read otherwise; the people, the link, the description and
// the comments wait for that read, and a read that fails offers to be tried
// again. The work story reads the stream, so it never waits.
export function IssueDetailPanel({ issueKey, listed }: { issueKey: string; listed?: Issue }) {
  const { data, error, isPending, isFetching, refetch } = useIssue(issueKey)
  // A Retry goes once the issue it reads again arrives, so its focus follows to
  // the issue's heading rather than falling to the page — once, so a later read
  // of the issue leaves focus wherever the user has put it since.
  const retried = useRef(false)
  const heading = useRef<HTMLHeadingElement>(null)

  useEffect(() => {
    if (retried.current && data && !error) {
      retried.current = false
      heading.current?.focus()
    }
  }, [data, error])

  return (
    <article aria-labelledby="issue-detail-heading" className="flex flex-col gap-block">
      <IssueHeading issueKey={issueKey} issue={listed ?? data} heading={heading} />
      {isPending ? <p className="text-sm text-muted-foreground">Reading {issueKey}…</p> : null}
      {error ? (
        <IssueUnread
          reason={apiErrorMessage(
            error,
            `${issueKey} could not be read. Press Retry to try again.`,
          )}
          retrying={isFetching}
          onRetry={() => {
            retried.current = true
            void refetch()
          }}
        />
      ) : null}
      {data ? <IssuePeople detail={data} /> : null}
      <section aria-labelledby="work-story-heading" className="flex flex-col gap-group">
        <h3 id="work-story-heading" className={sectionHeading}>
          Work story
        </h3>
        <WorkStory issueKey={issueKey} />
      </section>
      {data ? <Description text={data.description} /> : null}
      {data ? <Comments comments={data.comments} total={data.comment_total} /> : null}
    </article>
  )
}

// IssueUnread says why the issue could not be read, beside a Retry that reads
// it again: selecting the issue that is already selected reads nothing.
function IssueUnread({
  reason,
  retrying,
  onRetry,
}: {
  reason: string
  retrying: boolean
  onRetry: () => void
}) {
  return (
    <div className="flex flex-col items-start gap-item">
      <p role="alert" className="text-sm text-destructive">
        {reason}
      </p>
      <button
        type="button"
        disabled={retrying}
        onClick={onRetry}
        className="rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
      >
        {retrying ? 'Retrying…' : 'Retry'}
      </button>
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
          Open in Jira
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

// Comments lists the comments the tracker sent, oldest first, and says how many
// more the issue holds when the tracker sent only some.
function Comments({ comments, total }: { comments: Comment[]; total: number }) {
  return (
    <section aria-labelledby="comments-heading" className="flex flex-col gap-group">
      <h3 id="comments-heading" className={sectionHeading}>
        Comments
      </h3>
      {total === 0 ? <p className="text-sm text-muted-foreground">No comments.</p> : null}
      {comments.length > 0 ? (
        <ol aria-labelledby="comments-heading" className="flex flex-col gap-group">
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
    </section>
  )
}

function CommentItem({ comment }: { comment: Comment }) {
  const written = commentDate(comment.created)

  return (
    <li className="flex flex-col gap-tight text-sm">
      <p className="text-muted-foreground">
        <span className="text-foreground">{comment.author}</span>
        {written === null ? null : (
          <>
            {' · '}
            <time dateTime={comment.created}>{written}</time>
          </>
        )}
      </p>
      <p className="whitespace-pre-wrap">{comment.body}</p>
    </li>
  )
}

// commentDate is when a comment was written, for reading, or null for the zero
// time the server sends when the tracker's date was unreadable.
function commentDate(created: string): string | null {
  const written = new Date(created)
  if (written.getUTCFullYear() <= 1) {
    return null
  }

  return written.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
