import type { Task } from '@/api/generated/types.gen.ts'
import { makeTask } from '@/test/fixtures.ts'
import { orderedTasks, stateOf, type TaskOrder } from './taskOrder.ts'

// The twin of internal/taskwarrior/order_test.go: the cases carry the same
// names, so a change to one order's rule is made to both.

const now = Date.parse('2026-10-05T12:00:00Z')
const hour = 3_600_000

function task(uuid: string, overrides: Partial<Task> = {}): Task {
  return makeTask({ uuid, id: 1, urgency: 0, issue_key: '', ...overrides })
}

function uuids(tasks: Task[]): string[] {
  return tasks.map((each) => each.uuid)
}

const cases: [string, TaskOrder, Task[], string[]][] = [
  [
    'state: started, then pending, then waiting',
    'state',
    [
      task('s3', { wait: new Date(now + hour).toISOString(), urgency: 9 }),
      task('s2', { urgency: 1 }),
      task('s1', { start: new Date(now - hour).toISOString() }),
    ],
    ['s1', 's2', 's3'],
  ],
  [
    'id: lowest first, and a task outside the working set last',
    'id',
    [task('i3', { id: 0 }), task('i2', { id: 12 }), task('i1', { id: 3 })],
    ['i1', 'i2', 'i3'],
  ],
  [
    'tag: by the tags in order, whatever order they were added in, untagged last',
    'tag',
    [
      task('t4'),
      task('t3', { tags: ['web', 'ci'] }),
      task('t2', { tags: ['ci'] }),
      task('t1', { tags: ['api'] }),
    ],
    ['t1', 't2', 't3', 't4'],
  ],
  [
    'issue: keys in natural order, so PROJ-2 before PROJ-10, and unlinked last',
    'issue',
    [
      task('k4'),
      task('k3', { issue_key: 'PROJ-10' }),
      task('k2', { issue_key: 'PROJ-2' }),
      task('k1', { issue_key: 'ABC-7' }),
    ],
    ['k1', 'k2', 'k3', 'k4'],
  ],
  [
    'issue: keys equal but for leading zeros fall back to most urgent first',
    'issue',
    [
      task('z2', { issue_key: 'PROJ-1', urgency: 1 }),
      task('z1', { issue_key: 'PROJ-01', urgency: 5 }),
    ],
    ['z1', 'z2'],
  ],
  [
    'priority: high, medium, low, anything else, then none',
    'priority',
    [
      task('p5'),
      task('p3', { priority: 'L' }),
      task('p4', { priority: 'X' }),
      task('p1', { priority: 'H' }),
      task('p2', { priority: 'M' }),
    ],
    ['p1', 'p2', 'p3', 'p4', 'p5'],
  ],
  [
    'a tie falls back to most urgent first',
    'priority',
    [task('u2', { priority: 'H', urgency: 2 }), task('u1', { priority: 'H', urgency: 8 })],
    ['u1', 'u2'],
  ],
]

test.each(cases)('each order puts tasks where its key says: %s', (_name, order, tasks, want) => {
  // Act
  const got = uuids(orderedTasks(tasks, order, now))

  // Assert
  expect(got).toEqual(want)
})

test('ordering leaves the tasks it was given alone', () => {
  // Arrange
  const tags = ['web', 'ci']
  const tasks = [task('b', { tags }), task('a', { tags: ['docs'] })]

  // Act
  orderedTasks(tasks, 'tag', now)

  // Assert
  expect(uuids(tasks)).toEqual(['b', 'a'])
  expect(tags).toEqual(['web', 'ci'])
})

test.each([
  ['a started task', task('x', { start: new Date(now).toISOString() }), 'started'],
  ['a pending task', task('x'), 'pending'],
  ['a waiting task', task('x', { status: 'waiting' }), 'waiting'],
  ['a completed task', task('x', { status: 'completed' }), 'completed'],
])('a task state is worded as the list shows it: %s', (_name, given, want) => {
  // Act
  const got = stateOf(given, now)

  // Assert
  expect(got).toBe(want)
})
