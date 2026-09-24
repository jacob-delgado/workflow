import { ExternalLink } from 'lucide-react'
import type { ReactNode } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { CiState, ReviewRequest } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { ciMark, StateMark } from '@/shell/StateMark.tsx'
import { useReviewQueue } from './reviewQueueApi.ts'

// ciLabel says how CI stands on a request, in words: the queue is read without
// a request per entry, so a forge whose listing carries no summary reports none.
const ciLabel: Record<CiState, string> = {
  none: 'CI not reported',
  running: 'CI running',
  passed: 'CI passed',
  failed: 'CI failed',
}

const minute = 60_000
const hour = 60 * minute
const day = 24 * hour
const month = 30 * day

const control =
  'rounded-md border border-input px-3 py-1.5 text-sm hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none'

// ReviewQueuePanel lists the pull requests on the forge that wait on your
// review, the longest-waiting first — the queue `workflow reviews` prints and
// the terminal's Reviews pane shows. It reads its own endpoint rather than the
// stream: a search of the forge is worth a request only when someone looks, so
// the section reads it as it opens, unless it was read within the minute, and
// again when Refresh asks.
export function ReviewQueuePanel() {
  const query = useReviewQueue()
  // A read after a failed first read is pending again, with no queue yet; it
  // is a retry, which keeps the queue's place, and its control's focus, rather
  // than falling back to the first read's placeholder.
  const retrying = query.isPending && query.errorUpdateCount > 0

  if (query.isPending && !retrying) {
    return <EmptyState>Reading your review queue…</EmptyState>
  }

  if (query.data?.available === false) {
    return <NoForgeToAsk />
  }

  return (
    <Queue
      requests={query.data?.requests}
      readAt={query.dataUpdatedAt}
      failure={
        query.isError
          ? apiErrorMessage(
              query.error,
              'The review queue could not be read. Press Retry to try again.',
            )
          : null
      }
      failed={query.isError || retrying}
      reading={query.isFetching}
      onReadAgain={() => {
        void query.refetch()
      }}
    />
  )
}

interface QueueProps {
  // requests is the queue as last read, or undefined when no read has landed.
  requests: ReviewRequest[] | undefined
  // readAt is when that read landed, which is what each age is counted from.
  readAt: number
  // failure is why the last read failed, while it stands.
  failure: string | null
  // failed is whether the last read failed, still so while a retry is in
  // flight.
  failed: boolean
  reading: boolean
  onReadAgain: () => void
}

// Queue is the queue beneath the lines that say how it stands — how many wait,
// and why the last read failed — beside the one control that reads it again.
// That control stays the same button whether it says Retry or Refresh, so the
// read it starts never takes its focus away; a failed refresh leaves the queue
// last read in view. The summary is a status line that stays mounted, so a
// screen reader hears what each read found as it lands.
function Queue({ requests, readAt, failure, failed, reading, onReadAgain }: QueueProps) {
  const { noun } = useForgeWords()
  const outcome = useOutcome()

  return (
    <div className="mt-4 flex max-w-3xl flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <div className="flex flex-col">
          <p role="status" className="text-sm text-muted-foreground">
            {requests === undefined ? '' : queueSummary(requests.length, noun)}
          </p>
          {failure === null ? null : (
            <p role="alert" className="text-sm text-destructive">
              {failure}
            </p>
          )}
        </div>
        <button
          type="button"
          aria-disabled={reading}
          onClick={() => {
            if (!reading) {
              onReadAgain()
            }
          }}
          className={`${control} shrink-0 aria-disabled:cursor-not-allowed aria-disabled:bg-disabled aria-disabled:text-disabled-foreground`}
        >
          {readAgainLabel(failed, reading)}
        </button>
      </div>
      <OutcomeLine said={outcome.said} />
      {requests === undefined ? null : (
        <Requests requests={requests} readAt={readAt} teller={outcome} />
      )}
    </div>
  )
}

// queueSummary says how many requests wait, in the forge's own noun. An empty
// queue says so on screen below, in the list's place, so here it is said only
// to a screen reader.
function queueSummary(count: number, noun: string): ReactNode {
  if (count === 0) {
    return <span className="sr-only">Nothing is waiting on your review.</span>
  }

  return count === 1
    ? `1 ${noun} waits on your review, oldest first.`
    : `${String(count)} ${noun}s wait on your review, oldest first.`
}

// readAgainLabel names the control that reads the queue again: Retry after a
// failed read, Refresh otherwise, each saying so while the read is in flight.
function readAgainLabel(failed: boolean, reading: boolean): string {
  if (failed) {
    return reading ? 'Retrying…' : 'Retry'
  }

  return reading ? 'Refreshing…' : 'Refresh'
}

