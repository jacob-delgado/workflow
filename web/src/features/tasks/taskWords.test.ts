import type { Task, TaskFacet } from '@/api/generated/types.gen.ts'
import { describedTask, makeTask, standing, taskStanding } from '@/test/fixtures.ts'
import { dueWords, elapsedWords, markOf } from './taskWords.ts'

const minute = 60_000
const now = Date.parse('2026-09-28T12:00:00Z')

// before is the RFC 3339 time some minutes before now.
function before(minutes: number): string {
  return new Date(now - minutes * minute).toISOString()
}

test.each([
  ['a started moment ago', 0, '0m'],
  ['under an hour', 12, '12m'],
  ['under a day', 72, '1h12m'],
  ['days', (2 * 24 + 3) * 60 + 20, '2d 3h'],
  ['a start the clock has not reached', -5, '0m'],
])('says how long ago %s was, in the terminal spine words', (_, minutes, said) => {
  // Act
  const words = elapsedWords(before(minutes), now)

  // Assert
  expect(words).toBe(said)
})

test.each([
  ['a date already past', 30, 'overdue'],
  ['the very moment', 0, 'overdue'],
  ['a date still to come', -(27 * 60), 'due in 1d 3h'],
])('says when a task is due for %s', (_, minutes, said) => {
  // Act
  const words = dueWords(before(minutes), now)

  // Assert
  expect(words).toBe(said)
})

// finished is task 12 once Taskwarrior has closed it with status, five
// minutes ago, and the server reads it to stand as standing says: out of the
// working set, so numbered 0, which typed text no longer matches.
function finished(
  status: 'completed' | 'deleted',
  { state, facet }: { state: Task['state']; facet: TaskFacet },
): Task {
  const task12 = makeTask()

  return describedTask(
    { ...task12, id: 0, status, end: before(5) },
    {
      state,
      facets: [facet, ...task12.facets.slice(1)],
      searchable: ['proj-42: redact the token before it reaches the log', '', 'proj-42'],
    },
  )
}

test.each([
  [
    'a started task',
    taskStanding(makeTask(), { start: before(10) }, standing.started),
    'in-flight',
  ],
  ['a task not started', makeTask(), 'not-started'],
  [
    'a waiting task',
    taskStanding(makeTask(), { status: 'waiting', wait: before(-60) }, standing.waiting),
    'not-started',
  ],
  [
    'a recurring task',
    taskStanding(makeTask(), { status: 'recurring' }, standing.recurring),
    'not-started',
  ],
  ['a completed task', finished('completed', standing.completed), 'done'],
  ['a deleted task', finished('deleted', standing.deleted), 'not-started'],
])('marks %s by shape', (_, task, mark) => {
  // Act & Assert
  expect(markOf(task)).toBe(mark)
})
