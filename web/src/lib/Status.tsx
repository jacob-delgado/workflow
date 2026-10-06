import type { ReactNode } from 'react'
import { Button } from './Button.tsx'
import { cn } from './utils.ts'

// Failure says what failed, beside what failed: an alert, so a screen reader
// hears it as it lands, in the one color kept for failure. A caller that can
// fail the same way twice keys it by how many times it has, since an alert
// that keeps its words is not spoken again.
export function Failure({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <p role="alert" className={cn('text-sm whitespace-pre-line text-destructive', className)}>
      {children}
    </p>
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
// again is marked busy rather than disabled, so it keeps the focus it was
// pressed with through another refusal, and a press while busy starts nothing.
export function Unread({ reason, refusals, retrying, onRetry }: UnreadProps) {
  return (
    <div className="flex flex-col items-start gap-item">
      <Failure key={refusals}>{reason}</Failure>
      <Button
        variant="secondary"
        aria-disabled={retrying}
        onClick={() => {
          if (!retrying) {
            onRetry()
          }
        }}
      >
        {retrying ? 'Trying again…' : 'Try again'}
      </Button>
    </div>
  )
}
