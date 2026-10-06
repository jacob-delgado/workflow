import { useRef } from 'react'
import type { Task } from '@/api/generated/types.gen.ts'
import { useShortcut, useShortcutProps } from '@/features/keyboard/useShortcut.ts'
import { FilterChips } from '@/lib/FilterChips.tsx'
import { Input, Select } from '@/lib/Field.tsx'
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
        choices={taskFacetChoices(tasks, picked, now)}
        isPicked={(facet) => isTaskFacetPicked(picked, facet)}
        nameOf={taskFacetLabel}
        keyOf={(facet) => `${facet.kind}:${facet.value}`}
        onToggle={(facet) => {
          onPick((current) => toggleTaskFacet(current, facet))
        }}
        afterLast={filter}
      />
    </div>
  )
}
