import { useState } from 'react'
import type { Issue } from '@/api/generated/types.gen.ts'
import { admits, shownKey, type Place } from '@/features/issues/issuePlaces.ts'

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
export function useNarrowing(view: string | null) {
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

// narrowed is the issues the search and the places picked leave: those whose
// key or summary holds the search, ignoring case — the interface's `/` filter,
// over the same text — and that are in the picked places.
export function narrowed(
  issues: Issue[],
  { filter, places }: Pick<Narrowing, 'filter' | 'places'>,
  marksFor: (issue: Issue) => string[],
): Issue[] {
  const typed = filter.toLowerCase()

  return issues.filter(
    (issue) =>
      `${shownKey(issue)} ${issue.summary}`.toLowerCase().includes(typed) &&
      admits(places, issue, marksFor(issue)),
  )
}

// filterOutcome says what the filter and the places left of the loaded issues:
// nothing while neither narrows them (or there is nothing to narrow), and
// otherwise how many match.
export function filterOutcome(narrowedBy: boolean, shown: number, loaded: number): string {
  if (!narrowedBy || loaded === 0) {
    return ''
  }

  if (shown === 0) {
    return 'No loaded issue matches the filter.'
  }

  return `${String(shown)} of ${String(loaded)} loaded issues ${shown === 1 ? 'matches' : 'match'}.`
}
