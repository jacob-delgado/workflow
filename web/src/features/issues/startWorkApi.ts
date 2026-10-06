import { createBranch, createWorktree } from '@/api/generated'
import type { Branch, CreatedWorktree } from '@/api/generated/types.gen.ts'

// startWork creates and switches to a branch for a not-started issue — the write
// half of starting work on it — after fetching origin unless fetch is false,
// and returns the new branch, for the button to say what it made; the event
// stream reflects it too. A refusal (a branch already exists, a fetch that
// failed) throws the API error, whose message is safe to show through
// apiErrorMessage. Under VITE_MOCK it answers with a fresh, unpublished branch
// named for the issue.
export async function startWork(issueKey: string, fetch: boolean): Promise<Branch> {
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

  const result = await createBranch({ body: { issue_key: issueKey, fetch }, throwOnError: true })

  return result.data
}

// startWorkInWorktree creates a branch for a not-started issue in a new
// worktree beside the repository, leaving where the server works as it is,
// and returns where the worktree is, for the switch to it to be offered. It
// fetches first and is refused as startWork is. Under VITE_MOCK it answers
// with a worktree beside the mockup's repository.
export async function startWorkInWorktree(
  issueKey: string,
  fetch: boolean,
): Promise<CreatedWorktree> {
  if (import.meta.env.VITE_MOCK === 'true') {
    return {
      dir: `/home/ana/src/api-feat-${issueKey}`,
      shown: `~/src/api-feat-${issueKey}`,
      branch: `feat/${issueKey}`,
    }
  }

  const result = await createWorktree({ body: { issue_key: issueKey, fetch }, throwOnError: true })

  return result.data
}
