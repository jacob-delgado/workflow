import { useEffect, useRef, useState, type RefObject } from 'react'
import { apiErrorMessage } from '@/api/apiError.ts'
import type {
  Issue,
  IssuesPage,
  Problem,
  TaskBranch,
  TasksSummary,
} from '@/api/generated/types.gen.ts'
import { useLiveSnapshot, useSnapshotStore } from '@/api/snapshot.ts'
import { useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { issueTaskMark, linkedTo } from '@/features/tasks/taskWords.ts'
import { Button } from '@/lib/Button.tsx'
import { FilterChips } from '@/lib/FilterChips.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { Reading, ReadFailure } from '@/lib/Status.tsx'
import { useAsyncAction } from '@/lib/useAsyncAction.ts'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { EmptyState } from '@/shell/EmptyState.tsx'
import { StateMark } from '@/shell/StateMark.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { useMoreIssues } from './issueApi.ts'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'
import { IssueListControls } from './IssueListControls.tsx'
import { IssueStatus } from './IssueStatus.tsx'
import {
  admits,
  marksOf,
  placeChoices,
  samePlace,
  shownKey,
  togglePlace,
  type Place,
} from './issuePlaces.ts'

export function IssuesPanel() {
  const snapshot = useLiveSnapshot()

  return (
    <IssueBrowser
      streamed={snapshot.issues}
      unread={snapshot.problems?.issues ?? null}
      branches={snapshot.branches}
      tasks={snapshot.tasks}
    />
  )
}

interface IssueBrowserProps {
  streamed: IssuesPage
  // Why the view's first page could not be read, or null when it was.
  unread: Problem | null
  branches: TaskBranch[]
  tasks: TasksSummary
}

// IssueBrowser is the issue list beside the selected issue's detail. The list is
// the page the stream pushes, then any pages loaded past it, narrowed by the
// filter; the detail is keyed on the selection, not found in the list, so an
// issue the filter hides or the stream drops stays open. Until the chosen
// view's first frame lands, the stream's page is the last view's, so none of
// it is listed under the new name. A check-out from the list says where it
// went above the list, which outlives the row's button.
function IssueBrowser({ streamed, unread, branches, tasks }: IssueBrowserProps) {
  const view = useUiStore((state) => state.view)
  const streamedView = useSnapshotStore((state) => state.view)
  const { filter, places, narrow } = useNarrowing(view)
  const more = useMoreIssues(view, streamed)
  const focus = useArrivalFocus()
  const outcome = useOutcome()
  const switching = streamedView !== view
  const loaded = switching ? [] : mergeIssues(streamed.issues, more.data?.pages ?? [])
  const branchKeys = new Set(branches.map((branch) => branch.issue_key))
  const marksFor = (issue: Issue) => marksOf(issue, branchKeys, tasks)
  const shown = loaded.filter(
    (issue) => matchesFilter(issue, filter) && admits(places, issue, marksFor(issue)),
  )

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
    <div className="flex flex-col gap-group lg:min-h-0 lg:flex-1">
      <div className={cn('flex flex-col gap-item', contentMeasure)}>
        <IssueListControls
          filter={filter}
          onFilter={(typed) => {
            narrow({ filter: typed })
          }}
        />
        <FilterChips
          label="Filter"
          choices={placeChoices(loaded, marksFor, places)}
          isPicked={(place) => places.some((picked) => samePlace(picked, place))}
          nameOf={(place) => place.name}
          keyOf={(place) => `${place.kind}:${place.name}`}
          onToggle={(place) => {
            narrow({ places: togglePlace(places, place) })
          }}
        />
        <p role="status" className="text-sm text-muted-foreground">
          {filterOutcome(filter !== '' || places.length > 0, shown.length, loaded.length)}
        </p>
        <p role="status" className="text-sm text-destructive empty:sr-only">
          {streamed.unavailable.length > 0 ? `Not read: ${streamed.unavailable.join(', ')}.` : ''}
        </p>
        <OutcomeLine said={outcome.said} />
      </div>
      {switching ? (
        <Reading>Reading the {view ?? 'default'} view…</Reading>
      ) : (
        <ListAndDetail
          loaded={loaded}
          shown={shown}
          branches={branches}
          tasks={tasks}
          more={more}
          streamed={streamed}
          unread={unread}
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
  tasks: TasksSummary
  more: ReturnType<typeof useMoreIssues>
  streamed: IssuesPage
  unread: Problem | null
  focus: ReturnType<typeof useArrivalFocus>
  onLoadMore: () => Promise<void>
  outcome: Teller
}

// ListAndDetail is the view's list, with its loading of more, beside the
// selected issue's detail — or the view's empty state when it holds none, or
// why it could not be read, which is not an empty view. Below
// lg the list sits over the detail, in a pane of a few rows, so the detail it
// opens starts just beneath; from lg the two sit side by side and fill the
// window's height, each scrolling on its own. Each pane keeps a few pixels
// inside the edge it scrolls within, so a row's focus ring is not clipped.
// The detail's pane opens each issue at its top, as a section opens, and is
// positioned, as the content around it is, so the text it keeps for a screen
// reader scrolls and clips with it rather than growing the content.
function ListAndDetail(props: ListAndDetailProps) {
  const { loaded, shown, branches, tasks, more, streamed, unread, focus, onLoadMore, outcome } =
    props
  const selected = useUiStore((state) => state.selectedIssue?.key ?? null)

  if (unread !== null) {
    return (
      <ReadFailure
        unread="The issues could not be read"
        notSetUp="The issue tracker"
        problem={unread}
      />
    )
  }

  if (loaded.length === 0) {
    return <EmptyState>No issues match this view.</EmptyState>
  }

  return (
    <div className="flex flex-col gap-block lg:min-h-0 lg:flex-1 lg:flex-row">
      <div
        className={cn(
          'flex flex-col gap-group lg:min-h-0 lg:w-80 lg:shrink-0 xl:w-112',
          contentMeasure,
        )}
      >
        {shown.length === 0 ? null : (
          <IssueRows
            issues={shown}
            branches={branches}
            tasks={tasks}
            rowRefs={focus.rows}
            outcome={outcome}
          />
        )}
        <MoreIssues
          more={more}
          streamed={streamed}
          loaded={loaded.length}
          statusLine={focus.statusLine}
          onLoadMore={onLoadMore}
        />
      </div>
      <div
        key={selected ?? ''}
        className={cn('relative min-w-0 flex-1 lg:overflow-y-auto lg:px-1', contentMeasure)}
      >
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
  // tasks is what the stream carries of your tasks, whose marks the rows draw
  // once it says Taskwarrior can be asked.
  tasks: TasksSummary
  // Each listed row's button by issue key, so focus can be handed to a row.
  rowRefs: RefObject<Map<string, HTMLButtonElement>>
  outcome: Teller
}

function IssueRows({ issues, branches, tasks, rowRefs, outcome }: IssueRowsProps) {
  const selected = useUiStore((state) => state.selectedIssue?.key ?? null)
  const selectIssue = useUiStore((state) => state.selectIssue)
  const branchesByKey = groupBranchesByKey(branches)
  const pane = useSelectedRowInView(rowRefs, selected)

  return (
    <ul
      ref={pane}
      aria-label="Issues"
      className="flex max-h-80 min-h-0 flex-col gap-tight overflow-y-auto rounded-lg border border-border p-1 lg:max-h-none lg:rounded-none lg:border-0"
    >
      {issues.map((issue) => {
        // An issue can have more than one local branch; it is on HEAD when any
        // of them is, and check-out targets the most recent one (branches
        // arrive most-recently-committed first).
        const issueBranches = branchesByKey.get(issue.key) ?? []
        const newest = issueBranches[0]
        const onHead = issueBranches.some((branch) => branch.current)
        const taskMark = tasks.available
          ? issueTaskMark(linkedTo(tasks.linked, issue.key))
          : undefined

        return (
          <li
            key={issue.key}
            className={cn(
              'flex flex-col rounded-md border border-transparent hover:bg-accent motion-safe:transition-colors',
              issue.key === selected && 'border-border bg-accent',
            )}
          >
            {/* Not a Button: a row of the list that selects its issue, drawn as the row. */}
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
                selectIssue(issue)
              }}
              className="flex flex-col gap-tight rounded-md px-3 py-2 text-left focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            >
              <span className="flex flex-wrap items-center gap-item">
                <span className="text-xs text-muted-foreground tabular-nums">
                  {shownKey(issue)}
                </span>
                <IssueStatus category={issue.status_category} label={issue.status} />
                <RowMarks inFlight={newest !== undefined} taskMark={taskMark} />
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

// RowMarks are what a row marks at its end: the issue in flight, when a branch
// names it, and how its tasks stand, when Taskwarrior can be asked and a task
// is linked to it — each by its mark, beside the words for it. They sit
// together, so where the row is too narrow for them they move to a line of
// their own as one.
function RowMarks({
  inFlight,
  taskMark,
}: {
  inFlight: boolean
  taskMark: ReturnType<typeof issueTaskMark>
}) {
  if (!inFlight && taskMark === undefined) {
    return null
  }

  return (
    <span className="ml-auto flex items-center gap-item text-xs whitespace-nowrap text-muted-foreground">
      {inFlight ? (
        <span className="flex items-center gap-1">
          <StateMark state="in-flight" className="size-3 text-git" />
          <span>in flight</span>
        </span>
      ) : null}
      {taskMark === undefined ? null : (
        <span className="flex items-center gap-1">
          <StateMark state={taskMark.state} className="size-3 text-taskwarrior" />
          <span>{taskMark.words}</span>
        </span>
      )}
    </span>
  )
}

// useSelectedRowInView brings the selected issue's row into view in the list's
// pane, scrolling the pane alone and only as far as it must. The selection
// outlives a change of section and the pane's scroll does not, so the list
// would reopen at its top with the issue its detail shows out of sight.
function useSelectedRowInView(
  rowRefs: RefObject<Map<string, HTMLButtonElement>>,
  selected: string | null,
) {
  const pane = useRef<HTMLUListElement>(null)

  useEffect(() => {
    const list = pane.current
    const row = selected === null ? undefined : rowRefs.current.get(selected)
    if (list === null || row === undefined) {
      return
    }

    const top = row.getBoundingClientRect().top - list.getBoundingClientRect().top - list.clientTop
    const bottom = top + row.offsetHeight
    const inset = parseFloat(getComputedStyle(list).paddingTop)
    if (top < 0) {
      list.scrollTop += top - inset
    } else if (bottom > list.clientHeight) {
      list.scrollTop += bottom - list.clientHeight + inset
    }
  }, [rowRefs, selected])

  return pane
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
  const shortcut = useShortcutProps<HTMLButtonElement>('load-more')

  return (
    <>
      <div className="flex items-center gap-item px-3 text-sm">
        <p ref={statusLine} role="status" tabIndex={-1} className="text-muted-foreground">
          {loadOutcome(loaded, streamed.total, remain)}
        </p>
        {remain ? (
          <Button
            {...shortcut}
            variant="secondary"
            size="sm"
            aria-disabled={more.isFetchingNextPage}
            onClick={() => {
              void onLoadMore()
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

// Narrowing is how the list is narrowed in a view: the search typed and the
// places picked.
interface Narrowing {
  view: string | null
  filter: string
  places: Place[]
}

// useNarrowing is the search and the places picked, which belong to the view
// they were typed and picked in: another view — or the same one chosen again —
// starts with neither, as the terminal's nextIssueView seeds a fresh list,
// search and all. The user asked for a view, not a narrowed one.
function useNarrowing(view: string | null) {
  const [narrowing, setNarrowing] = useState<Narrowing>({ view, filter: '', places: [] })
  // Set while rendering, as React has state follow a prop, so the first frame
  // of the new view is already drawn unnarrowed.
  if (narrowing.view !== view) {
    setNarrowing({ view, filter: '', places: [] })
  }

  const current = narrowing.view === view ? narrowing : { view, filter: '', places: [] }

  return {
    filter: current.filter,
    places: current.places,
    narrow: (change: Partial<Omit<Narrowing, 'view'>>) => {
      setNarrowing({ ...current, ...change })
    },
  }
}

// filterOutcome says what the filter and the places left of the loaded issues:
// nothing while neither narrows them (or there is nothing to narrow), and
// otherwise how many match.
function filterOutcome(narrowed: boolean, shown: number, loaded: number): string {
  if (!narrowed || loaded === 0) {
    return ''
  }

  if (shown === 0) {
    return 'No loaded issue matches the filter.'
  }

  return `${String(shown)} of ${String(loaded)} loaded issues ${shown === 1 ? 'matches' : 'match'}.`
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
  return `${shownKey(issue)} ${issue.summary}`.toLowerCase().includes(filter.toLowerCase())
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
// the panel's outcome; a refusal — a dirty tree — is shown inline. It is the
// row's bottom line, inside the row's box and under its summary, so a row that
// offers it is as wide as one that does not and the list keeps one right edge.
function RowCheckout({ branch, issueKey, outcome }: RowCheckoutProps) {
  const { state, error, run } = useAsyncAction(() => checkoutBranch(branch), {
    fallback:
      'The branch was not switched to. Try again, or switch to it from a terminal to see why.',
    done: (switched) => `Switched to ${switched.name}.`,
    onStart: outcome.clear,
    onDone: outcome.say,
  })

  return (
    <div className="flex flex-col items-start gap-tight px-3 pb-2">
      <Button
        variant="secondary"
        size="sm"
        aria-label={
          state === 'running'
            ? `Switching branch for ${issueKey}…`
            : `Switch branch for ${issueKey}`
        }
        held={state === 'running'}
        onClick={() => {
          void run()
        }}
      >
        {state === 'running' ? 'Switching…' : 'Switch branch'}
      </Button>
      {state === 'error' ? (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      ) : null}
    </div>
  )
}
