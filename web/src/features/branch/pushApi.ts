import { push } from '@/api/generated'
import type { Branch } from '@/api/generated/types.gen.ts'

// pushBranch publishes the current branch to its remote and returns it as
// published, for the button to say what it pushed; the event stream reflects it
// too. A refusal — nothing to push, or a push that fails — throws the API error,
// whose message is safe to show.
export async function pushBranch(): Promise<Branch> {
  const result = await push({ throwOnError: true })

  return result.data
}
