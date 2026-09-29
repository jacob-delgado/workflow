import { makeTask } from '@/test/fixtures.ts'
import { dueWords, elapsedWords, markOf, statusWords, waitsAt } from './taskWords.ts'

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

test.each([
  ['a started task', makeTask({ start: before(10) }), 'started', 'in-flight'],
  ['a task not started', makeTask(), 'pending', 'not-started'],
  ['a waiting task', makeTask({ status: 'waiting' }), 'waiting', 'not-started'],
  ['a recurring task', makeTask({ status: 'recurring' }), 'recurring', 'not-started'],
  ['a completed task', makeTask({ status: 'completed', end: before(5) }), 'completed', 'done'],
  ['a deleted task', makeTask({ status: 'deleted' }), 'deleted', 'not-started'],
])("names %s in Taskwarrior's words, and marks it by shape", (_, task, words, mark) => {
  // Act & Assert
  expect([statusWords(task), markOf(task)]).toEqual([words, mark])
})

test.each([
  ['a task Taskwarrior says is waiting', makeTask({ status: 'waiting' }), true],
  ['a pending task hidden until later', makeTask({ wait: before(-60) }), true],
  ['a pending task whose wait is over', makeTask({ wait: before(60) }), false],
  ['a pending task with no wait', makeTask(), false],
])('counts %s as waiting or not', (_, task, waiting) => {
  // Act & Assert
  expect(waitsAt(task, now)).toBe(waiting)
})
