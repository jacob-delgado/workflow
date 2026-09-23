import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import { listIssues } from '@/api/generated'
import {
  getIssueOptions,
  listViewsOptions,
  listViewsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { IssueDetail, IssuesPage, ViewList } from '@/api/generated/types.gen.ts'

// The VITE_MOCK check is read inline (not via a helper) so Vite statically
// replaces it and code-splits the dev fixture out of a production build, while
// tests can still stub it at runtime.

// How long an issue read in full is taken as current. The event stream carries
// only the list's slim issues, so nothing pushes a newer detail: past this, the
// detail is read again the next time the issue is opened.
const issueFreshFor = 60_000

// useIssue reads one issue in full — description, comments, reporter, assignee
// and its link in the tracker. Under VITE_MOCK it serves a fixture built from
// the mock snapshot's issue, so the detail works with no backend.
export function useIssue(key: string) {
  const options = { ...getIssueOptions({ path: { key } }), staleTime: issueFreshFor }

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<IssueDetail> => {
            const { mockIssueDetail } = await import('@/dev/mockIssues.ts')

            return mockIssueDetail(key)
          },
        }
      : options,
  )
}

// useViews reads the configured issue views, in order — the ones the stream can
// carry. Saving the configuration refreshes it (see useSaveConfig). Under
// VITE_MOCK it serves the fixture's views.
export function useViews() {
  const options = listViewsOptions()

  return useQuery(
    import.meta.env.VITE_MOCK === 'true'
      ? {
          ...options,
          queryFn: async (): Promise<ViewList> => {
            const { mockViews } = await import('@/dev/mockIssues.ts')

            return mockViews
          },
        }
      : options,
  )
}

// useRefreshViews returns a function that reads the configured views again,
// for when the server turns out not to have one this tab still offers.
export function useRefreshViews(): () => void {
  const queryClient = useQueryClient()

  return () => {
    void queryClient.invalidateQueries({ queryKey: listViewsQueryKey() })
  }
}

// useMoreIssues pages through a view's issues past the page the stream pushes.
// Nothing is read until fetchNextPage asks: the first read starts where the
// stream's page ends, and each after it where the last one ended. The pages are
// kept per view, so another view starts with none of this one's.
export function useMoreIssues(view: string | null, streamed: IssuesPage) {
  return useInfiniteQuery({
    queryKey: ['moreIssues', view],
    queryFn: async ({ pageParam, signal }): Promise<IssuesPage> => {
      const query = view === null ? { start_at: pageParam } : { view, start_at: pageParam }
      const result = await listIssues({ query, signal, throwOnError: true })

      return result.data
    },
    initialPageParam: streamed.start_at + streamed.issues.length,
    getNextPageParam: nextStart,
    enabled: false,
  })
}

// nextStart is where the page after this one starts, or undefined when this
// one reached the end of the view.
function nextStart(page: IssuesPage): number | undefined {
  const next = page.start_at + page.issues.length
  if (page.issues.length === 0 || next >= page.total) {
    return undefined
  }

  return next
}
