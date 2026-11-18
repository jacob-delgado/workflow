import { useRef, useState } from 'react'
import { apiErrorMessage, problemCode } from '@/api/apiError.ts'

// Attempt is where a one-shot write stands, each state carrying only what it
// means: not yet run (or backed out of), in flight, answered with what the
// server said, or refused with a message safe to show and the refusal's
// machine code, for a control that offers a way out of one kind of refusal
// (empty when the refusal names none).
export type Attempt<T> =
  | { state: 'idle' }
  | { state: 'running' }
  | { state: 'done'; result: T }
  | { state: 'error'; error: string; code: string }

export type AsyncState = Attempt<unknown>['state']

// refusalOf is why the first refused of attempts was refused, or '' when none
// was: the words a write's control shows under it, and, for a control drawn
// from more than one write, one refusal at a time.
export function refusalOf(...attempts: Attempt<unknown>[]): string {
  return attempts.find((attempt) => attempt.state === 'error')?.error ?? ''
}

// ActionWords are what a write says: its fallback when a refusal gives no
// reason of its own, and — for a write that says what it did — the message it
// ends on, from what the server answered and what it was asked, with whoever
// should hear that message.
interface ActionWords<A extends unknown[], T> {
  fallback: string
  done?: (result: T, ...args: A) => string
  // onStart hears the write start: a panel's outcome line lets go of what the
  // last write said, so it never stands beside this one's refusal.
  onStart?: () => void
  // onDone hears the message, and what the write answered, once it is done: a
  // panel's outcome line, which outlives the control that the snapshot
  // confirming the write takes away.
  onDone?: (message: string, result: T) => void
}

// useAsyncAction runs a one-shot write — or a read a step waits on, such as a
// preview — and tracks it for the control that starts it as an Attempt.
// reset puts it back to idle, for a step the user backs out of, and lets go of
// a run still going: what that run answers is not heard, by the hook or by
// onDone, since only the latest run's answer is the control's. It lives here,
// beside no one feature, so every write shares the one state machine rather
// than declaring its own.
export function useAsyncAction<A extends unknown[], T>(
  action: (...args: A) => Promise<T>,
  words: ActionWords<A, T>,
) {
  const [attempt, setAttempt] = useState<Attempt<T>>({ state: 'idle' })
  const latest = useRef(0)

  const run = async (...args: A): Promise<void> => {
    latest.current += 1
    const thisRun = latest.current
    words.onStart?.()
    setAttempt({ state: 'running' })
    try {
      const answered = await action(...args)
      if (thisRun !== latest.current) {
        return
      }
      const said = words.done?.(answered, ...args) ?? ''
      setAttempt({ state: 'done', result: answered })
      words.onDone?.(said, answered)
    } catch (caught) {
      if (thisRun !== latest.current) {
        return
      }
      setAttempt({
        state: 'error',
        error: apiErrorMessage(caught, words.fallback),
        code: problemCode(caught),
      })
    }
  }

  const reset = () => {
    latest.current += 1
    setAttempt({ state: 'idle' })
  }

  return { ...attempt, run, reset }
}
