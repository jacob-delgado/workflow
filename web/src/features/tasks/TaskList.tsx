import type { Task } from '@/api/generated/types.gen.ts'
import { cn } from '@/lib/utils.ts'
import { StateMark } from '@/shell/StateMark.tsx'
import { listsWaiting, matchesNarrowing, type TaskNarrowing } from './taskFacets.ts'
import { orderedTasks, type TaskOrder } from './taskOrder.ts'
import { dueWords, markOf, statusWords, taskNumber, waitsAt, waitsUntilWords } from './taskWords.ts'

// TaskGroups is the pending tasks as the section lists them: those for an
// issue the Issues list holds, then the others, each in the chosen order, and
// how many wait unlisted. order is that order, which each row's tail shows
// where it is by priority or by tag.
export interface TaskGroups {
  forIssues: Task[]
  others: Task[]
  waiting: number
  order: TaskOrder
}

// TaskListing is how the list is listed: its order and what narrows it.
export interface TaskListing extends TaskNarrowing {
  order: TaskOrder
}

// groupTasks sorts the pending tasks into the section's groups, as the
// terminal's Tasks pane does: a task the narrowing leaves out is not listed, a
// waiting task is counted rather than listed unless waiting is picked, and
// each group is in order.
export function groupTasks(
  tasks: Task[],
  issueKeys: Set<string>,
  now: number,
  listing: TaskListing,
): TaskGroups {
  const groups: TaskGroups = { forIssues: [], others: [], waiting: 0, order: listing.order }

  for (const task of tasks) {
    if (!matchesNarrowing(listing, task, now)) {
      continue
    }

    if (waitsAt(task, now) && !listsWaiting(listing.picked)) {
      groups.waiting++
    } else if (task.issue_key !== '' && issueKeys.has(task.issue_key)) {
      groups.forIssues.push(task)
    } else {
      groups.others.push(task)
    }
  }

  groups.forIssues = orderedTasks(groups.forIssues, listing.order, now)
  groups.others = orderedTasks(groups.others, listing.order, now)

  return groups
}

// listedOf is every task the section lists, in the order it draws them.
export function listedOf(groups: TaskGroups): Task[] {
  return [...groups.forIssues, ...groups.others]
}

interface TaskListProps {
  groups: TaskGroups
  // selected is the uuid of the task whose detail is shown.
  selected: string
  onSelect: (uuid: string) => void
  now: number
}

// TaskList is the listed tasks, one a row: those for your issues under a
// heading of their own, and the others under theirs, when there are both — one
// list, unheaded, when there are not — then how many wait unlisted.
export function TaskList({ groups, selected, onSelect, now }: TaskListProps) {
  const rows = { selected, onSelect, now, order: groups.order }
  const headed = groups.forIssues.length > 0 && groups.others.length > 0

  return (
    <>
      {headed ? (
        <>
          <Group title="For my issues" tasks={groups.forIssues} {...rows} />
          <Group title="Other tasks" tasks={groups.others} {...rows} />
        </>
      ) : (
        <Rows label="Tasks" tasks={listedOf(groups)} {...rows} />
      )}
      <WaitingCount waiting={groups.waiting} />
    </>
  )
}

// WaitingCount says how many pending tasks wait unlisted, or nothing when none
// does.
export function WaitingCount({ waiting }: { waiting: number }) {
  if (waiting === 0) {
    return null
  }

  return <p className="px-3 text-sm text-muted-foreground">{waiting} waiting</p>
}

interface RowsProps {
  label: string
  tasks: Task[]
  selected: string
  onSelect: (uuid: string) => void
  now: number
  order: TaskOrder
}

// Group is one group of rows under its heading.
function Group({ title, ...rows }: Omit<RowsProps, 'label'> & { title: string }) {
  return (
    <div className="flex flex-col gap-tight">
      <h2 className="px-3 text-sm font-semibold">{title}</h2>
      <Rows label={title} {...rows} />
    </div>
  )
}

// Rows is a list of tasks, each a row that shows its detail.
function Rows({ label, tasks, selected, onSelect, now, order }: RowsProps) {
  return (
    <ul aria-label={label} className="flex flex-col gap-tight">
      {tasks.map((task) => (
        <li key={task.uuid}>
          <TaskRow
            task={task}
            current={task.uuid === selected}
            now={now}
            order={order}
            onSelect={() => {
              onSelect(task.uuid)
            }}
          />
        </li>
      ))}
    </ul>
  )
}

interface TaskRowProps {
  task: Task
  current: boolean
  now: number
  order: TaskOrder
  onSelect: () => void
}

// TaskRow is one task: whether it is started, by its mark and in words, its id
// and what it is, then a quiet tail of the issue it is for, when it is due and
// how urgent Taskwarrior counts it. The mark and the smaller id each stand in a
// line as tall as the description's first, so however it wraps the mark sits
// centered on that line, and the id's words on its baseline.
function TaskRow({ task, current, now, order, onSelect }: TaskRowProps) {
  const number = taskNumber(task)

  return (
    <button
      type="button"
      aria-current={current ? true : undefined}
      onClick={onSelect}
      className={cn(
        'flex w-full flex-col gap-tight rounded-md border border-transparent px-3 py-2 text-left motion-safe:transition-colors',
        'hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none',
        current && 'border-border bg-accent',
      )}
    >
      <span className="flex items-start gap-item">
        <span className="flex h-5 shrink-0 items-center">
          <StateMark state={markOf(task)} className="text-taskwarrior" />
        </span>
        <span className="sr-only">{statusWords(task)}</span>
        {number === '' ? null : (
          <span className="shrink-0 text-xs leading-5 text-muted-foreground tabular-nums">
            #{number}
          </span>
        )}
        <span className="text-sm">{task.description}</span>
      </span>
      <span className="text-xs text-muted-foreground">{rowTail(task, now, order)}</span>
    </button>
  )
}

// rowTail is what a row says after the task: the issue it is for, when it is
// due, what it is sorted by where that is its priority or its tags, and its
// urgency, leaving out what it does not have.
function rowTail(task: Task, now: number, order: TaskOrder): string {
  const tail: string[] = []
  if (task.issue_key !== '') {
    tail.push(task.issue_key)
  }

  if (task.due !== undefined) {
    tail.push(dueWords(task.due, now))
  }

  if (waitsAt(task, now)) {
    tail.push(waitsUntilWords(task))
  }

  const sortedBy = sortKeyWords(task, order)
  if (sortedBy !== '') {
    tail.push(sortedBy)
  }

  tail.push(`urgency ${task.urgency.toFixed(1)}`)

  return tail.join(' · ')
}

// sortKeyWords is a task's priority or tags, as the terminal words them, when
// the list is sorted by that; nothing otherwise.
function sortKeyWords(task: Task, order: TaskOrder): string {
  switch (order) {
    case 'priority':
      return task.priority === '' ? 'no priority' : `priority ${task.priority}`
    case 'tag':
      return task.tags.length === 0 ? 'no tags' : task.tags.map((tag) => `+${tag}`).join(' ')
    default:
      return ''
  }
}
