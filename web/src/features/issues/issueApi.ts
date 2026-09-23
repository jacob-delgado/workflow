import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  getIssueOptions,
  listViewsOptions,
  listViewsQueryKey,
} from '@/api/generated/@tanstack/react-query.gen.ts'
import type { IssueDetail, ViewList } from '@/api/generated/types.gen.ts'

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
