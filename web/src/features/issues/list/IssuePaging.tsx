import { useEffect, useRef, useState, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Issue, IssuesPage } from '@/api/generated/types.gen.ts'
import { useMoreIssues } from '@/features/issues/issueApi.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { Button } from '@/lib/Button.tsx'

// IssuePaging is the view's issues loaded so far, and how to load more.
export type IssuePaging = ReturnType<typeof useIssuePaging>

// useIssuePaging is the stream's page followed by any pages loaded past it —
// none while a switch of view is under way, since the stream's page is still
// the last view's — and how to load the next. A load hands focus to the first
// issue it added that the list shows, or to the load's status line when it
// shows none, so focus never falls to the page when the Load more button that
// had it goes once the view is fully loaded.
export function useIssuePaging(view: string | null, streamed: IssuesPage, switching: boolean) {
  const more = useMoreIssues(view, streamed)
  const rows = useRef(new Map<string, HTMLButtonElement>())
  const statusLine = useRef<HTMLParagraphElement>(null)
  const arrived = useArrivalFocus(rows, statusLine)
  const loaded = switching ? [] : mergeIssues(streamed.issues, more.data?.pages ?? [])

  // loadMore reads the next page, then hands focus to what it added. The state
  // arrived sets re-renders the list, which reads the page that just landed.
  async function loadMore(): Promise<void> {
    const listed = new Set(loaded.map((issue) => issue.key))
    const result = await more.fetchNextPage({ cancelRefetch: false })
    const page = result.data?.pages.at(-1)
    if (result.isError || page === undefined) {
      return
    }

    arrived(page.issues.map((issue) => issue.key).filter((key) => !listed.has(key)))
  }

  return { streamed, loaded, more, rows, statusLine, loadMore }
}

// useArrivalFocus moves focus, once a load lands, to the first of the keys it
// is handed whose row is listed, or to the status line when none is.
function useArrivalFocus(
  rows: RefObject<Map<string, HTMLButtonElement>>,
  statusLine: RefObject<HTMLParagraphElement | null>,
) {
  const [arrival, setArrival] = useState<{ keys: string[] } | null>(null)

  useEffect(() => {
    if (arrival === null) {
      return
    }

    const row = arrival.keys
      .map((key) => rows.current.get(key))
      .find((element) => element !== undefined)
    ;(row ?? statusLine.current)?.focus()
  }, [arrival, rows, statusLine])

  return (keys: string[]) => {
    setArrival({ keys })
  }
}

// MoreIssues offers the next page while the view holds issues past those
// loaded — judged from the stream's page until a page has been read, then from
// the last page read — says how many are loaded, and says why when a read
// fails. While a page is in flight the button holds rather than going
// disabled, so it keeps focus, and a second press waits on the same read
// (loadMore asks with cancelRefetch off) rather than starting it over.
export function MoreIssues({ paging }: { paging: IssuePaging }) {
  const { streamed, loaded, more, statusLine, loadMore } = paging
  const paged = more.data !== undefined
  const remain = paged ? more.hasNextPage : streamed.issues.length < streamed.total
  const shortcut = useShortcutProps<HTMLButtonElement>('load-more')

  return (
    <>
      <div className="flex items-center gap-item px-3 text-sm">
        <p ref={statusLine} role="status" tabIndex={-1} className="text-muted-foreground">
          {loadOutcome(loaded.length, streamed.total, remain)}
        </p>
        {remain ? (
          <Button
            {...shortcut}
            variant="secondary"
            size="sm"
            held={more.isFetchingNextPage}
            onClick={() => {
              void loadMore()
            }}
          >
            {more.isFetchingNextPage ? 'Loading more…' : 'Load more'}
          </Button>
        ) : null}
      </div>
      {more.isError ? (
        <p role="alert" className="px-3 text-sm text-destructive">
          {apiErrorMessage(
            more.error,
            'More issues could not be loaded. Press Load more to try again.',
          )}
        </p>
      ) : null}
    </>
  )
}

// loadOutcome says how much of the view is loaded, in one form whether more
// remain or not — also when the stream's page held the whole view, since the
// list's pane can hold fewer rows than that.
function loadOutcome(loaded: number, total: number, remain: boolean): string {
  return `${String(loaded)} of ${String(remain ? total : loaded)} loaded.`
}

// mergeIssues is the stream's page followed by the pages loaded past it, each
// issue once. The stream re-reads its page every few seconds while a loaded page
// stays as it was read, so an issue that moved between them is in both.
function mergeIssues(streamed: Issue[], pages: IssuesPage[]): Issue[] {
  const seen = new Set(streamed.map((issue) => issue.key))
  const merged = [...streamed]
  for (const issue of pages.flatMap((page) => page.issues)) {
    if (!seen.has(issue.key)) {
      seen.add(issue.key)
      merged.push(issue)
    }
  }

  return merged
}
