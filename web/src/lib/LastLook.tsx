import { useEffect, useId, useRef, type ReactNode } from 'react'
import { create } from 'zustand'
import { Button } from './Button.tsx'
import { Failure } from './Status.tsx'
import type { AsyncState } from './useAsyncAction.ts'
import { cn } from './utils.ts'

// How many questions drawn now hold the single-key shortcuts: a last look, or
// a preview of what is about to be sent.
const useHolds = create<{ holds: number }>(() => ({ holds: 0 }))

// useHoldShortcuts holds every single-key shortcut while the question calling
// it is drawn — a last look, a preview — since it asks something of its own,
// and a key pressed while it is open would otherwise press a control behind
// it. LastLook holds them itself; a preview that is no last look calls this.
export function useHoldShortcuts(): void {
  useEffect(() => {
    useHolds.setState(({ holds }) => ({ holds: holds + 1 }))

    return () => {
      useHolds.setState(({ holds }) => ({ holds: holds - 1 }))
    }
  }, [])
}

// shortcutsHeld reports a question drawn now that holds the single keys.
export function shortcutsHeld(): boolean {
  return useHolds.getState().holds > 0
}

// Named is what names a last look: the question it shows, or, for a preview
// whose content is its question, a label of its own.
type Named = { question: ReactNode; label?: never } | { label: string; question?: never }

// Written is a write the last look makes in place, as useAsyncAction tracks
// it: running holds both buttons, and a refusal is shown above them.
interface Written {
  state: AsyncState
  error: string
}

type LastLookProps = Named & {
  // cost is what the act costs, in the muted line under the question.
  cost?: ReactNode
  // children are what is about to be sent, shown before the buttons.
  children?: ReactNode
  // act is the act's own word on its button, and acting the word while it runs.
  act: string
  acting?: string
  // write is the write a last look makes in place; one that hands it back to
  // what opened it, which writes and shows its refusal, passes none.
  write?: Written
  // section draws the last look as a section of its own under a heading, for
  // one asked at the foot of a long list and scrolled to.
  section?: boolean
  className?: string
  onAct: () => void
  onCancel: () => void
}

// LastLook asks before a write that cannot be taken back: the question and
// what it costs, what is about to be sent, a refusal, and Cancel beside the
// act. It takes the focus as it opens, so a screen reader hears the question,
// and holds the single-key shortcuts while it is drawn. While its write runs
// both buttons are held: the act keeps the focus it was pressed with, and
// Cancel cannot call back a write that has left.
export function LastLook(props: LastLookProps) {
  const { cost, children, act, acting = act, write, className, onAct, onCancel } = props
  const running = write?.state === 'running'
  useHoldShortcuts()

  return (
    <Asking {...props} className={cn('flex flex-col items-start gap-item text-sm', className)}>
      {cost === undefined ? null : <p className="text-muted-foreground">{cost}</p>}
      {children}
      {write?.state === 'error' ? <Failure>{write.error}</Failure> : null}
      <div className="flex flex-wrap items-center gap-item">
        <Button variant="secondary" held={running} onClick={onCancel}>
          Cancel
        </Button>
        <Button variant="primary" held={running} onClick={onAct}>
          {running ? acting : act}
        </Button>
      </div>
    </Asking>
  )
}

type AskingProps = Named & { section?: boolean; className: string; children: ReactNode }

// Asking is the last look's frame, named by its question and given the focus
// as it is drawn: a group, or a section whose heading takes the focus.
function Asking({ question, label, section = false, className, children }: AskingProps) {
  const questionId = useId()
  const focused = useRef<HTMLElement | null>(null)
  useEffect(() => {
    focused.current?.focus()
  }, [])

  if (section) {
    return (
      <section aria-labelledby={questionId} className={className}>
        <h2
          id={questionId}
          ref={(element) => {
            focused.current = element
          }}
          tabIndex={-1}
          className="text-base font-semibold focus-visible:outline-none"
        >
          {question ?? label}
        </h2>
        {children}
      </section>
    )
  }

  return (
    <div
      ref={(element) => {
        focused.current = element
      }}
      role="group"
      aria-labelledby={question === undefined ? undefined : questionId}
      aria-label={label}
      tabIndex={-1}
      className={cn(
        'rounded-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        className,
      )}
    >
      {question === undefined ? null : <p id={questionId}>{question}</p>}
      {children}
    </div>
  )
}
