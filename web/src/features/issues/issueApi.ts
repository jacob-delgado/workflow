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
// Nothing is read until fetchNextPage asks: the first read follows the last
// issue of the stream's page as it is then, and each after it the last issue of
// the page before, wherever the view has moved it since, so an issue joining or
// leaving the view ahead of a page neither skips nor repeats a row. The pages
// are kept per view, so another view starts with none of this one's.
export function useMoreIssues(view: string | null, streamed: IssuesPage) {
  return useInfiniteQuery({
    queryKey: ['moreIssues', view],
    queryFn: async ({ pageParam, signal }): Promise<IssuesPage> => {
      const query = view === null ? { after: pageParam } : { view, after: pageParam }
      const result = await listIssues({ query, signal, throwOnError: true })

      return result.data
    },
    initialPageParam: streamed.next,
    getNextPageParam: nextAfter,
    enabled: false,
  })
}

// nextAfter is the cursor the page after this one is asked with, or undefined
// when this one reached the end of the view. A page can come back empty and
// still name one: its cursor's issue moved to the end of the page read after
// it, and the view goes on past that page.
function nextAfter(page: IssuesPage): string | undefined {
  return page.next === '' ? undefined : page.next
}
