import type { CiState } from '@/api/generated/types.gen.ts'
import { cn } from '@/lib/utils.ts'

// A state in the interface's own vocabulary (internal/tui/glyphs.go): not
// started ○, in flight ◐, done ●, failed ✗, and unknown · for a service that
// reports none.
export type MarkState = 'not-started' | 'in-flight' | 'done' | 'failed' | 'unknown'

// The status light each state reports in, unless its mark sits on the loop and
// takes its system's hue instead (className). Red is failure, and nothing else.
const statusLight: Record<MarkState, string> = {
  'not-started': 'text-muted-foreground',
  'in-flight': 'text-warning',
  done: 'text-success',
  failed: 'text-destructive',
  unknown: 'text-muted-foreground',
}

// How CI stands, as a mark — the checks of the branch's pull request and the
// review queue share it, as the interface's Review pane and queue do.
export const ciMark: Record<CiState, MarkState> = {
  none: 'unknown',
  running: 'in-flight',
  passed: 'done',
  failed: 'failed',
}

// StateMark draws a state by its shape, so it reads without its color — in
// either theme, to a reader who cannot tell the lights apart. Inline SVG rather
// than the characters, which each platform's fallback font sizes and seats
// differently. It is hidden from assistive tech: the state's words always stand
// beside it.
export function StateMark({ state, className }: { state: MarkState; className?: string }) {
  return (
    <svg
      aria-hidden
      viewBox="0 0 16 16"
      className={cn('size-3.5 shrink-0', statusLight[state], className)}
    >
      <Shape state={state} />
    </svg>
  )
}

function Shape({ state }: { state: MarkState }) {
  switch (state) {
    case 'not-started':
      return <Ring />
    case 'in-flight':
      return (
        <>
          <Ring />
          <path d="M8 2.75a5.25 5.25 0 0 0 0 10.5z" fill="currentColor" />
        </>
      )
    case 'done':
      return <circle cx="8" cy="8" r="6" fill="currentColor" />
    case 'failed':
      return (
        <path
          d="M4 4l8 8M12 4l-8 8"
          fill="none"
          stroke="currentColor"
          strokeWidth="2.25"
          strokeLinecap="round"
        />
      )
    case 'unknown':
      return <circle cx="8" cy="8" r="2" fill="currentColor" />
  }
}

function Ring() {
  return <circle cx="8" cy="8" r="5.25" fill="none" stroke="currentColor" strokeWidth="1.5" />
}
