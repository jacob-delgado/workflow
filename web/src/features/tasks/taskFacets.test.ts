import type { Task } from '@/api/generated/types.gen.ts'
import { makeTask } from '@/test/fixtures.ts'
import {
  listsWaiting,
  matchesNarrowing,
  taskFacetChoices,
  taskFacetLabel,
  type TaskFacet,
  type TaskNarrowing,
} from './taskFacets.ts'

// The twin of internal/taskwarrior/narrow_test.go: the cases carry the same
// names, so a change to one rule is made to both.

const now = Date.parse('2026-10-05T12:00:00Z')

function narrowedTasks(): Task[] {
  return [
    makeTask({
      uuid: 'n1',
      id: 4,
      description: 'Fix the token leak',
      start: new Date(now).toISOString(),
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
      wait: new Date(now + 3_600_000).toISOString(),
      issue_key: '',
    }),
  ]
}

function admitted(narrowing: TaskNarrowing): string[] {
  return narrowedTasks()
    .filter((task) => matchesNarrowing(narrowing, task, now))
    .map((task) => task.uuid)
}

const facet = (kind: TaskFacet['kind'], value: string): TaskFacet => ({ kind, value })

test.each<[string, TaskNarrowing, string[]]>([
  [
    'nothing picked or typed lets every task through',
    { picked: [], text: '' },
    ['n1', 'n2', 'n3', 'n4'],
  ],
  [
    'values picked in one kind widen',
    { picked: [facet('priority', 'H'), facet('priority', 'L')], text: '' },
    ['n1', 'n3'],
  ],
  [
    'values picked in two kinds narrow together',
    { picked: [facet('issue', 'linked'), facet('state', 'started')], text: '' },
    ['n1'],
  ],
  ['no tag is a value of its own', { picked: [facet('tag', '')], text: '' }, ['n3', 'n4']],
  ['typed text matches the description, ignoring case', { picked: [], text: 'CERT' }, ['n2']],
  ['typed text matches a tag written with its plus', { picked: [], text: '+ci' }, ['n2']],
  ['typed text matches an issue key or an id', { picked: [], text: '#9' }, ['n3']],
  [
    'typed text and picks narrow together',
    { picked: [facet('project', 'infra')], text: 'the' },
    ['n2'],
  ],
])('a narrowing lets through the tasks it picks: %s', (_name, narrowing, want) => {
  // Act
  const got = admitted(narrowing)

  // Assert
  expect(got).toEqual(want)
})

test('typed text matches one field at a time', () => {
  // Act
  const got = admitted({ picked: [], text: 'api fix' })

  // Assert
  expect(got).toEqual([])
})

test('choices offer each value with its count', () => {
  // Act
  const choices = taskFacetChoices(narrowedTasks(), [], now)

  // Assert
  expect(
    choices.map((choice) => `${taskFacetLabel(choice.value)} ${String(choice.count)}`),
  ).toEqual([
    'started 1',
    'pending 2',
    'waiting 1',
    'priority H 1',
    'priority L 1',
    'no priority 1',
    'project api 1',
    'project infra 1',
    'no project 1',
    '+ci 1',
    '+web 1',
    'no tag 1',
    'with issue 2',
    'no issue 1',
  ])
})

test('a picked value no task holds is still offered at zero', () => {
  // Act
  const choices = taskFacetChoices(narrowedTasks(), [facet('priority', 'M')], now)

  // Assert
  expect(choices).toContainEqual({ value: facet('priority', 'M'), count: 0 })
})

test.each<[string, TaskFacet[], boolean]>([
  ['nothing picked', [], false],
  ['pending picked', [facet('state', 'pending')], false],
  ['waiting picked', [facet('state', 'waiting')], true],
  ['a priority only', [facet('priority', 'H')], false],
])('only picking waiting lists waiting tasks: %s', (_name, picked, want) => {
  // Act
  const got = listsWaiting(picked)

  // Assert
  expect(got).toBe(want)
})
