import { useEffect, useRef, useState } from 'react'
import { cn } from './utils.ts'

// Said is what a write said, the element that had focus as it said it — the
// write's own control, as a rule — and whether that was already somewhere
// other than the control the write was started from: the user moved on.
interface Said {
  text: string
  from: Element | null
  movedOn: boolean
}

// Teller is what a write's control is handed of its panel's outcome: clear, as
// the write starts, so a new run never stands beside the last one's success,
// and say, once it is done.
export interface Teller {
  clear: () => void
  say: (text: string) => void
}

// useOutcome is a panel's outcome: say puts what a write did in the panel's
// OutcomeLine, and clear takes it away again. The line stays mounted when the
// snapshot confirming the write takes the write's control away — the button
// that checked out a branch goes once the branch is on HEAD — so what was said
// stays said until the next write starts. clear, as the write starts, notes the
// control it was started from: a write said once focus has left that control
// leaves focus where the user put it, at once, rather than waiting on whatever
// holds it — which the user may yet press for a write of its own.
export function useOutcome() {
  const [said, setSaid] = useState<Said | null>(null)
  const started = useRef<Element | null>(null)

  return {
    said,
    clear: () => {
      started.current = document.activeElement
      setSaid(null)
    },
    say: (text: string) => {
      const from = document.activeElement
      const movedOn = started.current !== null && from !== started.current && from !== document.body
      setSaid({ text, from, movedOn })
    },
  }
}

// OutcomeLine is the live line useOutcome says into. It is always mounted, empty
// until there is something to say, so a screen reader hears each message as it
// changes. When the control that had focus as the write was said can no longer
// hold it — gone from the page, or disabled — or focus had already fallen to
// the page, focus follows to the line rather than staying lost; focus the user
// has moved on to, before the write was said or since, even to the page itself
// while that control stands, stays where they put it.
export function OutcomeLine({ said, className }: { said: Said | null; className?: string }) {
  const line = useRef<HTMLParagraphElement>(null)
  // The last outcome whose focus is settled: handed to the line, or left with
  // the user.
  const settled = useRef<Said | null>(null)

  useEffect(() => {
    if (said === null || settled.current === said) {
      return
    }

    const focused = document.activeElement
    if (said.movedOn || (focused !== document.body && focused !== said.from)) {
      settled.current = said

      return
    }

    // Follow to the line only once the control that said it is gone or off; a
    // control still standing whose focus the user dropped to the page leaves
    // it there, and one still holding focus may yet be taken away.
    const gone = said.from === document.body || !canHoldFocus(said.from)
    if (gone) {
      settled.current = said
      line.current?.focus()
    } else if (focused !== said.from) {
      settled.current = said
    }
  })

  return (
    <p
      ref={line}
      role="status"
      tabIndex={-1}
      className={cn('text-sm text-success empty:sr-only', className)}
    >
      {said?.text ?? ''}
    </p>
  )
}

// canHoldFocus reports whether an element can still have focus: it is on the
// page and not disabled.
function canHoldFocus(element: Element | null): boolean {
  return element !== null && element.isConnected && !element.matches(':disabled')
}