interface RequestsProps {
  requests: ReviewRequest[]
  readAt: number
  teller: Teller
}

function Requests({ requests, readAt, teller }: RequestsProps) {
  if (requests.length === 0) {
    return <EmptyState>Nothing is waiting on your review.</EmptyState>
  }

  return (
    <ul
      aria-label="Review requests"
      className="flex flex-col divide-y divide-border rounded-lg border border-border"
    >
      {requests.map((request) => (
        <RequestRow key={request.url} request={request} readAt={readAt} teller={teller} />
      ))}
    </ul>
  )
}

interface RequestRowProps {
  request: ReviewRequest
  readAt: number
  teller: Teller
}

// RequestRow is one request: its number in the forge's own mark and its title;
// where it is, who asks, how long it has waited and whether it is a draft; how
// its CI stands; and a link to open it and a control to copy its URL.
function RequestRow({ request, readAt, teller }: RequestRowProps) {
  const { sigil } = useForgeWords()
  const mark = `${sigil}${String(request.number)}`

  return (
    <li className="flex flex-col gap-1.5 px-4 py-3">
      <p className="flex items-baseline gap-2">
        <span className="font-mono text-sm text-muted-foreground">{mark}</span>
        <span className="font-medium">{request.title}</span>
      </p>
      <p className="text-sm text-muted-foreground">
        {whereAndWho(request)} ·{' '}
        <time dateTime={request.opened_at}>{waited(request.opened_at, readAt)}</time>
        {request.draft ? ' · Draft' : ''}
      </p>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
        <span className="flex items-center gap-1.5">
          <StateMark state={ciMark[request.ci]} />
          {ciLabel[request.ci]}
        </span>
        <a
          href={request.url}
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-1.5 text-primary underline-offset-4 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
        >
          Open
          <ExternalLink aria-hidden className="size-3.5" />{' '}
          <span className="sr-only">{mark} (opens in a new tab)</span>
        </a>
        <CopyURL url={request.url} mark={mark} teller={teller} />
      </div>
    </li>
  )
}

// whereAndWho is the repository a request is in, when the forge said, and who
// asks for the review.
function whereAndWho(request: ReviewRequest): string {
  const who = `by ${request.author}`

  return request.repository === '' ? who : `${request.repository} · ${who}`
}

// waited is how long before the queue was read a request was opened, in the
// terminal's words: just now, then minutes, hours and days, and the date past
// a month. The server sends the zero time for a date the forge did not give.
function waited(openedAt: string, readAt: number): string {
  const opened = Date.parse(openedAt)
  if (opened <= 0) {
    return 'some time ago'
  }

  const elapsed = readAt - opened
  if (elapsed < minute) {
    return 'just now'
  }

  if (elapsed < hour) {
    return `${String(Math.floor(elapsed / minute))}m ago`
  }

  if (elapsed < day) {
    return `${String(Math.floor(elapsed / hour))}h ago`
  }

  return elapsed < month
    ? `${String(Math.floor(elapsed / day))}d ago`
    : new Date(opened).toLocaleDateString()
}

// copyAddress puts an address on the clipboard.
function copyAddress(url: string): Promise<void> {
  return navigator.clipboard.writeText(url)
}

interface CopyURLProps {
  url: string
  mark: string
  teller: Teller
}

// CopyURL copies a request's URL — the interface's "copy url" — and says so in
// the panel's outcome line; a copy the browser refuses says, beside it, how to
// get the URL.
function CopyURL({ url, mark, teller }: CopyURLProps) {
  const copy = useAsyncAction(copyAddress, {
    fallback: `The URL of ${mark} could not be copied; open it, and copy it from the address bar.`,
    done: () => `Copied the URL of ${mark}.`,
    onStart: teller.clear,
    onDone: teller.say,
  })

  return (
    <>
      <button
        type="button"
        onClick={() => {
          void copy.run(url)
        }}
        className={control}
      >
        Copy URL <span className="sr-only">to {mark}</span>
      </button>
      {copy.state === 'error' ? (
        <p role="alert" className="basis-full text-destructive">
          {copy.error}
        </p>
      ) : null}
    </>
  )
}

// NoForgeToAsk says there is no forge to read the queue from here, and what
// would give it one.
function NoForgeToAsk() {
  return (
    <EmptyState>
      <span>
        No forge to ask: reviews come from the GitHub or GitLab this repository&apos;s origin is on.
        Start <code className="font-mono">workflow --web</code> in a repository on one; for a
        self-hosted forge, set <code className="font-mono">forge.kind</code> and{' '}
        <code className="font-mono">forge.host</code>.
      </span>
    </EmptyState>
  )
}
