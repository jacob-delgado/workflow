import { useState } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'

type AsyncState = 'idle' | 'running' | 'error'

// useAsyncAction runs a one-shot write and tracks its state for a button: it is
// running while the promise is pending, and holds the failure's message — safe to
// show — when it rejects. The event stream reflects a success, so there is
// nothing to return on the happy path. It lives here, beside no one feature, so
// every one-shot write's button shares the one state machine rather than
// declaring its own.
export function useAsyncAction(action: () => Promise<void>, fallback: string) {
  const [state, setState] = useState<AsyncState>('idle')
  const [error, setError] = useState('')

  const run = async () => {
    setState('running')
    try {
      await action()
      setError('')
      setState('idle')
    } catch (caught) {
      setError(apiErrorMessage(caught, fallback))
      setState('error')
    }
  }

  return { state, error, run }
}
