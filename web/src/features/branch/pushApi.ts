import { push } from '@/api/generated'
import type { Branch } from '@/api/generated/types.gen.ts'

// pushBranch publishes the current branch to its remote and returns it as
// published, for the button to say what it pushed; the event stream reflects it
// too. A refusal — nothing to push, or a push that fails — throws the API error,
// whose message is safe to show. Under VITE_MOCK it answers with the mockup's
// branch, caught up with its upstream.
export async function pushBranch(): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return { ...mockSnapshot.branch, ahead: 0 }
  }

  const result = await push({ throwOnError: true })

  return result.data
}
