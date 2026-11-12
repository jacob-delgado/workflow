import { create } from 'zustand'
import type { Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { checkoutBranch } from './checkoutApi.ts'

// The moves of the working tree the page has out — a branch switch, from the
// issue rows or the issue's story, and work started here, which makes a branch
// and switches to it — each by a mark of its own, so one that lands only takes
// away its own.
const useMovesOut = create<{ out: ReadonlySet<symbol> }>(() => ({ out: new Set() }))

// useTreeHeld reports a move of the working tree out from the page. One move
// goes at a time: while any runs, every other is held, since two would race
// for the one working tree, and the last to land, not the last said, would
// stand.
export function useTreeHeld(): boolean {
  return useMovesOut((state) => state.out.size > 0)
}

// movingTheTree runs move, which moves the working tree to what named names,
// marked out from the page until it lands or is refused.
export async function movingTheTree<T>(named: string, move: () => Promise<T>): Promise<T> {
  const mark = Symbol(named)
  useMovesOut.setState(({ out }) => ({ out: new Set(out).add(mark) }))
  try {
    return await move()
  } finally {
    useMovesOut.setState(({ out }) => ({ out: without(out, mark) }))
  }
}

// useBranchSwitch switches the working tree to branch. Where it went is said
// in outcome; a refusal, such as a dirty tree, is the action's error. While
// any move of the working tree runs from the page, held holds this one.
export function useBranchSwitch(branch: string, outcome: Teller) {
  const held = useTreeHeld()
  const action = useAsyncAction(() => movingTheTree(branch, () => checkoutBranch(branch)), {
    fallback:
      'The branch was not switched to. Try again, or switch to it from a terminal to see why.',
    done: (switched) => `Switched to ${switched.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return { ...action, held }
}

// without is out with mark taken away.
function without(out: ReadonlySet<symbol>, mark: symbol): ReadonlySet<symbol> {
  const left = new Set(out)
  left.delete(mark)

  return left
}
