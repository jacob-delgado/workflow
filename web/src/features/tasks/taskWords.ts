import type { Task, TaskList } from '@/api/generated/types.gen.ts'
import type { MarkState } from '@/lib/StateMark.tsx'

const minute = 60_000
const hour = 60 * minute
const day = 24 * hour

// How much of a uuid names a task that has no id, as Taskwarrior's own short
// uuids do.
const shortUUID = 8

// Started is a task that is started: its start is always there.
type Started = Task & { start: string }

// isActive reports a task that is started: pending, with a start.
export function isActive(task: Task): task is Started {
  return task.status === 'pending' && task.start !== undefined
}

// stillToDo reports a task not completed — pending, waiting or recurring — which
// is what tracks the issue it is linked to, for the issue's mark and its Track
// alike.
export function stillToDo(task: Task): boolean {
  return task.status !== 'completed'
}

// waitsAt reports a task hidden until a date still after now.
export function waitsAt(task: Task, now: number): boolean {
  return (
    task.status === 'waiting' ||
    (task.status === 'pending' && task.wait !== undefined && Date.parse(task.wait) > now)
  )
}

// linkedTo is every task linked to an issue.
export function linkedTo(tasks: Task[], issueKey: string): Task[] {
  return tasks.filter((task) => task.issue_key === issueKey)
}

// statusWords names a task's state in Taskwarrior's own words, a started task
// as started.
export function statusWords(task: Task): string {
  return isActive(task) ? 'started' : task.status
}

// markOf is how far a task has got, by shape — started, completed, or not
// started yet — as the terminal's glyph for it is.
export function markOf(task: Task): MarkState {
  if (isActive(task)) {
    return 'in-flight'
  }

  return task.status === 'completed' ? 'done' : 'not-started'
}

// IssueTaskMark is how an issue's tasks stand, as a mark and in the words beside
// it.
interface IssueTaskMark {
  state: MarkState
  words: string
}

// issueTaskMark is an issue's task state by shape, as the terminal's Issues
// rows mark it: active while a linked task is started, tracked while one is
// still to do, done once every one is completed — or none, with no task linked.
export function issueTaskMark(linked: Task[]): IssueTaskMark | undefined {
  if (linked.some(isActive)) {
    return { state: 'in-flight', words: 'task active' }
  }

  if (linked.some(stillToDo)) {
    return { state: 'not-started', words: 'tracked' }
  }

  return linked.length > 0 ? { state: 'done', words: 'task done' } : undefined
}

// spanWords says a length of time as the terminal's spine does: minutes under
// an hour, hours and minutes under a day, and days and hours past one.
function spanWords(length: number): string {
  if (length < hour) {
    return `${String(Math.floor(length / minute))}m`
  }

  if (length < day) {
    return `${String(Math.floor(length / hour))}h${String(Math.floor((length % hour) / minute))}m`
  }

  return `${String(Math.floor(length / day))}d ${String(Math.floor((length % day) / hour))}h`
}

// elapsedWords says how long ago from was, as of now; a start the clock has
// not reached yet is no time at all.
export function elapsedWords(from: string, now: number): string {
  return spanWords(Math.max(0, now - Date.parse(from)))
}

// dueWords says how soon a task is due, as of now, or that it is overdue.
export function dueWords(due: string, now: number): string {
  const left = Date.parse(due) - now

  return left <= 0 ? 'overdue' : `due in ${spanWords(left)}`
}

// waitsUntilWords says until which day a waiting task waits, in the browser's
// zone, as the terminal says it in the clock's.
export function waitsUntilWords(task: Task): string {
  if (task.wait === undefined) {
    return 'waiting'
  }

  const until = new Date(task.wait)
  const day = (part: number) => String(part).padStart(2, '0')

  return `waits until ${String(until.getFullYear())}-${day(until.getMonth() + 1)}-${day(until.getDate())}`
}

// taskNumber is a task's working-set id, or '' where the id means nothing: only
// a pending or waiting task has one, and reads skip the garbage collection that
// would clear a finished task's stale id.
export function taskNumber(task: Task): string {
  if (task.id === 0 || (task.status !== 'pending' && task.status !== 'waiting')) {
    return ''
  }

  return String(task.id)
}

// shortened is the start of a uuid, as Taskwarrior's own short uuids are.
function shortened(uuid: string): string {
  return uuid.slice(0, shortUUID)
}

// taskName names a task as a sentence does: by its id, or by the start of its
// uuid where it has none.
export function taskName(task: Task): string {
  const number = taskNumber(task)

  return `task ${number === '' ? shortened(task.uuid) : number}`
}

// addedTask is the task an add or a track made, as the list after it holds it,
// or undefined where the list — narrowed by a context — leaves it out.
export function addedTask(list: TaskList): Task | undefined {
  return list.tasks.find((task) => task.uuid === list.added)
}

// addedName names the task an add or a track made: as the list after it holds
// it, by the start of its uuid where the list leaves it out, or as "the task"
// where the answer names none.
export function addedName(list: TaskList): string {
  const task = addedTask(list)
  if (task !== undefined) {
    return taskName(task)
  }

  return list.added === undefined ? 'the task' : `task ${shortened(list.added)}`
}

// saidWords is what an undo or a sync says it did, and what Taskwarrior said of
// it, when it said anything.
export function saidWords(did: string, said: string): string {
  return said === '' ? `${did}.` : `${did}: ${said}`
}
