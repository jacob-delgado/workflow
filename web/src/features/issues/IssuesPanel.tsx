import type {
  Issue,
  IssuesPage,
  Problem,
  TaskBranch,
  TasksSummary,
} from '@/api/generated/types.gen.ts'
import { useLiveSnapshot, useSnapshotStore } from '@/api/snapshot.ts'
import { EmptyState } from '@/lib/EmptyState.tsx'
import { FilterChips } from '@/lib/FilterChips.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { Reading, ReadFailure } from '@/lib/Status.tsx'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'
import { marksOf, placeChoices, samePlace, togglePlace } from './issuePlaces.ts'
import { IssueListControls } from './list/IssueListControls.tsx'
import { filterOutcome, narrowed, useNarrowing } from './list/issueNarrowing.ts'
import { MoreIssues, useIssuePaging, type IssuePaging } from './list/IssuePaging.tsx'
import { groupBranchesByKey, IssueRows, type RowFacts } from './list/IssueRows.tsx'

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
  const switching = useSnapshotStore((state) => state.view) !== view
  const { filter, places, narrow } = useNarrowing(view)
  const paging = useIssuePaging(view, streamed, switching)
  const outcome = useOutcome()
  const facts: RowFacts = { branchesByKey: groupBranchesByKey(branches), tasks }
  const marksFor = (issue: Issue) => marksOf(issue, facts.branchesByKey, tasks)
  const { loaded } = paging
  const shown = narrowed(loaded, { filter, places }, marksFor)

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
          paging={paging}
          shown={shown}
          facts={facts}
          unread={unread}
          outcome={outcome}
        />
      )}
    </div>
  )
}

interface ListAndDetailProps {
  paging: IssuePaging
  shown: Issue[]
  facts: RowFacts
  unread: Problem | null
  outcome: Teller
}

// ListAndDetail is the view's list, with its loading of more, beside the
// selected issue's detail — or the view's empty state when it holds none, or
// why it could not be read, which is not an empty view. Below lg the list sits
// over the detail, in a pane of a few rows, so the detail it opens starts just
// beneath; from lg the two sit side by side and fill the window's height, each
// scrolling on its own. Each pane keeps a few pixels inside the edge it
// scrolls within, so a row's focus ring is not clipped. The detail's pane
// opens each issue at its top, as a section opens, and is positioned, as the
// content around it is, so the text it keeps for a screen reader scrolls and
// clips with it rather than growing the content.
function ListAndDetail({ paging, shown, facts, unread, outcome }: ListAndDetailProps) {
  const selected = useUiStore((state) => state.selectedIssue)

  if (unread !== null) {
    return (
      <ReadFailure
        unread="The issues could not be read"
        notSetUp="The issue tracker"
        problem={unread}
      />
    )
  }

  if (paging.loaded.length === 0) {
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
          <IssueRows issues={shown} facts={facts} rowRefs={paging.rows} outcome={outcome} />
        )}
        <MoreIssues paging={paging} />
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
            listed={paging.loaded.find((issue) => issue.key === selected)}
          />
        )}
      </div>
    </div>
  )
}
