import {
  cacheTuning,
  certificate,
  firstInEveryOrder,
  ranked,
  secondInEveryOrder,
  tokenLeak,
} from '@/test/tasks.ts'
import { orderedTasks } from './taskOrder.ts'

// The page sorts by the rank the server gives each task in each order; how
// each order ranks a task is the server's, pinned by
// internal/taskwarrior/order_test.go.

test('each order lists the tasks by their rank in it', () => {
  // Arrange
  // As the server ranks the three, by issue the two that track one come
  // first, by key, then the certificate.
  const tasks = [
    ranked(cacheTuning, { urgency: 2, state: 2, id: 2, tag: 2, issue: 1, priority: 2 }),
    ranked(certificate, { urgency: 1, state: 1, id: 1, tag: 1, issue: 2, priority: 1 }),
    ranked(tokenLeak, firstInEveryOrder),
  ]

  // Act
  const got = orderedTasks(tasks, 'issue')

  // Assert
  expect(got.map((task) => task.description)).toEqual([
    tokenLeak.description,
    cacheTuning.description,
    certificate.description,
  ])
})

test('ordering leaves the tasks it was given alone', () => {
  // Arrange
  const tasks = [ranked(certificate, secondInEveryOrder), ranked(tokenLeak, firstInEveryOrder)]

  // Act
  orderedTasks(tasks, 'tag')

  // Assert
  expect(tasks.map((task) => task.description)).toEqual([
    certificate.description,
    tokenLeak.description,
  ])
})
