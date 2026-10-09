import { screen, within } from '@testing-library/react'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { describedTask, makeSnapshot, standing, taskFacet, taskStanding } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { startedTokenLeak, tokenLeak as tokenLeakTask } from '@/test/tasks.ts'
import { IssuesPanel } from './IssuesPanel.tsx'

// How each issue's row marks the tasks that track it: started, still to do,
// every one done, or none, and nothing while Taskwarrior is unavailable.

const tokenLeak: Issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
}

const setupDocs: Issue = {
  key: 'PROJ-2',
  tracker: 'jira',
  summary: 'Write the setup docs',
  status: 'To Do',
  status_category: 'new',
  type: 'Task',
}

// taskMarkRows lists four issues — one whose task is started, one whose task is
// still to do, one whose every task is completed, and one no task tracks —
// under a stream frame whose task summary is available as given.
function taskMarkRows(available: boolean) {
  const shipped: Issue = { ...setupDocs, key: 'PROJ-3', summary: 'Ship the release' }
  const untracked: Issue = { ...setupDocs, key: 'PROJ-4', summary: 'Plan the next one' }
  const linkedFacets = [
    taskFacet.noPriority,
    taskFacet.noProject,
    taskFacet.withIssue,
    taskFacet.noTag,
  ]
  const unlinkedFields = { project: '', priority: '', tags: [] }
  const linked = [
    startedTokenLeak('2026-09-28T09:00:00Z'),
    describedTask(
      {
        ...unlinkedFields,
        uuid: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2',
        id: 2,
        description: 'PROJ-2: Write the setup docs',
        status: 'pending',
        issue_key: 'PROJ-2',
        issue_url: 'https://jira.example.com/browse/PROJ-2',
      },
      {
        state: 'pending',
        facets: [taskFacet.pending, ...linkedFacets],
        searchable: ['proj-2: write the setup docs', '', 'proj-2', '#2'],
      },
    ),
    describedTask(
      {
        ...unlinkedFields,
        uuid: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3',
        id: 0,
        description: 'PROJ-3: Ship the release',
        status: 'completed',
        end: '2026-09-28T10:00:00Z',
        issue_key: 'PROJ-3',
        issue_url: 'https://jira.example.com/browse/PROJ-3',
      },
      {
        state: 'completed',
        facets: [taskFacet.completed, ...linkedFacets],
        searchable: ['proj-3: ship the release', '', 'proj-3'],
      },
    ),
  ]
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: {
        total: 4,
        start_at: 0,
        unavailable: [],
        issues: [tokenLeak, setupDocs, shipped, untracked],
      },
      tasks: { available, reason: available ? '' : 'Turned off by taskwarrior.disabled.', linked },
    }),
  })
}

test("marks each issue's task by shape, beside the words for it", () => {
  // Arrange
  taskMarkRows(true)

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const rows = within(screen.getByRole('list', { name: 'Issues' })).getAllByRole('listitem')
  const marked = ['task active', 'tracked', 'task done'].map((words) => {
    const said = screen.getByText(words)

    return [rows.findIndex((row) => row.contains(said)), markShape(said.parentElement ?? said)]
  })
  expect(marked).toEqual([
    [0, drawnMark('in-flight')],
    [1, drawnMark('not-started')],
    [2, drawnMark('done')],
  ])
  expect(rows[3]?.textContent).not.toMatch(/tracked|task active|task done/)
})

test.each([
  [
    'waits until a later date',
    taskStanding(
      tokenLeakTask,
      { status: 'waiting', wait: '2099-01-01T00:00:00Z' },
      standing.waiting,
    ),
  ],
  ['recurs', taskStanding(tokenLeakTask, { status: 'recurring' }, standing.recurring)],
])('an issue whose only task %s is tracked, never done', (_, task) => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: { total: 1, start_at: 0, unavailable: [], issues: [tokenLeak] },
      tasks: { available: true, reason: '', linked: [task] },
    }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const said = screen.getByText('tracked')
  expect(markShape(said.parentElement ?? said)).toBe(drawnMark('not-started'))
  expect(screen.queryByText('task done')).toBeNull()
})

test('draws no task marks when Taskwarrior is unavailable', () => {
  // Arrange
  taskMarkRows(false)

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByRole('list', { name: 'Issues' }).textContent).not.toMatch(
    /tracked|task active|task done/,
  )
})
