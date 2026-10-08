import { useRef, type ReactNode, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { CiState, ReviewRequest } from '@/api/generated/types.gen.ts'
import { useForgeWords } from '@/api/health.ts'
import { useShortcut } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'
import { CopyURL } from '@/lib/CopyURL.tsx'
import { writtenDate } from '@/lib/dates.ts'
import { Select } from '@/lib/Field.tsx'
import { FilterChips } from '@/lib/FilterChips.tsx'
import { Meta } from '@/lib/Meta.tsx'
import { NewTabLink } from '@/lib/NewTabLink.tsx'
import { OutcomeLine, useOutcome } from '@/lib/Outcome.tsx'
import { Reading } from '@/lib/Status.tsx'
import { cn, contentMeasure, plural } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { ciMark, StateMark } from '@/shell/StateMark.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { admits, facetChoices, facetLabel, isPicked, toggleFacet } from './reviewFacets.ts'
import {
  byRepository,
  ordered,
  orderWords,
  repositoryHeading,
  type ReviewOrder,
} from './reviewOrder.ts'
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

// ReviewQueuePanel lists the pull requests on the forge that wait on your
// review, the longest-waiting first unless another order is chosen — the queue
// `workflow reviews` prints and the terminal's Reviews pane shows. It reads its
// own endpoint rather than the stream: a search of the forge is worth a request
// only when someone looks, so the section reads it as it opens, unless it was
// read within the last 30 seconds (freshFor), and again when Refresh asks.
export function ReviewQueuePanel() {
  const query = useReviewQueue()
  // A read after a failed first read is pending again, with no queue yet; it
  // is a retry, which keeps the queue's place, and its control's focus, rather
  // than falling back to the first read's placeholder.
  const retrying = query.isPending && query.errorUpdateCount > 0

  if (query.isPending && !retrying) {
    return <Reading>Reading your review queue…</Reading>
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
          ? apiErrorMessage(query.error, 'The review queue could not be read. Press Try again.')
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
// That control stays the same button whether it says Try again or Refresh, so the
// read it starts never takes its focus away; a failed refresh leaves the queue
// last read in view. The summary is a status line that stays mounted, so a
// screen reader hears what each read found as it lands. The order and the
// filter chosen hold across reads, and across visits to other sections.
function Queue({ requests, readAt, failure, failed, reading, onReadAgain }: QueueProps) {
  const { noun } = useForgeWords()
  const order = useUiStore((state) => state.reviewOrder)
  const setOrder = useUiStore((state) => state.setReviewOrder)
  const picked = useUiStore((state) => state.reviewFilter)
  const pick = useUiStore((state) => state.pickReviewFilter)
  const sort = useRef<HTMLSelectElement>(null)
  const shown = requests?.filter((request) => admits(picked, request))

  return (
    <div className={cn('flex flex-col gap-group', contentMeasure)}>
      <div className="flex items-center justify-between gap-group">
        <div className="flex flex-col">
          <p role="status" className="text-sm text-muted-foreground">
            {requests === undefined || shown === undefined
              ? ''
              : queueSummary(requests.length, shown.length, noun, order)}
          </p>
          {failure === null ? null : (
            <p role="alert" className="text-sm text-destructive">
              {failure}
            </p>
          )}
        </div>
        <Button variant="secondary" held={reading} onClick={onReadAgain} className="shrink-0">
          {readAgainLabel(failed, reading)}
        </Button>
      </div>
      <OrderSelect ref={sort} order={order} onOrder={setOrder} />
      {requests === undefined ? null : (
        <FilterChips
          label="Filter"
          choices={facetChoices(requests, picked)}
          isPicked={(facet) => isPicked(picked, facet)}
          nameOf={facetLabel}
          keyOf={(facet) => `${facet.kind}:${facet.value}`}
          afterLast={sort}
          onToggle={(facet) => {
            pick((now) => toggleFacet(now, facet))
          }}
        />
      )}
      {/* The queue's own heading, unseen, so each repository's h3 does not
          skip a level under the section's h1. */}
      <h2 className="sr-only">Waiting on your review</h2>
      {requests === undefined || shown === undefined ? null : (
        <Requests
          requests={ordered(shown, order)}
          filtered={shown.length < requests.length}
          grouped={order === 'repository'}
          readAt={readAt}
        />
      )}
    </div>
  )
}

// emptyQueue is the empty queue in the forge's own noun, the sentence the
// terminal and `workflow reviews` say too.
function emptyQueue(noun: string): string {
  return `No ${noun}s are waiting on your review.`
}

// queueSummary says how many requests wait, in the forge's own noun, the
// order they are listed in, and how many the filter shows when it hides any.
// An empty queue says so on screen below, in the list's place, so here it is
// said only to a screen reader.
function queueSummary(count: number, shown: number, noun: string, order: ReviewOrder): ReactNode {
  if (count === 0) {
    return <span className="sr-only">{emptyQueue(noun)}</span>
  }

  const listed = orderWords[order].toLowerCase()
  const filtered = shown < count ? `; ${String(shown)} shown` : ''

  return `${plural(count, noun)} ${count === 1 ? 'waits' : 'wait'} on your review, ${listed}${filtered}.`
}

interface OrderSelectProps {
  ref: RefObject<HTMLSelectElement | null>
  order: ReviewOrder
  onOrder: (order: ReviewOrder) => void
}

// OrderSelect chooses the order the queue is listed in, as the terminal's s
// cycles it.
function OrderSelect({ ref, order, onOrder }: OrderSelectProps) {
  const shortcut = useShortcut('sort-reviews', ref, 'focus')

  return (
    <label className="flex items-center gap-item text-sm">
      <span className="text-muted-foreground">Sort</span>
      <Select
        size="sm"
        ref={ref}
        aria-keyshortcuts={shortcut}
        value={order}
        onChange={(event) => {
          onOrder(event.target.value as ReviewOrder)
        }}
      >
        {(Object.keys(orderWords) as ReviewOrder[]).map((choice) => (
          <option key={choice} value={choice}>
            {orderWords[choice]}
          </option>
        ))}
      </Select>
    </label>
  )
}

// readAgainLabel names the control that reads the queue again: Try again after a
// failed read, Refresh otherwise, each saying so while the read is in flight.
function readAgainLabel(failed: boolean, reading: boolean): string {
  if (failed) {
    return reading ? 'Trying again…' : 'Try again'
  }

  return reading ? 'Refreshing…' : 'Refresh'
}

interface RequestsProps {
  requests: ReviewRequest[]
  // filtered is whether the filter hides any of the queue.
  filtered: boolean
  // grouped heads each repository's requests with its name, as the terminal
  // does by repository.
  grouped: boolean
  readAt: number
}

function Requests({ requests, filtered, grouped, readAt }: RequestsProps) {
  const { noun } = useForgeWords()
  if (requests.length === 0) {
    return (
      <EmptyState>{filtered ? 'No request matches the filters.' : emptyQueue(noun)}</EmptyState>
    )
  }

  if (!grouped) {
    return <RequestList label="Waiting on your review" requests={requests} readAt={readAt} />
  }

  return byRepository(requests).map(({ repository, requests: inRepository }) => (
    <section key={repository} className="flex flex-col gap-item">
      <h3 className="font-medium">{repositoryHeading(repository)}</h3>
      <RequestList
        label={`Waiting on your review in ${repositoryHeading(repository)}`}
        requests={inRepository}
        readAt={readAt}
      />
    </section>
  ))
}

interface RequestListProps {
  label: string
  requests: ReviewRequest[]
  readAt: number
}

function RequestList({ label, requests, readAt }: RequestListProps) {
  return (
    <ul
      aria-label={label}
      className="flex flex-col divide-y divide-border rounded-lg border border-border"
    >
      {requests.map((request) => (
        <RequestRow key={request.url} request={request} readAt={readAt} />
      ))}
    </ul>
  )
}

interface RequestRowProps {
  request: ReviewRequest
  readAt: number
}

// RequestRow is one request: its number in the forge's own mark and its title;
// where it is, who asks, how long it has waited and whether it is a draft; how
// its CI stands; a link to open it and a control to copy its URL; and, under
// them, what the last copy said, in the row whose URL it copied.
function RequestRow({ request, readAt }: RequestRowProps) {
  const { sigil } = useForgeWords()
  const copied = useOutcome()
  const mark = `${sigil}${String(request.number)}`

  return (
    <li className="flex flex-col gap-tight px-4 py-3">
      <p className="flex items-baseline gap-2">
        <span className="text-sm text-muted-foreground tabular-nums">{mark}</span>
        <span className="text-sm font-medium">{request.title}</span>
      </p>
      <Meta className="text-sm text-muted-foreground">
        {request.repository === '' ? null : request.repository}
        {`by ${request.author}`}
        <time dateTime={request.opened_at}>{waited(request.opened_at, readAt)}</time>
        {request.draft ? <DraftTag /> : null}
      </Meta>
      <div className="flex flex-wrap items-center gap-x-group gap-y-item text-sm">
        <span className="flex items-center gap-1.5">
          <StateMark state={ciMark[request.ci]} />
          {ciLabel[request.ci]}
        </span>
        <NewTabLink href={request.url}>
          Open <span className="sr-only">{mark}</span>
        </NewTabLink>
        <CopyURL url={request.url} mark={mark} teller={copied} />
      </div>
      <OutcomeLine said={copied.said} />
    </li>
  )
}

// DraftTag says a request is a draft, not yet ready for review: its mark
// carries the state, so the word stays in the plain foreground among the
// muted facts, and sits on their baseline while the mark centers on it.
function DraftTag() {
  return (
    <span className="inline-flex items-baseline gap-tight text-foreground">
      <StateMark state="not-started" className="self-center" />
      Draft
    </span>
  )
}

// waited is how long before the queue was read a request was opened, in the
// terminal's words: just now, then minutes, hours and days, and past a month
// the date, YYYY-MM-DD. A request the forge gave no time for, which the
// server leaves the time out of, waited some time.
function waited(openedAt: string | undefined, readAt: number): string {
  if (openedAt === undefined) {
    return 'some time ago'
  }

  const opened = Date.parse(openedAt)

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
    : writtenDate(new Date(opened))
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
