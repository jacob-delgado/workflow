import type { ReactElement, ReactNode } from 'react'
import type { Issue, Problem } from '@/api/generated/types.gen.ts'
import { EmptyState } from '@/lib/EmptyState.tsx'
import { FilterChips } from '@/lib/FilterChips.tsx'
import { OutcomeLine, useOutcome, type Teller } from '@/lib/Outcome.tsx'
import { failureGround, Reading, ReadFailure } from '@/lib/Status.tsx'
import { cn, contentMeasure } from '@/lib/utils.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { IssueDetailPanel } from './IssueDetailPanel.tsx'
import { marksOf, placeChoices, samePlace, togglePlace, type Place } from './issuePlaces.ts'
import { StatusMark } from './IssueStatus.tsx'
import { IssueListControls } from './list/IssueListControls.tsx'
import { filterOutcome, narrowed, useNarrowing } from './list/issueNarrowing.ts'
import { useListedIssues, type ListedIssues } from './list/listedIssues.ts'
import { MoreIssues, useIssuePaging, type IssuePaging } from './list/IssuePaging.tsx'
import { groupBranchesByKey, IssueRows, rowMarksOf, type RowFacts } from './list/IssueRows.tsx'

export function IssuesPanel() {
  const view = useUiStore((state) => state.view)

  return <IssueBrowser view={view} listed={useListedIssues(view)} />
}

interface IssueBrowserProps {
  view: string | null
  listed: ListedIssues
}

// IssueBrowser is the issue list beside the selected issue's detail. The list is
// the page the stream pushes, then any pages loaded past it, narrowed by the
// filter; the detail is keyed on the selection, not found in the list, so an
// issue the filter hides or the stream drops stays open. Until the chosen
// view's first frame lands, the stream's page is the last view's, so none of
// it is listed under the new name. A check-out from the list says where it
// went above the list, which outlives the row's button. Before the view's
// first frame lands, the list is the one the store kept for the view, said
// to be read again, with no more to load until Jira answers: the kept list's
// total is only what was kept.
function IssueBrowser({ view, listed }: IssueBrowserProps) {
  const { page: streamed, unread, branches, tasks } = listed
  const switching = listed.from === 'switching'
  const { filter, places, narrow } = useNarrowing(view)
  const paging = useIssuePaging(view, streamed, switching)
  const outcome = useOutcome()
  const facts: RowFacts = { branchesByKey: groupBranchesByKey(branches), tasks }
  const marksFor = (issue: Issue) => marksOf(issue, facts.branchesByKey, tasks)
  const { loaded } = paging
  const shown = narrowed(loaded, { filter, places }, marksFor)
  const drawn = placeMarks(loaded, facts)

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
          keyOf={placeKey}
          markOf={(place) => drawn.get(placeKey(place))}
          onToggle={(place) => {
            narrow({ places: togglePlace(places, place) })
          }}
        />
        <p role="status" className="text-sm text-muted-foreground">
          {filterOutcome(filter !== '' || places.length > 0, shown.length, loaded.length)}
        </p>
        <p role="status" className={cn(failureGround, 'empty:sr-only')}>
          {streamed.unavailable.length > 0 ? `Not read: ${streamed.unavailable.join(', ')}.` : ''}
        </p>
        <OutcomeLine said={outcome.said} />
        {listed.from === 'seed' ? (
          <Reading>Reading Jira… these are the issues it listed last time.</Reading>
        ) : null}
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
          more={listed.from === 'stream' ? <MoreIssues paging={paging} /> : null}
        />
      )}
    </div>
  )
}

// placeKey tells a place apart from every other: its kind and its name.
function placeKey(place: Place): string {
  return `${place.kind}:${place.name}`
}

// placeMarks is the mark the rows draw for each place a loaded issue is in, by
// the place's key: a status by its category's mark, and a row's mark by its
// words. A place no loaded issue is in, and a mark no row draws, has none.
function placeMarks(loaded: Issue[], facts: RowFacts): Map<string, ReactElement> {
  const drawn = new Map<string, ReactElement>()
  for (const issue of loaded) {
    drawn.set(
      placeKey({ kind: 'status', name: issue.status }),
      <StatusMark category={issue.status_category} />,
    )
    for (const { words, mark } of rowMarksOf(issue, facts)) {
      drawn.set(placeKey({ kind: 'mark', name: words }), mark)
    }
  }

  return drawn
}

interface ListAndDetailProps {
  paging: IssuePaging
  shown: Issue[]
  facts: RowFacts
  unread: Problem | null
  outcome: Teller
  // more is the loading of more beneath the list, or nothing while the list
  // is not yet Jira's answer.
  more: ReactNode
}

// ListAndDetail is the view's list, with its loading of more, beside the
// selected issue's detail — or the view's empty state when it holds none, or
// why it could not be read, which is not an empty view. Below lg the list sits
// over the detail, in a pane of a few rows, so the detail it opens starts just
// beneath; from lg the two sit side by side and fill the window's height, each
// scrolling on its own. Each pane keeps a few pixels inside the edge it
// scrolls within, so a row's focus ring is not clipped, and the detail's pane
// pads its scroll as the content does, since WebKit scrolls a field that takes
// focus only until its caret shows. The detail's pane opens each issue at its
// top, as a section opens, and is positioned, as the content around it is, so
// the text it keeps for a screen reader scrolls and clips with it rather than
// growing the content.
function ListAndDetail({ paging, shown, facts, unread, outcome, more }: ListAndDetailProps) {
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
        {more}
      </div>
      <div
        key={selected ?? ''}
        className={cn(
          'relative min-w-0 flex-1 lg:overflow-y-auto lg:scroll-py-block lg:px-1',
          contentMeasure,
        )}
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
