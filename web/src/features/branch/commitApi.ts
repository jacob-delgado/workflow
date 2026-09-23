import { commit } from '@/api/generated'
import type { Branch, CommitRequest } from '@/api/generated/types.gen.ts'

// commitChanges commits the staged changes with a Conventional Commit message
// and returns the branch now carrying the commit, for the form to say which it
// made; the event stream reflects the cleared index too. A refusal — nothing
// staged, an invalid message, a failing hook — throws the API error, whose
// message is safe to show. Under VITE_MOCK it answers with the mockup's branch,
// the new commit on top.
export async function commitChanges(message: CommitRequest): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')
    const hash = 'd4e5f6a7'
    const subject = `${message.type}: ${message.subject}`

    return {
      ...mockSnapshot.branch,
      head: hash,
      commits: [...mockSnapshot.branch.commits, { hash, subject }],
    }
  }

  const result = await commit({ body: message, throwOnError: true })

  return result.data
}
