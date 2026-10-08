import type { Task, TaskFacet } from '@/api/generated/types.gen.ts'
import { makeTask } from '@/test/fixtures.ts'
import {
  listsWaiting,
  matchesNarrowing,
  taskFacetChoices,
  toggleTaskFacet,
  type TaskNarrowing,
} from './taskFacets.ts'

// The page counts, picks and admits over the facets and fields the server
// describes; what a task holds, how a value is labeled, the order values are
// offered in and the fields typed text matches are the server's, pinned by
// internal/taskwarrior/narrow_test.go.

function narrowedTasks(): Task[] {
  return [
    makeTask({
      uuid: 'n1',
      id: 4,
      description: 'Fix the token leak',
      start: '2026-10-05T12:00:00Z',
      priority: 'H',
      project: 'api',
      tags: ['web'],
      issue_key: 'PROJ-1',
    }),
    makeTask({
      uuid: 'n2',
      id: 7,
      description: 'Renew the cert',
      project: 'infra',
      tags: ['ci'],
      issue_key: '',
    }),
    makeTask({
      uuid: 'n3',
      id: 9,
      description: 'Tune the cache',
      priority: 'L',
      issue_key: 'PROJ-2',
    }),
    makeTask({
      uuid: 'n4',
      id: 0,
      description: 'Book the room',
      status: 'waiting',
      issue_key: '',
    }),
  ]
}

// held is the facet of kind holding value, as the tasks above carry it.
function held(kind: TaskFacet['kind'], value: string): TaskFacet {
  const facet = narrowedTasks()
    .flatMap((task) => task.facets)
    .find((one) => one.kind === kind && one.value === value)
  if (facet === undefined) {
    throw new Error(`no task holds ${kind} ${value}`)
  }

  return facet
}

function admitted(narrowing: TaskNarrowing): string[] {
  return narrowedTasks()
    .filter((task) => matchesNarrowing(narrowing, task))
    .map((task) => task.uuid)
}

test.each<[string, () => TaskNarrowing, string[]]>([
  [
    'nothing picked or typed lets every task through',
    () => ({ picked: [], text: '' }),
    ['n1', 'n2', 'n3', 'n4'],
  ],
  [
    'values picked in one kind widen',
    () => ({ picked: [held('priority', 'H'), held('priority', 'L')], text: '' }),
    ['n1', 'n3'],
  ],
  [
    'values picked in two kinds narrow together',
    () => ({ picked: [held('issue', 'linked'), held('state', 'started')], text: '' }),
    ['n1'],
  ],
  ['no tag is a value of its own', () => ({ picked: [held('tag', '')], text: '' }), ['n3', 'n4']],
  ['typed text matches a field, ignoring case', () => ({ picked: [], text: 'CERT' }), ['n2']],
  [
    'typed text and picks narrow together',
    () => ({ picked: [held('project', 'infra')], text: 'the' }),
    ['n2'],
  ],
])('a narrowing lets through the tasks it picks: %s', (_name, narrowing, want) => {
  // Act
  const got = admitted(narrowing())

  // Assert
  expect(got).toEqual(want)
})

test('typed text matches one field at a time', () => {
  // Act
  const got = admitted({ picked: [], text: 'api fix' })

  // Assert
  expect(got).toEqual([])
})

test('typed text is lower-cased letter by letter, as the server lowers the fields', () => {
  // Arrange
  // The server lowers İ to i alone, where JavaScript's own lower case is two
  // letters.
  const task = makeTask({ searchable: ['istanbul office'] })

  // Act
  const got = matchesNarrowing({ picked: [], text: 'İSTANBUL' }, task)

  // Assert
  expect(got).toBe(true)
})

test('choices count the values the server offers, in its order, a waiting task by its state alone', () => {
  // Arrange
  const order = [
    held('state', 'waiting'),
    held('tag', ''),
    held('project', 'infra'),
    held('state', 'pending'),
  ]

  // Act
  const choices = taskFacetChoices(narrowedTasks(), order, [])

  // Assert
  expect(choices.map((choice) => `${choice.value.label} ${String(choice.count)}`)).toEqual([
    'waiting 1',
    'no tag 1',
    'project infra 1',
    'pending 2',
  ])
})

test('a picked value the server no longer offers comes last, at zero', () => {
  // Arrange
  const gone: TaskFacet = { kind: 'project', value: 'gone', label: 'project gone' }

  // Act
  const choices = taskFacetChoices(narrowedTasks(), [held('priority', 'H')], [gone])

  // Assert
  expect(choices).toEqual([
    { value: held('priority', 'H'), count: 1 },
    { value: gone, count: 0 },
  ])
})

test('toggling a picked value unpicks it', () => {
  // Arrange
  const high = held('priority', 'H')
  const infra = held('project', 'infra')

  // Act
  const picked = toggleTaskFacet([high, infra], high)

  // Assert
  expect(picked).toEqual([infra])
})

test.each<[string, () => TaskFacet[], boolean]>([
  ['nothing picked', () => [], false],
  ['pending picked', () => [held('state', 'pending')], false],
  ['waiting picked', () => [held('state', 'waiting')], true],
  ['a priority only', () => [held('priority', 'H')], false],
])('only picking waiting lists waiting tasks: %s', (_name, picked, want) => {
  // Act
  const got = listsWaiting(picked())

  // Assert
  expect(got).toBe(want)
})
