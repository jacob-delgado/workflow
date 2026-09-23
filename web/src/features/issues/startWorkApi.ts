import { createBranch } from '@/api/generated'
import type { Branch } from '@/api/generated/types.gen.ts'

// startWork creates and switches to a branch for a not-started issue — the write
// half of starting work on it — and returns the new branch, for the button to
// say what it made; the event stream reflects it too. A refusal (a branch
// already exists) throws the API error, whose message is safe to show through
// apiErrorMessage. Under VITE_MOCK it answers with a fresh, unpublished branch
// named for the issue.
export async function startWork(issueKey: string): Promise<Branch> {
  if (import.meta.env.VITE_MOCK === 'true') {
    const { mockSnapshot } = await import('@/dev/mockSnapshot.ts')

    return {
      ...mockSnapshot.branch,
      name: `feat/${issueKey}`,
      upstream: '',
      ahead: 0,
      behind: 0,
      commits: [],
    }
  }

  const result = await createBranch({ body: { issue_key: issueKey }, throwOnError: true })

  return result.data
}
