import type { Task } from '@/api/generated/types.gen.ts'
import { cn } from '@/lib/utils.ts'
import { StateMark } from '@/shell/StateMark.tsx'
import { dueWords, markOf, statusWords, taskNumber, waitsAt } from './taskWords.ts'

// TaskGroups is the pending tasks as the section lists them: those for an
// issue the Issues list holds, then the others, each most urgent first, and
// how many wait unlisted.
export interface TaskGroups {
  forIssues: Task[]
  others: Task[]
  waiting: number
}

// groupTasks sorts the pending tasks into the section's groups, as the
// terminal's Tasks pane does: a waiting task is counted, not listed.
export function groupTasks(tasks: Task[], issueKeys: Set<string>, now: number): TaskGroups {
  const groups: TaskGroups = { forIssues: [], others: [], waiting: 0 }

  for (const task of tasks) {
    if (waitsAt(task, now)) {
      groups.waiting++
    } else if (task.issue_key !== '' && issueKeys.has(task.issue_key)) {
      groups.forIssues.push(task)
    } else {
      groups.others.push(task)
    }
  }

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
  const rows = { selected, onSelect, now }
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
function Rows({ label, tasks, selected, onSelect, now }: RowsProps) {
  return (
    <ul aria-label={label} className="flex flex-col gap-tight">
      {tasks.map((task) => (
        <li key={task.uuid}>
          <TaskRow
            task={task}
            current={task.uuid === selected}
            now={now}
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
  onSelect: () => void
}

// TaskRow is one task: whether it is started, by its mark and in words, its id
// and what it is, then a quiet tail of the issue it is for, when it is due and
// how urgent Taskwarrior counts it. The mark and the smaller id each stand in a
// line as tall as the description's first, so however it wraps the mark sits
// centered on that line, and the id's words on its baseline.
function TaskRow({ task, current, now, onSelect }: TaskRowProps) {
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
      <span className="text-xs text-muted-foreground">{rowTail(task, now)}</span>
    </button>
  )
}

// rowTail is what a row says after the task: the issue it is for, when it is
// due, and its urgency, leaving out what it does not have.
function rowTail(task: Task, now: number): string {
  const tail: string[] = []
  if (task.issue_key !== '') {
    tail.push(task.issue_key)
  }

  if (task.due !== undefined) {
    tail.push(dueWords(task.due, now))
  }

  tail.push(`urgency ${task.urgency.toFixed(1)}`)

  return tail.join(' · ')
}
