import { useQueryClient } from '@tanstack/react-query'
import { addComment } from '@/api/generated'
import { getIssueQueryKey } from '@/api/generated/@tanstack/react-query.gen.ts'

// usePostComment returns a function that posts a comment on a Jira issue and
// then reads the issue again, so the thread shows the comment as Jira keeps
// it, beside any another hand wrote meanwhile. A refusal throws the API error,
// whose message is safe to show.
export function usePostComment(issueKey: string): (text: string) => Promise<void> {
  const queryClient = useQueryClient()

  return async (text: string) => {
    await addComment({ path: { key: issueKey }, body: { text }, throwOnError: true })

    await queryClient.invalidateQueries({ queryKey: getIssueQueryKey({ path: { key: issueKey } }) })
  }
}
