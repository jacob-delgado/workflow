import { useEffect, useRef, useState, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import { useHealthStore } from '@/api/health.ts'
import type { Comment, Health, Issue, IssueDetail } from '@/api/generated/types.gen.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { IssueTasks } from '@/features/tasks/IssueTasks.tsx'
import { ago, day, writtenMoment } from '@/lib/dates.ts'
import { Meta } from '@/lib/Meta.tsx'
import { NewTabLink } from '@/lib/NewTabLink.tsx'
import { Reading, Unread } from '@/lib/Status.tsx'
import { definitionList } from '@/lib/utils.ts'
import { useIssue } from './issueApi.ts'
import { IssueStatus } from './IssueStatus.tsx'
import { WorkStory } from './WorkStory.tsx'
import { IssueActions } from './writes/IssueActions.tsx'
import { CommentComposer } from './CommentComposer.tsx'
import { CommentBody, largestParsedBody } from './wiki/WikiText.tsx'

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
  // A Try again goes once the issue it reads again arrives, so its focus follows to
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
      {isPending && refusal === null ? <Reading>Reading {issueKey}…</Reading> : null}
      {refusal === null ? null : (
        <Unread
          reason={apiErrorMessage(refusal, `${issueKey} could not be read. Press Try again.`)}
          refusals={errorUpdateCount}
          retrying={isFetching}
          onRetry={() => {
            retried.current = true
            void refetch()
          }}
        />
      )}
      {data ? <IssuePeople detail={data} /> : null}
      {data ? <IssueActions detail={data} /> : null}
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
// refusal is kept and shown until the read answers: the busy Try again beside it
// keeps its place, and the focus it was pressed with, rather than giving way
// to "Reading…".
function useShownRefusal<Refusal>(error: Refusal | null, isPending: boolean): Refusal | null {
  const [lastRefusal, setLastRefusal] = useState<Refusal | null>(null)
  if (error !== null && error !== lastRefusal) {
    setLastRefusal(error)
  }

  return error ?? (isPending ? lastRefusal : null)
}

interface IssueHeadingProps {
  issueKey: string
  issue?: Issue
  heading: RefObject<HTMLHeadingElement | null>
}

// IssueHeading names the issue. Its heading takes focus from a Try again that goes
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
        <Meta className="text-sm text-muted-foreground">
          {issue.type}
          {issue.priority ? `${issue.priority} priority` : null}
        </Meta>
      ) : null}
    </div>
  )
}

function IssuePeople({ detail }: { detail: IssueDetail }) {
  const tracker = useTrackerName(detail.tracker)
  const shortcut = useShortcutProps<HTMLAnchorElement>('open-link')

  return (
    <div className="flex flex-col gap-group">
      <dl className={definitionList}>
        <dt className="text-muted-foreground">Reporter</dt>
        <dd>{detail.reporter === '' ? '—' : detail.reporter}</dd>
        <dt className="text-muted-foreground">Assignee</dt>
        <dd>{detail.assignee ?? 'Unassigned'}</dd>
      </dl>
      {detail.url === '' ? null : (
        <NewTabLink {...shortcut} href={detail.url} className="self-start text-sm">
          Open in {tracker}
        </NewTabLink>
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
// first, how many more it holds when it sent only some, and a composer under
// it for the next one.
function Comments({ detail }: { detail: IssueDetail }) {
  const { comments, comment_total: total } = detail
  const unparsed = beyondParsing(comments)

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
            <CommentItem
              plain={unparsed[index] ?? true}
              key={`${String(index)}-${comment.created ?? ''}`}
              comment={comment}
              markdown={detail.tracker === 'forge'}
            />
          ))}
        </ol>
      ) : null}
      {total > comments.length ? (
        <p className="text-sm text-muted-foreground">
          Showing {comments.length} of {total} comments.
        </p>
      ) : null}
      <CommentComposer issueKey={detail.key} tracker={detail.tracker} />
    </section>
  )
}

// Trade-off TRADE-31: past the budget below, a comment is drawn unparsed.
//
// beyondParsing marks the comments a thread draws as plain text: each one past
// the first, counting back from the newest, that takes the thread's bodies over
// the size one comment is parsed at. Each comment is bounded on its own, but a
// thread of many near the bound would still stall the page.
function beyondParsing(comments: Comment[]): boolean[] {
  let parsed = 0

  return comments
    .toReversed()
    .map((comment) => {
      parsed += comment.body.length

      return parsed > largestParsedBody
    })
    .toReversed()
}

// CommentItem is one comment: who wrote it, beside their initials, when, and
// what, drawn from Jira's wiki markup, or from Markdown on a forge issue.
function CommentItem({
  comment,
  markdown,
  plain,
}: {
  comment: Comment
  markdown: boolean
  plain: boolean
}) {
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
          <CommentBody body={comment.body} markdown={markdown} plain={plain} />
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

// A comment is said by how long ago it was written within the week, and by
// its date after that.
const week = 7 * day

// WrittenAt says when a comment was written: how long ago within the week,
// and the day after; the exact moment shows on hover. A comment the tracker
// gave no date for, which the server leaves the date out of, says nothing.
function WrittenAt({ created }: { created: string | undefined }) {
  if (created === undefined) {
    return null
  }

  const written = new Date(created)

  return (
    <time
      dateTime={created}
      title={writtenMoment(written)}
      className="text-xs text-muted-foreground"
    >
      {ago(written.getTime(), new Date().getTime(), { style: 'words', dateAfter: week })}
    </time>
  )
}

// The forges by name, as the server's health tells them apart; one it cannot
// name, or one not yet read, is just the forge.
const forgeNames: Record<Health['forge_kind'], string> = {
  github: 'GitHub',
  gitlab: 'GitLab',
  unknown: 'the forge',
}

// useTrackerName names where an issue lives, for its link: Jira, or the forge
// the repository is on, by which forge the server's health says it is.
function useTrackerName(tracker: IssueDetail['tracker']): string {
  const kind = useHealthStore((state) => state.health?.forge_kind ?? 'unknown')

  return tracker === 'jira' ? 'Jira' : forgeNames[kind]
}
