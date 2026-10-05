import type { FilterChoice } from '@/lib/FilterChips.tsx'
import type { Issue, StatusCategory, TasksSummary } from '@/api/generated/types.gen.ts'
import { issueTaskMark, linkedTo } from '@/features/tasks/taskWords.ts'

// A place is where an issue can be: one of its tracker's status names, or one
// of workflow's own marks. Places in one group widen the list, and the two
// groups narrow it together.
//
// Trade-off TRADE-21: these rules are written again in
// internal/tui/issueplaces.go, and twin-named tests pin the two.
type PlaceKind = 'status' | 'mark'

export interface Place {
  kind: PlaceKind
  name: string
}

// PlaceChoice is a place on offer, with how many loaded issues are in it.
export type PlaceChoice = FilterChoice<Place>

const inFlight = 'in flight'

const forgeIssue = 'forge issue'

// The marks in the order they are offered, as the rows word them.
const markOrder = [inFlight, 'task active', 'tracked', 'task done', forgeIssue]

const categoryOrder: StatusCategory[] = ['new', 'indeterminate', 'done']

export function samePlace(a: Place, b: Place): boolean {
  return a.kind === b.kind && a.name === b.name
}

// marksOf is the marks an issue is in: in flight when a branch names it, how
// its tasks stand when Taskwarrior can be asked and one is linked, and on the
// forge when it is the repository's own forge issue rather than Jira's.
export function marksOf(issue: Issue, branchKeys: Set<string>, tasks: TasksSummary): string[] {
  const marks: string[] = []
  if (branchKeys.has(issue.key)) {
    marks.push(inFlight)
  }

  const taskMark = tasks.available ? issueTaskMark(linkedTo(tasks.linked, issue.key)) : undefined
  if (taskMark !== undefined) {
    marks.push(taskMark.words)
  }

  if (issue.tracker === 'forge') {
    marks.push(forgeIssue)
  }

  return marks
}

// admits reports whether an issue is in the picked places: in any picked
// status, when a status is picked, and holding any picked mark, when a mark is.
export function admits(picked: Place[], issue: Issue, marks: string[]): boolean {
  return admitsIn(picked, 'status', [issue.status]) && admitsIn(picked, 'mark', marks)
}

function admitsIn(picked: Place[], kind: PlaceKind, names: string[]): boolean {
  const inGroup = picked.filter((place) => place.kind === kind)

  return inGroup.length === 0 || inGroup.some((place) => names.includes(place.name))
}

// placeChoices is every place the issues are in, with how many are in each —
// statuses by category, not started first, then as they first appear, then
// the marks in their fixed order — and every picked place no issue is in, at
// zero, so it can still be unpicked.
export function placeChoices(
  issues: Issue[],
  marksFor: (issue: Issue) => string[],
  picked: Place[],
): PlaceChoice[] {
  const counts = new Map<string, number>()
  const bump = (place: Place) => {
    counts.set(placeId(place), (counts.get(placeId(place)) ?? 0) + 1)
  }
  for (const issue of issues) {
    bump({ kind: 'status', name: issue.status })
    marksFor(issue).forEach((name) => {
      bump({ kind: 'mark', name })
    })
  }

  // A status in a category Jira does not name (its "No Category") comes after
  // the three it does, so it can still be narrowed to.
  const statuses = unique([
    ...categoryOrder.flatMap((category) =>
      issues.filter((issue) => issue.status_category === category).map((issue) => issue.status),
    ),
    ...issues
      .filter((issue) => !categoryOrder.includes(issue.status_category))
      .map((issue) => issue.status),
  ])
  const gone = picked.filter((place) => place.kind === 'status' && !statuses.includes(place.name))
  const offered: Place[] = [
    ...statuses.map((name): Place => ({ kind: 'status', name })),
    ...gone,
    ...markOrder.map((name): Place => ({ kind: 'mark', name })),
  ]

  return offered
    .map((place) => ({ value: place, count: counts.get(placeId(place)) ?? 0 }))
    .filter((choice) => choice.count > 0 || picked.some((place) => samePlace(place, choice.value)))
}

// togglePlace picks a place, or unpicks it when it is already picked.
export function togglePlace(picked: Place[], place: Place): Place[] {
  return picked.some((chosen) => samePlace(chosen, place))
    ? picked.filter((chosen) => !samePlace(chosen, place))
    : [...picked, place]
}

function placeId(place: Place): string {
  return `${place.kind}:${place.name}`
}

function unique(names: string[]): string[] {
  return [...new Set(names)]
}

// shownKey is an issue's key as the list shows it: a forge issue's number
// after a #, as the forge writes it, so it reads apart from a Jira key.
export function shownKey(issue: Pick<Issue, 'key' | 'tracker'>): string {
  return issue.tracker === 'forge' ? `#${issue.key}` : issue.key
}

// shownLinkKey is the key a branch was linked to by hand, shown as shownKey
// shows its issue: the link keeps no tracker, but only a forge issue's key is
// a bare number, as convention.RefOf has it.
export function shownLinkKey(key: string): string {
  return shownKey({ key, tracker: /^[1-9][0-9]*$/.test(key) ? 'forge' : 'jira' })
}
