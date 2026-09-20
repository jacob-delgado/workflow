import { commit } from '@/api/generated'
import type { CommitRequest } from '@/api/generated/types.gen.ts'

// commitChanges commits the staged changes with a Conventional Commit message.
// On success the event stream reflects the new commit and the cleared index; a
// refusal — nothing staged, an invalid message, a failing hook — throws the API
// error, whose message is safe to show. Under VITE_MOCK it is a no-op.
export async function commitChanges(message: CommitRequest): Promise<void> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return
  }

  await commit({ body: message, throwOnError: true })
}
