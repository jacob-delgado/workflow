import { taskOrderWords, type TaskOrder } from './taskOrder.ts'

interface TaskListControlsProps {
  order: TaskOrder
  onOrder: (order: TaskOrder) => void
}

// TaskListControls choose how the list is listed: the order it is sorted in
// within its groups, as the terminal's O cycles it. The row wraps rather than
// scroll sideways in a narrow window.
export function TaskListControls({ order, onOrder }: TaskListControlsProps) {
  return (
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
    </div>
  )
}
