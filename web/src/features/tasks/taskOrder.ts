import type { Task } from '@/api/generated/types.gen.ts'
import { isActive, waitsAt } from './taskWords.ts'

// TaskOrder is the order the Tasks list is sorted in within its groups, as
// the terminal's O cycles it.
//
// Trade-off TRADE-29: these orders are written again in
// internal/taskwarrior/order.go, and twin-named tests pin the two.
export type TaskOrder = 'urgency' | 'state' | 'id' | 'tag' | 'issue' | 'priority'

// taskOrderWords names each order, in the order they are offered.
export const taskOrderWords: Record<TaskOrder, string> = {
  urgency: 'Most urgent first',
  state: 'By state',
  id: 'By ID',
  tag: 'By tag',
  issue: 'By issue',
  priority: 'By priority',
}

// TaskState is where a task stands, as the list words and orders it.
export type TaskState = 'started' | 'pending' | 'waiting' | 'recurring' | 'completed' | 'deleted'

const stateRank: Record<TaskState, number> = {
  started: 0,
  pending: 1,
  waiting: 2,
  recurring: 3,
  completed: 4,
  deleted: 5,
}

// stateOf is where a task stands at now: waiting until its wait passes,
// started once begun, and otherwise its status.
export function stateOf(task: Task, now: number): TaskState {
  if (waitsAt(task, now)) {
    return 'waiting'
  }

  return isActive(task) ? 'started' : task.status
}

type Compare = (first: Task, second: Task) => number

// orderedTasks is tasks in the order, every tie falling back to most urgent
// first, then id, then uuid, as Go's sortedBy has it.
export function orderedTasks(tasks: Task[], order: TaskOrder, now: number): Task[] {
  const first = primary(order, now)

  return [...tasks].sort((a, b) => first(a, b) || mostUrgentFirst(a, b))
}

function primary(order: TaskOrder, now: number): Compare {
  switch (order) {
    case 'urgency':
      return () => 0
    case 'state':
      return (a, b) => stateRank[stateOf(a, now)] - stateRank[stateOf(b, now)]
    case 'id':
      return (a, b) => lastWhen(a.id === 0, b.id === 0) || a.id - b.id
    case 'tag':
      return (a, b) =>
        lastWhen(a.tags.length === 0, b.tags.length === 0) ||
        compareLists(sortedTags(a), sortedTags(b))
    case 'issue':
      return (a, b) =>
        lastWhen(a.issue_key === '', b.issue_key === '') || naturalOrder(a.issue_key, b.issue_key)
    case 'priority':
      return (a, b) =>
        priorityRank(a.priority) - priorityRank(b.priority) ||
        codePointOrder(a.priority, b.priority)
  }
}

function mostUrgentFirst(a: Task, b: Task): number {
  return b.urgency - a.urgency || a.id - b.id || codePointOrder(a.uuid, b.uuid)
}

// lastWhen orders whatever a condition holds for after whatever it does not.
function lastWhen(firstLast: boolean, secondLast: boolean): number {
  return Number(firstLast) - Number(secondLast)
}

function sortedTags(task: Task): string[] {
  return task.tags.toSorted(codePointOrder)
}

// compareLists compares two lists element by element, a shorter one that is a
// prefix of the other first, as Go's slices.Compare does.
function compareLists(first: string[], second: string[]): number {
  return firstDifference(first, second, codePointOrder) || first.length - second.length
}

// firstDifference is the first nonzero comparison of the two lists' items at
// the same place, over the length of the shorter, or 0 when there is none.
function firstDifference<T>(first: T[], second: T[], compare: (a: T, b: T) => number): number {
  for (const [index, item] of first.slice(0, second.length).entries()) {
    const other = second[index]
    const order = other === undefined ? 0 : compare(item, other)
    if (order !== 0) {
      return order
    }
  }

  return 0
}

function priorityRank(priority: string): number {
  switch (priority) {
    case 'H':
      return 0
    case 'M':
      return 1
    case 'L':
      return 2
    case '':
      return 4
    default:
      return 3
  }
}

// naturalOrder compares two keys reading each run of ASCII digits as a number,
// so PROJ-2 comes before PROJ-10, and anything else by code point.
function naturalOrder(first: string, second: string): number {
  const runs = /\d+|\D+/g
  const firstRuns = first.match(runs) ?? []
  const secondRuns = second.match(runs) ?? []

  return firstDifference(firstRuns, secondRuns, compareRuns) || firstRuns.length - secondRuns.length
}

function compareRuns(first: string, second: string): number {
  const digits = /^\d/
  if (!digits.test(first) || !digits.test(second)) {
    return codePointOrder(first, second)
  }

  const a = first.replace(/^0+/, '')
  const b = second.replace(/^0+/, '')

  return a.length - b.length || codePointOrder(a, b)
}

// codePointOrder compares by code point, which is the order Go's byte-wise
// compare of UTF-8 gives; JavaScript's own < compares UTF-16 code units, which
// differ for a character outside the Basic Multilingual Plane.
export function codePointOrder(first: string, second: string): number {
  const a = codePoints(first)
  const b = codePoints(second)

  return firstDifference(a, b, (x, y) => x - y) || a.length - b.length
}

// codePoints is text as its code points, a character outside the Basic
// Multilingual Plane being one point rather than two halves.
function codePoints(text: string): number[] {
  const points: number[] = []
  for (const character of text) {
    points.push(character.codePointAt(0) ?? 0)
  }

  return points
}
