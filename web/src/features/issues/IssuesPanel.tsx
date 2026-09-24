import { useEffect, useRef, useState, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type { Issue, IssuesPage, TaskBranch } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { StateMark } from '@/shell/StateMark.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { useMoreIssues } from './issueApi.ts'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'
import { IssueListControls } from './IssueListControls.tsx'
import { IssueStatus } from './IssueStatus.tsx'

export function IssuesPanel() {
  const snapshot = useSnapshotStore((state) => state.snapshot)

  // The shell says it is connecting until the first snapshot lands.
  if (!snapshot) {
    return null
  }

  return <IssueBrowser streamed={snapshot.issues} branches={snapshot.branches} />
}

// IssueBrowser is the issue list beside the selected issue's detail. The list is
// the page the stream pushes, then any pages loaded past it, narrowed by the
// filter; the detail is keyed on the selection, not found in the list, so an
// issue the filter hides or the stream drops stays open. Until the chosen
// view's first frame lands, the stream's page is the last view's, so none of
// it is listed under the new name. A check-out from the list says where it
// went above the list, which outlives the row's button.
function IssueBrowser({ streamed, branches }: { streamed: IssuesPage; branches: TaskBranch[] }) {
  const view = useUiStore((state) => state.view)
  const streamedView = useSnapshotStore((state) => state.view)
  const [filter, setFilter] = useState('')
  const more = useMoreIssues(view, streamed)
  const focus = useArrivalFocus()
  const outcome = useOutcome()
  const switching = streamedView !== view
  const loaded = switching ? [] : mergeIssues(streamed.issues, more.data?.pages ?? [])
  const shown = loaded.filter((issue) => matchesFilter(issue, filter))

  // loadMore reads the next page and then hands focus to what it added: the
  // button that had focus is gone once the view is fully loaded. The state it
  // sets re-renders this component, which reads the page that just landed.
  async function loadMore(): Promise<void> {
    const listed = new Set(loaded.map((issue) => issue.key))
    const result = await more.fetchNextPage({ cancelRefetch: false })
    const page = result.data?.pages.at(-1)
    if (result.isError || page === undefined) {
      return
    }

    focus.arrived(page.issues.map((issue) => issue.key).filter((key) => !listed.has(key)))
  }

  return (
    <div className="flex flex-col gap-group">
      <div className="flex flex-col gap-item">
        <IssueListControls filter={filter} onFilter={setFilter} />
        <p role="status" className="text-sm text-muted-foreground">
          {filterOutcome(filter, shown.length, loaded.length)}
        </p>
        <OutcomeLine said={outcome.said} />
      </div>
      {switching ? (
        <EmptyState>Reading the {view ?? 'default'} view…</EmptyState>
      ) : (
        <ListAndDetail
          loaded={loaded}
          shown={shown}
          branches={branches}
          more={more}
          streamed={streamed}
          focus={focus}
          onLoadMore={loadMore}
          outcome={outcome}
        />
      )}
    </div>
  )
}

interface ListAndDetailProps {
  loaded: Issue[]
  shown: Issue[]
  branches: TaskBranch[]
  more: ReturnType<typeof useMoreIssues>
  streamed: IssuesPage
  focus: ReturnType<typeof useArrivalFocus>
  onLoadMore: () => Promise<void>
  outcome: Teller
}

// ListAndDetail is the view's list, with its loading of more, beside the
// selected issue's detail — or the view's empty state when it holds none.
function ListAndDetail(props: ListAndDetailProps) {
  const { loaded, shown, branches, more, streamed, focus, onLoadMore, outcome } = props
  const selected = useUiStore((state) => state.selectedIssue)

  if (loaded.length === 0) {
    return <EmptyState>No issues match this view.</EmptyState>
  }

  return (
    <div className="flex gap-block">
      <div className="flex w-80 shrink-0 flex-col gap-group">
        {shown.length === 0 ? null : (
          <IssueRows issues={shown} branches={branches} rowRefs={focus.rows} outcome={outcome} />
        )}
        <MoreIssues
          more={more}
          streamed={streamed}
          loaded={loaded.length}
          statusLine={focus.statusLine}
          onLoadMore={onLoadMore}
        />
      </div>
      <div className="flex-1">
        {selected === null ? (
          <EmptyState>Select an issue to see its detail.</EmptyState>
        ) : (
          <IssueDetailPanel
            key={selected}
            issueKey={selected}
            listed={loaded.find((issue) => issue.key === selected)}
          />
        )}
      </div>
    </div>
  )
}

// useArrivalFocus moves focus, once a load lands, to the first issue it added
// that the list shows — or to the load's status line when it shows none — so
// focus never falls to the page when the Load more button that had it goes.
function useArrivalFocus() {
  const rows = useRef(new Map<string, HTMLButtonElement>())
  const statusLine = useRef<HTMLParagraphElement>(null)
  const [arrival, setArrival] = useState<{ keys: string[] } | null>(null)

  useEffect(() => {
    if (arrival === null) {
      return
    }

    const row = arrival.keys
      .map((key) => rows.current.get(key))
      .find((element) => element !== undefined)
    ;(row ?? statusLine.current)?.focus()
  }, [arrival])

  return {
    rows,
    statusLine,
    arrived: (keys: string[]) => {
      setArrival({ keys })
    },
  }
}

interface IssueRowsProps {
  issues: Issue[]
  branches: TaskBranch[]
  // Each listed row's button by issue key, so focus can be handed to a row.
  rowRefs: RefObject<Map<string, HTMLButtonElement>>
  outcome: Teller
}

function IssueRows({ issues, branches, rowRefs, outcome }: IssueRowsProps) {
  const selected = useUiStore((state) => state.selectedIssue)
  const selectIssue = useUiStore((state) => state.selectIssue)
  const branchesByKey = groupBranchesByKey(branches)

  return (
    <ul aria-label="Issues" className="flex flex-col gap-tight">
      {issues.map((issue) => {
        // An issue can have more than one local branch; it is on HEAD when any
        // of them is, and check-out targets the most recent one (branches
        // arrive most-recently-committed first).
        const issueBranches = branchesByKey.get(issue.key) ?? []
        const newest = issueBranches[0]
        const onHead = issueBranches.some((branch) => branch.current)

        return (
          <li key={issue.key} className="flex flex-wrap items-center gap-tight">
            <button
              ref={(element) => {
                if (element !== null) {
                  rowRefs.current.set(issue.key, element)
                }

                return () => {
                  rowRefs.current.delete(issue.key)
                }
              }}
              type="button"
              aria-current={issue.key === selected ? true : undefined}
              onClick={() => {
                selectIssue(issue.key)
              }}
              className={cn(
                'flex flex-1 flex-col gap-tight rounded-md border border-transparent px-3 py-2 text-left motion-safe:transition-colors',
                'hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
                issue.key === selected && 'border-border bg-accent',
              )}
            >
              <span className="flex items-center gap-item">
                <span className="text-xs text-muted-foreground tabular-nums">{issue.key}</span>
                <IssueStatus category={issue.status_category} label={issue.status} />
                {newest ? (
                  <span className="ml-auto flex items-center gap-1 text-xs whitespace-nowrap text-muted-foreground">
                    <StateMark state="in-flight" className="size-3 text-git" />
                    <span>in flight</span>
                  </span>
                ) : null}
              </span>
              <span className="text-sm">{issue.summary}</span>
            </button>
            {newest && !onHead ? (
              <RowCheckout branch={newest.name} issueKey={issue.key} outcome={outcome} />
            ) : null}
          </li>
        )
      })}
    </ul>
  )
}

interface MoreIssuesProps {
  more: ReturnType<typeof useMoreIssues>
  streamed: IssuesPage
  loaded: number
  statusLine: RefObject<HTMLParagraphElement | null>
  onLoadMore: () => Promise<void>
}

// MoreIssues offers the next page while the view holds issues past those
// loaded — judged from the stream's page until a page has been read, then from
// the last page read — says how many are loaded, and says why when a read
// fails. While a page is in flight the button holds rather than going
// disabled, so it keeps focus, and a second press waits on the same read
// (loadMore asks with cancelRefetch off) rather than starting it over.
function MoreIssues({ more, streamed, loaded, statusLine, onLoadMore }: MoreIssuesProps) {
  const paged = more.data !== undefined
  const remain = paged ? more.hasNextPage : streamed.issues.length < streamed.total

  return (
    <>
      <div className="flex items-center gap-item px-3 text-sm">
        <p ref={statusLine} role="status" tabIndex={-1} className="text-muted-foreground">
          {loadOutcome(loaded, streamed.total, remain, paged)}
        </p>
        {remain ? (
          <button
            type="button"
            aria-disabled={more.isFetchingNextPage}
            onClick={() => {
              void onLoadMore()
            }}
            className="rounded-sm border border-input px-2 py-1 hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
          >
            {more.isFetchingNextPage ? 'Loading more…' : 'Load more'}
          </button>
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

// loadOutcome says how much of the view is loaded: how many of how many while
// more remain, that all are once a read reached the end, and nothing when the
// stream's page held the whole view.
function loadOutcome(loaded: number, total: number, remain: boolean, paged: boolean): string {
  if (remain) {
    return `${String(loaded)} of ${String(total)} loaded`
  }

  return paged ? `All ${String(loaded)} loaded.` : ''
}

// filterOutcome says what the filter left of the loaded issues: nothing while
// there is no filter (or nothing to filter), and otherwise how many match.
function filterOutcome(filter: string, shown: number, loaded: number): string {
  if (filter === '' || loaded === 0) {
    return ''
  }

  if (shown === 0) {
    return 'No loaded issue matches the filter.'
  }

  return `${String(shown)} of ${String(loaded)} loaded issues match.`
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

// matchesFilter reports whether an issue's key or summary contains the filter,
// ignoring case — the interface's `/` filter, over the same text.
function matchesFilter(issue: Issue, filter: string): boolean {
  return `${issue.key} ${issue.summary}`.toLowerCase().includes(filter.toLowerCase())
}

// groupBranchesByKey groups the local branches by the issue they name, keeping
// their order (most-recently-committed first) so the first is the most recent.
function groupBranchesByKey(branches: TaskBranch[]): Map<string, TaskBranch[]> {
  const byKey = new Map<string, TaskBranch[]>()
  for (const branch of branches) {
    const existing = byKey.get(branch.issue_key)
    if (existing) {
      existing.push(branch)
    } else {
      byKey.set(branch.issue_key, [branch])
    }
  }

  return byKey
}

interface RowCheckoutProps {
  branch: string
  issueKey: string
  outcome: Teller
}

// RowCheckout switches to an in-flight issue's branch from the list, so moving
// between tasks does not need the detail panel first. Where it went is said in
// the panel's outcome; a refusal — a dirty tree — is shown inline.
function RowCheckout({ branch, issueKey, outcome }: RowCheckoutProps) {
  const { state, error, run } = useAsyncAction(() => checkoutBranch(branch), {
    fallback:
      'The branch was not checked out. Try again, or switch to it from a terminal to see why.',
    done: (checkedOut) => `Checked out ${checkedOut.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <>
      <button
        type="button"
        aria-label={`Check out ${issueKey}`}
        disabled={state === 'running'}
        onClick={() => {
          void run()
        }}
        className="shrink-0 rounded-sm border border-input px-2 py-1 text-xs hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-disabled disabled:text-disabled-foreground"
      >
        {state === 'running' ? 'Switching…' : 'Check out'}
      </button>
      {state === 'error' ? (
        <p role="alert" className="basis-full text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </>
  )
}
