import type { Task, TaskFacet as WireTaskFacet } from '@/api/generated/types.gen.ts'
import type { FilterChoice } from '@/lib/FilterChips.tsx'

// A task facet is one value a task holds in one of the five kinds the list is
// narrowed by, as the server describes it: each task carries its own, each
// labeled, and the list the order the filter offers them in. Values picked in
// one kind widen the list, and the kinds narrow it together; the page only
// counts, picks and admits.
export type TaskFacet = WireTaskFacet

export type TaskFacetChoice = FilterChoice<TaskFacet>

// TaskNarrowing is what the list is narrowed to: the facets picked, and text
// typed.
export interface TaskNarrowing {
  picked: TaskFacet[]
  text: string
}

function sameFacet(a: TaskFacet, b: TaskFacet): boolean {
  return a.kind === b.kind && a.value === b.value
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
  return picked.some((chosen) => chosen.kind === 'state' && chosen.value === 'waiting')
}

// narrows reports a narrowing that leaves any task out.
export function narrows(narrowing: TaskNarrowing): boolean {
  return narrowing.picked.length > 0 || narrowing.text !== ''
}

// matchesNarrowing reports a task the narrowing lets through: the text in one
// of the fields the server says it matches, and, in every kind with a value
// picked, one held.
export function matchesNarrowing(narrowing: TaskNarrowing, task: Task): boolean {
  const needle = lowerCased(narrowing.text)
  if (!task.searchable.some((field) => field.includes(needle))) {
    return false
  }

  return narrowing.picked.every((chosen) =>
    task.facets.some(
      (held) => isTaskFacetPicked(narrowing.picked, held) && held.kind === chosen.kind,
    ),
  )
}

// lowerCased is text lower-cased letter by letter, each to its own lower case
// alone, as the server lowers the fields it matches: never to two letters, as
// JavaScript lowers İ, nor by the letters around it, as it lowers a final Σ.
function lowerCased(text: string): string {
  return Array.from(text, (letter) =>
    String.fromCodePoint(letter.toLowerCase().codePointAt(0) ?? 0),
  ).join('')
}

// taskFacetChoices is each value the server offers, in its order, that a task
// holds or that is picked, with how many tasks hold it, then each picked value
// it no longer offers, at zero, in the order it was picked. A count is over the
// tasks the list shows with nothing picked — those waiting left out — but for
// their state, which counts them.
export function taskFacetChoices(
  tasks: Task[],
  order: TaskFacet[],
  picked: TaskFacet[],
): TaskFacetChoice[] {
  const held = tasks.flatMap((task) =>
    task.state === 'waiting' ? task.facets.filter((facet) => facet.kind === 'state') : task.facets,
  )
  const countOf = (facet: TaskFacet) => held.filter((one) => sameFacet(one, facet)).length
  const offered = order
    .map((facet) => ({ value: facet, count: countOf(facet) }))
    .filter((choice) => choice.count > 0 || isTaskFacetPicked(picked, choice.value))
  const unoffered = picked
    .filter((facet) => !order.some((one) => sameFacet(one, facet)))
    .map((facet) => ({ value: facet, count: 0 }))

  return [...offered, ...unoffered]
}
