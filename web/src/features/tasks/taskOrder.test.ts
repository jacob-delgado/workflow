import type { Task, TaskRanks } from '@/api/generated/types.gen.ts'
import { makeTask } from '@/test/fixtures.ts'
import { orderedTasks } from './taskOrder.ts'

// The page sorts by the rank the server gives each task in each order; how
// each order ranks a task is the server's, pinned by
// internal/taskwarrior/order_test.go.

function ranked(uuid: string, ranks: Partial<TaskRanks>): Task {
  return makeTask({
    uuid,
    ranks: { urgency: 0, state: 0, id: 0, tag: 0, issue: 0, priority: 0, ...ranks },
  })
}

test('each order lists the tasks by their rank in it', () => {
  // Arrange
  const tasks = [
    ranked('c', { urgency: 0, priority: 2 }),
    ranked('a', { urgency: 1, priority: 0 }),
    ranked('b', { urgency: 2, priority: 1 }),
  ]

  // Act
  const got = orderedTasks(tasks, 'priority')

  // Assert
  expect(got.map((task) => task.uuid)).toEqual(['a', 'b', 'c'])
})

test('ordering leaves the tasks it was given alone', () => {
  // Arrange
  const tasks = [ranked('b', { tag: 1 }), ranked('a', { tag: 0 })]

  // Act
  orderedTasks(tasks, 'tag')

  // Assert
  expect(tasks.map((task) => task.uuid)).toEqual(['b', 'a'])
})
