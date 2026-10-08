import type { Task, TaskRanks } from '@/api/generated/types.gen.ts'

// TaskOrder is the order the Tasks list is sorted in within its groups, as
// the terminal's O cycles it. The server ranks each task in every one of them,
// so the page sorts by the rank and works out no order of its own.
export type TaskOrder = keyof TaskRanks

// taskOrderWords names each order, in the order they are offered.
export const taskOrderWords: Record<TaskOrder, string> = {
  urgency: 'Most urgent first',
  state: 'By state',
  id: 'By ID',
  tag: 'By tag',
  issue: 'By issue',
  priority: 'By priority',
}

// orderedTasks is tasks in the order, by each task's rank in it.
export function orderedTasks(tasks: Task[], order: TaskOrder): Task[] {
  return tasks.toSorted((first, second) => first.ranks[order] - second.ranks[order])
}
