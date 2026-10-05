import type { Task } from '@/api/generated/types.gen.ts'
import { FilterChips } from '@/lib/FilterChips.tsx'
import {
  isTaskFacetPicked,
  taskFacetChoices,
  taskFacetLabel,
  toggleTaskFacet,
  type TaskFacet,
} from './taskFacets.ts'
import { taskOrderWords, type TaskOrder } from './taskOrder.ts'

interface TaskListControlsProps {
  tasks: Task[]
  now: number
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
  now,
  order,
  onOrder,
  text,
  onText,
  picked,
  onPick,
}: TaskListControlsProps) {
  return (
    <div className="flex flex-col gap-item">
      <div className="flex flex-wrap items-center gap-group">
        <label className="flex items-center gap-item text-sm">
          <span className="text-muted-foreground">Sort</span>
          <select
            value={order}
            onChange={(event) => {
              onOrder(event.target.value as TaskOrder)
            }}
            className="rounded-md border border-input bg-background px-2 py-1 focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
          >
            {(Object.keys(taskOrderWords) as TaskOrder[]).map((choice) => (
              <option key={choice} value={choice}>
                {taskOrderWords[choice]}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-item text-sm">
          <span className="text-muted-foreground">Filter</span>
          <input
            type="search"
            value={text}
            placeholder="Text, +tag, issue or #id"
            onChange={(event) => {
              onText(event.target.value)
            }}
            className="w-48 rounded-md border border-input bg-background px-2 py-1 placeholder:text-muted-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
          />
        </label>
      </div>
      <FilterChips
        label="Narrow"
        choices={taskFacetChoices(tasks, picked, now)}
        isPicked={(facet) => isTaskFacetPicked(picked, facet)}
        nameOf={taskFacetLabel}
        keyOf={(facet) => `${facet.kind}:${facet.value}`}
        onToggle={(facet) => {
          onPick((now) => toggleTaskFacet(now, facet))
        }}
      />
    </div>
  )
}
