import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import { listIssues } from '@/api/generated'
import {
  getIssueOptions,
  listViewsOptions,
  listViewsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { IssuesPage } from '@/api/generated/types.gen.ts'
import { freshFor } from '@/queryClient.ts'

// useIssue reads one issue in full — description, comments, reporter, assignee
// and its link in the tracker.
export function useIssue(key: string) {
  // The event stream carries only the list's slim issues, so nothing pushes a
  // newer detail: past freshFor, it is read again the next time the issue opens.
  return useQuery({ ...getIssueOptions({ path: { key } }), staleTime: freshFor })
}

// useViews reads the configured issue views, in order — the ones the stream can
// carry. Saving the configuration, reloading it, and a read of it that may have
// taken up an edit refresh it (see configApi.ts).
export function useViews() {
  return useQuery(listViewsOptions())
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
