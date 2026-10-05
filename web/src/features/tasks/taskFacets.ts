import type { Task } from '@/api/generated/types.gen.ts'
import type { FilterChoice } from '@/lib/FilterChips.tsx'
import { codePointOrder, stateOf, type TaskState } from './taskOrder.ts'

// A task facet is one value a task holds in one of five kinds: its state as
// the list words it, priority, project, tag ('' for none of each), or whether
// it is linked to an issue. Values picked in one kind widen the list, and the
// kinds narrow it together.
//
// Trade-off TRADE-29: these rules are written again in
// internal/taskwarrior/narrow.go, and twin-named tests pin the two.
type TaskFacetKind = 'state' | 'priority' | 'project' | 'tag' | 'issue'

export interface TaskFacet {
  kind: TaskFacetKind
  value: string
}

export type TaskFacetChoice = FilterChoice<TaskFacet>

// TaskNarrowing is what the list is narrowed to: the facets picked, and text
// typed.
export interface TaskNarrowing {
  picked: TaskFacet[]
  text: string
}

const withIssue = 'linked'
const noIssue = 'unlinked'

const stateOrder: TaskState[] = [
  'started',
  'pending',
  'waiting',
  'recurring',
  'completed',
  'deleted',
]
const kindOrder: TaskFacetKind[] = ['state', 'priority', 'project', 'tag', 'issue']
const noneWords: Record<TaskFacetKind, string> = {
  state: 'no state',
  priority: 'no priority',
  project: 'no project',
  tag: 'no tag',
  issue: 'no issue',
}

function sameFacet(a: TaskFacet, b: TaskFacet): boolean {
  return a.kind === b.kind && a.value === b.value
}

// taskFacetLabel is a facet as the list words it.
export function taskFacetLabel(facet: TaskFacet): string {
  if (facet.value === '') {
    return noneWords[facet.kind]
  }

  switch (facet.kind) {
    case 'priority':
      return `priority ${facet.value}`
    case 'project':
      return `project ${facet.value}`
    case 'tag':
      return `+${facet.value}`
    case 'issue':
      return facet.value === withIssue ? 'with issue' : 'no issue'
    case 'state':
      return facet.value
  }
}

// taskFacetsOf is every value a task holds at now.
function taskFacetsOf(task: Task, now: number): TaskFacet[] {
  const facets: TaskFacet[] = [
    { kind: 'state', value: stateOf(task, now) },
    { kind: 'priority', value: task.priority },
    { kind: 'project', value: task.project },
    { kind: 'issue', value: task.issue_key === '' ? noIssue : withIssue },
  ]
  const tags = task.tags.length === 0 ? [''] : task.tags

  return [...facets, ...tags.map((value): TaskFacet => ({ kind: 'tag', value }))]
}

// isTaskFacetPicked reports whether facet is among the picked.
export function isTaskFacetPicked(picked: TaskFacet[], facet: TaskFacet): boolean {
  return picked.some((chosen) => sameFacet(chosen, facet))
}

// toggleTaskFacet picks a facet, or unpicks it when it is already picked.
export function toggleTaskFacet(picked: TaskFacet[], facet: TaskFacet): TaskFacet[] {
  return isTaskFacetPicked(picked, facet)
    ? picked.filter((chosen) => !sameFacet(chosen, facet))
    : [...picked, facet]
}

// listsWaiting reports a narrowing that asks for the waiting tasks, which the
// list otherwise only counts.
export function listsWaiting(picked: TaskFacet[]): boolean {
  return isTaskFacetPicked(picked, { kind: 'state', value: 'waiting' })
}

// narrows reports a narrowing that leaves any task out.
export function narrows(narrowing: TaskNarrowing): boolean {
  return narrowing.picked.length > 0 || narrowing.text !== ''
}

// matchesNarrowing reports a task the narrowing lets through at now: the text
// in one of its fields, and, in every kind with a value picked, one held.
export function matchesNarrowing(narrowing: TaskNarrowing, task: Task, now: number): boolean {
  if (!mentions(task, narrowing.text)) {
    return false
  }

  const held = taskFacetsOf(task, now)
  const kinds = kindOrder.filter((kind) => narrowing.picked.some((facet) => facet.kind === kind))

  return kinds.every((kind) =>
    held.some((facet) => facet.kind === kind && isTaskFacetPicked(narrowing.picked, facet)),
  )
}

// mentions reports text, ignoring case, within one of a task's fields: its
// description, project, a tag written +tag, its issue key, or #id. A match
// never spans two fields.
function mentions(task: Task, text: string): boolean {
  if (text === '') {
    return true
  }

  const needle = text.toLowerCase()
  const fields = [
    task.description,
    task.project,
    task.issue_key,
    ...task.tags.map((tag) => `+${tag}`),
  ]
  if (task.id > 0) {
    fields.push(`#${String(task.id)}`)
  }

  return fields.some((field) => field.toLowerCase().includes(needle))
}

// taskFacetChoices is every value the tasks hold, with how many hold each, in
// the order the list offers them, and every picked value none holds, at zero.
// A count is over the tasks the list shows with nothing picked — the waiting
// ones left out — but for the waiting state, which counts them.
export function taskFacetChoices(
  tasks: Task[],
  picked: TaskFacet[],
  now: number,
): TaskFacetChoice[] {
  const counts = new Map<string, number>()
  const id = (facet: TaskFacet) => `${facet.kind}\u0000${facet.value}`

  for (const task of tasks) {
    const waits = stateOf(task, now) === 'waiting'
    for (const facet of taskFacetsOf(task, now)) {
      if (!waits || facet.kind === 'state') {
        counts.set(id(facet), (counts.get(id(facet)) ?? 0) + 1)
      }
    }
  }

  const held = [...counts.keys()].map((key): TaskFacet => {
    const [kind, value] = key.split('\u0000') as [TaskFacetKind, string]

    return { kind, value }
  })
  const offered: TaskFacet[] = [
    ...stateOrder.map((value): TaskFacet => ({ kind: 'state', value })),
    ...ranked('priority', ['H', 'M', 'L'], [...held, ...picked]),
    ...ranked('project', [], [...held, ...picked]),
    ...ranked('tag', [], [...held, ...picked]),
    { kind: 'issue', value: withIssue },
    { kind: 'issue', value: noIssue },
  ]

  return offered
    .map((facet) => ({ value: facet, count: counts.get(id(facet)) ?? 0 }))
    .filter((choice) => choice.count > 0 || isTaskFacetPicked(picked, choice.value))
}

// ranked is a kind's values in order: those named first, then the rest by
// code point, as Go sorts strings, then none.
function ranked(kind: TaskFacetKind, first: string[], facets: TaskFacet[]): TaskFacet[] {
  const rest = [
    ...new Set(
      facets
        .filter(
          (facet) => facet.kind === kind && facet.value !== '' && !first.includes(facet.value),
        )
        .map((facet) => facet.value),
    ),
  ].sort(codePointOrder)

  return [...first, ...rest, ''].map((value) => ({ kind, value }))
}
