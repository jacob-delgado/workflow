import { create } from 'zustand'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { checkoutBranch } from './checkoutApi.ts'

// The branch switches the page has out, from the issue rows and the issue's
// story alike, each by a mark of its own, so one that lands only takes away
// its own.
const useSwitchesOut = create<{ out: ReadonlySet<symbol> }>(() => ({ out: new Set() }))

// useBranchSwitch switches the working tree to branch. Where it went is said
// in outcome; a refusal, such as a dirty tree, is the action's error. One
// switch goes at a time from the page: while any runs, held holds every
// other, since two would race for the one working tree, and the last to land,
// not the last said, would stand.
export function useBranchSwitch(branch: string, outcome: Teller) {
  const held = useSwitchesOut((state) => state.out.size > 0)
  const action = useAsyncAction(
    async () => {
      const mark = Symbol(branch)
      useSwitchesOut.setState(({ out }) => ({ out: new Set(out).add(mark) }))
      try {
        return await checkoutBranch(branch)
      } finally {
        useSwitchesOut.setState(({ out }) => ({ out: without(out, mark) }))
      }
    },
    {
      fallback:
        'The branch was not switched to. Try again, or switch to it from a terminal to see why.',
      done: (switched) => `Switched to ${switched.name}.`,
      onStart: outcome.clear,
      onDone: outcome.say,
    },
  )

  return { ...action, held }
}

// without is out with mark taken away.
function without(out: ReadonlySet<symbol>, mark: symbol): ReadonlySet<symbol> {
  const left = new Set(out)
  left.delete(mark)

  return left
}
