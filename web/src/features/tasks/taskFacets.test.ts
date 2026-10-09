import type { Task, TaskFacet } from '@/api/generated/types.gen.ts'
import { describedTask, taskFacet } from '@/test/fixtures.ts'
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

// The values the tasks below hold, each as the server labels it.
const { started, pending, waiting, noPriority, noProject, withIssue, noIssue, noTag } = taskFacet
const priorityH: TaskFacet = { kind: 'priority', value: 'H', label: 'priority H' }
const priorityL: TaskFacet = { kind: 'priority', value: 'L', label: 'priority L' }
const projectApi: TaskFacet = { kind: 'project', value: 'api', label: 'project api' }
const projectInfra: TaskFacet = { kind: 'project', value: 'infra', label: 'project infra' }
const tagWeb: TaskFacet = { kind: 'tag', value: 'web', label: '+web' }
const tagCi: TaskFacet = { kind: 'tag', value: 'ci', label: '+ci' }

// narrowedTasks is four tasks as the server describes them: n1 started, n2 and
// n3 pending, and n4 waiting.
function narrowedTasks(): Task[] {
  return [
    describedTask(
      {
        uuid: 'n1',
        id: 4,
        description: 'Fix the token leak',
        status: 'pending',
        start: '2026-10-05T12:00:00Z',
        priority: 'H',
        project: 'api',
        tags: ['web'],
        issue_key: 'PROJ-1',
        issue_url: 'https://jira.example.com/browse/PROJ-1',
      },
      {
        state: 'started',
        facets: [started, priorityH, projectApi, withIssue, tagWeb],
        searchable: ['fix the token leak', 'api', 'proj-1', '+web', '#4'],
      },
    ),
    describedTask(
      {
        uuid: 'n2',
        id: 7,
        description: 'Renew the cert',
        status: 'pending',
        priority: '',
        project: 'infra',
        tags: ['ci'],
        issue_key: '',
        issue_url: '',
      },
      {
        state: 'pending',
        facets: [pending, noPriority, projectInfra, noIssue, tagCi],
        searchable: ['renew the cert', 'infra', '', '+ci', '#7'],
      },
    ),
    describedTask(
      {
        uuid: 'n3',
        id: 9,
        description: 'Tune the cache',
        status: 'pending',
        priority: 'L',
        project: '',
        tags: [],
        issue_key: 'PROJ-2',
        issue_url: 'https://jira.example.com/browse/PROJ-2',
      },
      {
        state: 'pending',
        facets: [pending, priorityL, noProject, withIssue, noTag],
        searchable: ['tune the cache', '', 'proj-2', '#9'],
      },
    ),
    describedTask(
      {
        uuid: 'n4',
        id: 0,
        description: 'Book the room',
        status: 'waiting',
        priority: '',
        project: '',
        tags: [],
        issue_key: '',
        issue_url: '',
      },
      {
        state: 'waiting',
        facets: [waiting, noPriority, noProject, noIssue, noTag],
        searchable: ['book the room', '', ''],
      },
    ),
  ]
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
    () => ({ picked: [priorityH, priorityL], text: '' }),
    ['n1', 'n3'],
  ],
  [
    'values picked in two kinds narrow together',
    () => ({ picked: [withIssue, started], text: '' }),
    ['n1'],
  ],
  ['no tag is a value of its own', () => ({ picked: [noTag], text: '' }), ['n3', 'n4']],
  ['typed text matches a field, ignoring case', () => ({ picked: [], text: 'CERT' }), ['n2']],
  ['typed text and picks narrow together', () => ({ picked: [projectInfra], text: 'the' }), ['n2']],
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
  const task = describedTask(
    {
      id: 12,
      description: 'Book the İstanbul office',
      status: 'pending',
      priority: '',
      project: '',
      tags: [],
      issue_key: '',
      issue_url: '',
    },
    {
      state: 'pending',
      facets: [pending, noPriority, noProject, noIssue, noTag],
      searchable: ['book the istanbul office', '', '', '#12'],
    },
  )

  // Act
  const got = matchesNarrowing({ picked: [], text: 'İSTANBUL' }, task)

  // Assert
  expect(got).toBe(true)
})

test('choices count the values the server offers, in its order, a waiting task by its state alone', () => {
  // Arrange
  const order = [waiting, noTag, projectInfra, pending]

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
  const choices = taskFacetChoices(narrowedTasks(), [priorityH], [gone])

  // Assert
  expect(choices).toEqual([
    { value: priorityH, count: 1 },
    { value: gone, count: 0 },
  ])
})

test('toggling a picked value unpicks it', () => {
  // Act
  const picked = toggleTaskFacet([priorityH, projectInfra], priorityH)

  // Assert
  expect(picked).toEqual([projectInfra])
})

test.each<[string, () => TaskFacet[], boolean]>([
  ['nothing picked', () => [], false],
  ['pending picked', () => [pending], false],
  ['waiting picked', () => [waiting], true],
  ['a priority only', () => [priorityH], false],
])('only picking waiting lists waiting tasks: %s', (_name, picked, want) => {
  // Act
  const got = listsWaiting(picked())

  // Assert
  expect(got).toBe(want)
})
