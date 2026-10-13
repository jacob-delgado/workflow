import type { ReactNode } from 'react'
import type { Problem } from '@/api/generated/types.gen.ts'
import { StateMark } from './StateMark.tsx'
import { Button } from './Button.tsx'
import { cn } from './utils.ts'

// failureGround draws a failure in the one color kept for it, on that color's
// notice ground so the eye finds it: Failure's look, for a line that says a
// failure without being an alert.
export const failureGround =
  'rounded-md border border-destructive/40 bg-destructive-surface px-3 py-2 text-sm text-destructive'

interface FailureProps {
  children: ReactNode
  // id names the failure for the field it describes (aria-describedby).
  id?: string
  className?: string
}

// Failure says what failed, beside what failed: an alert, so a screen reader
// hears it as it lands, drawn on the failure's ground. With nothing to say it
// draws nothing, though it stays in place to be heard. A caller that can fail
// the same way twice keys it by how many times it has, since an alert that
// keeps its words is not spoken again.
export function Failure({ children, id, className }: FailureProps) {
  return (
    <p
      id={id}
      role="alert"
      className={cn(
        'flex items-start gap-2 whitespace-pre-line empty:sr-only',
        failureGround,
        className,
      )}
    >
      {children}
    </p>
  )
}

interface ReadFailureProps {
  // What could not be read, as a sentence starts: "The issues could not be read".
  unread: string
  // What is not set up, as a sentence starts: "The forge" — said instead when
  // the problem's code is not_set_up.
  notSetUp?: string
  problem: Problem
}

// ReadFailure is a read the server could not make, shown as that failure
// rather than as the empty answer it left in its place: the failure mark
// beside the sentence that says what could not be read, then why, as an
// alert. The reason is the problem's detail, which the server curates and
// never lets carry a host. A service that is not set up was never asked, so
// nothing failed: it is said as NotSetUp's guidance instead.
export function ReadFailure({ unread, notSetUp, problem }: ReadFailureProps) {
  if (notSetUp !== undefined && problem.code === 'not_set_up') {
    return <NotSetUp>{`${notSetUp} is not set up: ${problem.detail}`}</NotSetUp>
  }

  return (
    <Failure>
      <StateMark state="failed" className="mt-0.5 shrink-0" />
      <span>{`${unread}: ${problem.detail}`}</span>
    </Failure>
  )
}

// NotSetUp says a service is not set up and how to set it up: guidance, not a
// failure — the not-started mark beside a plain status line in the muted
// color, as the terminal draws it, where a refusal takes the failure's.
export function NotSetUp({ children }: { children: ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <StateMark state="not-started" className="mt-0.5" />
      <p role="status" className="text-sm whitespace-pre-line text-muted-foreground">
        {children}
      </p>
    </div>
  )
}

interface UnreadProps {
  reason: string
  refusals: number
  retrying: boolean
  onRetry: () => void
}

// Unread is a read that failed: why, as a Failure keyed by how many refusals
// there have been, over a Try again that reads it again. While it reads, Try
// again is held rather than disabled, so it keeps the focus it was pressed
// with through another refusal, and a press while held starts nothing.
export function Unread({ reason, refusals, retrying, onRetry }: UnreadProps) {
  return (
    <div className="flex flex-col items-start gap-item">
      <Failure key={refusals}>{reason}</Failure>
      <Button variant="secondary" held={retrying} onClick={onRetry}>
        {retrying ? 'Trying again…' : 'Try again'}
      </Button>
    </div>
  )
}

// Reading says a read is in flight, in words that begin "Reading": a muted
// status line, so a screen reader hears the wait as it starts, and the same
// line for a first read as for a read again.
export function Reading({ children }: { children: ReactNode }) {
  return (
    <p role="status" className="text-sm text-muted-foreground">
      {children}
    </p>
  )
}
