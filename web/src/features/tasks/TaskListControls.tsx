import { useRef, type ReactElement } from 'react'
import type { Task } from '@/api/generated/types.gen.ts'
import { useShortcut, useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { FilterChips } from '@/lib/FilterChips.tsx'
import { Input, Select } from '@/lib/Field.tsx'
import {
  isTaskFacetPicked,
  taskFacetChoices,
  toggleTaskFacet,
  type TaskFacet,
} from './taskFacets.ts'
import { TaskMark } from './TaskList.tsx'
import { markOf } from './taskWords.ts'
import { taskOrderWords, type TaskOrder } from './taskOrder.ts'

interface TaskListControlsProps {
  tasks: Task[]
  // facetOrder is every value the filter offers, in the server's order.
  facetOrder: TaskFacet[]
  order: TaskOrder
  onOrder: (order: TaskOrder) => void
  text: string
  onText: (text: string) => void
  picked: TaskFacet[]
  onPick: (pick: (picked: TaskFacet[]) => TaskFacet[]) => void
}

// TaskListControls choose how the list is listed, as the terminal's keys do:
// the order within its groups (O), a filter over what the tasks say (/), and
// the values to narrow it to (f). The first row wraps rather than scroll
// sideways in a narrow window.
export function TaskListControls({
  tasks,
  facetOrder,
  order,
  onOrder,
  text,
  onText,
  picked,
  onPick,
}: TaskListControlsProps) {
  const filter = useRef<HTMLInputElement>(null)
  const searchKeys = useShortcut('search-tasks', filter, 'focus')
  const sort = useShortcutProps<HTMLSelectElement>('sort-tasks', 'focus')

  return (
    <div className="flex flex-col gap-item">
      <div className="flex flex-wrap items-center gap-group">
        <label className="flex items-center gap-item text-sm">
          <span className="text-muted-foreground">Sort</span>
          <Select
            {...sort}
            size="sm"
            value={order}
            onChange={(event) => {
              onOrder(event.target.value as TaskOrder)
            }}
          >
            {(Object.keys(taskOrderWords) as TaskOrder[]).map((choice) => (
              <option key={choice} value={choice}>
                {taskOrderWords[choice]}
              </option>
            ))}
          </Select>
        </label>
        <label className="flex items-center gap-item text-sm">
          <span className="text-muted-foreground">Search</span>
          <Input
            size="sm"
            ref={filter}
            aria-keyshortcuts={searchKeys}
            type="search"
            value={text}
            placeholder="Text, +tag, issue or #id"
            onChange={(event) => {
              onText(event.target.value)
            }}
            className="w-64"
          />
        </label>
      </div>
      <FilterChips
        label="Filter"
        choices={taskFacetChoices(tasks, facetOrder, picked)}
        isPicked={(facet) => isTaskFacetPicked(picked, facet)}
        nameOf={(facet) => facet.label}
        keyOf={(facet) => `${facet.kind}:${facet.value}`}
        markOf={(facet) => stateMark(facet, tasks)}
        onToggle={(facet) => {
          onPick((current) => toggleTaskFacet(current, facet))
        }}
        afterLast={filter}
      />
    </div>
  )
}

// stateMark is the mark the rows draw for a state, when every task in it is
// drawn alike. A started task whose wait is still ahead is waiting to the
// server but in flight to its row, beside the waiting ones not started, so a
// state can hold tasks marked differently, and then its chip draws none. The
// rows mark nothing else a task holds, so no other value has one, nor a state
// no task is in.
function stateMark(facet: TaskFacet, tasks: Task[]): ReactElement | undefined {
  const inState = facet.kind === 'state' ? tasks.filter((task) => task.state === facet.value) : []
  const [first] = inState
  if (first === undefined || inState.some((task) => markOf(task) !== markOf(first))) {
    return undefined
  }

  return <TaskMark task={first} />
}
