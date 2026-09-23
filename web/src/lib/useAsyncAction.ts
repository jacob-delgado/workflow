import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'

// AsyncState is where a one-shot write stands: not yet run (or backed out of),
// in flight, answered, or refused.
export type AsyncState = 'idle' | 'running' | 'done' | 'error'

// ActionWords are what a write says when a refusal gives no reason of its own.
interface ActionWords {
  fallback: string
}

// useAsyncAction runs a one-shot write — or a read a step waits on, such as a
// preview — and tracks it for the control that starts it: running while the
// promise is pending, done with what it answered once it resolves, and holding
// the failure's message, safe to show, when it rejects. reset puts it back to
// idle, for a step the user backs out of. It lives here, beside no one feature,
// so every write shares the one state machine rather than declaring its own.
export function useAsyncAction<A extends unknown[], T>(
  action: (...args: A) => Promise<T>,
  words: ActionWords,
) {
  const [state, setState] = useState<AsyncState>('idle')
  const [result, setResult] = useState<T>()
  const [error, setError] = useState('')

  const run = async (...args: A): Promise<void> => {
    setState('running')
    try {
      const answered = await action(...args)
      setResult(() => answered)
      setError('')
      setState('done')
    } catch (caught) {
      setError(apiErrorMessage(caught, words.fallback))
      setState('error')
    }
  }

  const reset = () => {
    setError('')
    setState('idle')
  }

  return { state, result, error, run, reset }
}
