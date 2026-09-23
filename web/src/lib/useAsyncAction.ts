import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'

// AsyncState is where a one-shot write stands: not yet run (or backed out of),
// in flight, answered, or refused.
export type AsyncState = 'idle' | 'running' | 'done' | 'error'

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
// preview — and tracks it for the control that starts it: running while the
// promise is pending, done with what it answered and what that means once it
// resolves, and holding the failure's message, safe to show, when it rejects.
// reset puts it back to idle, for a step the user backs out of. It lives here,
// beside no one feature, so every write shares the one state machine rather
// than declaring its own.
export function useAsyncAction<A extends unknown[], T>(
  action: (...args: A) => Promise<T>,
  words: ActionWords<A, T>,
) {
  const [state, setState] = useState<AsyncState>('idle')
  const [result, setResult] = useState<T>()
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  const run = async (...args: A): Promise<void> => {
    words.onStart?.()
    setState('running')
    try {
      const answered = await action(...args)
      const said = words.done?.(answered, ...args) ?? ''
      setResult(() => answered)
      setMessage(said)
      setError('')
      setState('done')
      words.onDone?.(said, answered)
    } catch (caught) {
      setError(apiErrorMessage(caught, words.fallback))
      setState('error')
    }
  }

  const reset = () => {
    setError('')
    setState('idle')
  }

  return { state, result, message, error, run, reset }
}
