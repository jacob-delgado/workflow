import { push } from '@/api/generated'

// pushBranch publishes the current branch to its remote. On success the event
// stream reflects the published branch; a refusal — nothing to push, or a push
// that fails — throws the API error, whose message is safe to show. Under
// VITE_MOCK it is a no-op.
export async function pushBranch(): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await push({ throwOnError: true })
}
