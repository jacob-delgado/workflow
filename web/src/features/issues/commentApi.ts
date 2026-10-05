import { useQueryClient } from '@tanstack/react-query'
import { addComment } from '@/api/generated'
import { getIssueQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and drops the SDK call from a production build's mock path, while
// tests can still stub the module.

// usePostComment returns a function that posts a comment on a Jira issue and
// then reads the issue again, so the thread shows the comment as Jira keeps
// it, beside any another hand wrote meanwhile. A refusal throws the API error,
// whose message is safe to show. Under VITE_MOCK the post is a no-op.
export function usePostComment(issueKey: string): (text: string) => Promise<void> {
  const queryClient = useQueryClient()

  return async (text: string) => {
    if (import.meta.env.VITE_MOCK !== 'true') {
      await addComment({ path: { key: issueKey }, body: { text }, throwOnError: true })
    }

    await queryClient.invalidateQueries({ queryKey: getIssueQueryKey({ path: { key: issueKey } }) })
  }
}
