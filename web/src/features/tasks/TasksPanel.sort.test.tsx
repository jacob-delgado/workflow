import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fakeApi } from '@/test/fakeApi.ts'
import { describedTask, makeTaskList, taskFacet } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { firstInEveryOrder, ranked } from '@/test/tasks.ts'
import { TasksPanel } from './TasksPanel.tsx'

// unlinked are the fields of a task with no project or issue.
const unlinked = { status: 'pending', project: '', issue_key: '', issue_url: '' } as const

// Each task as the server describes it: its values, as it labels them, the
// fields typed text matches, and its place in every order of the three.
const urgent = describedTask(
  {
    ...unlinked,
    uuid: 'a',
    id: 1,
    description: 'Fix the token leak',
    priority: '',
    tags: [],
    urgency: 9.5,
    ranks: { urgency: 0, state: 0, id: 0, tag: 1, issue: 0, priority: 2 },
  },
  {
    state: 'pending',
    facets: [
      taskFacet.pending,
      taskFacet.noPriority,
      taskFacet.noProject,
      taskFacet.noIssue,
      taskFacet.noTag,
    ],
    searchable: ['fix the token leak', '', '', '#1'],
  },
)
const certificate = describedTask(
  {
    ...unlinked,
    uuid: 'b',
    id: 2,
    description: 'Renew the certificate',
    priority: 'H',
    tags: [],
    urgency: 5.1,
    ranks: { urgency: 1, state: 1, id: 1, tag: 2, issue: 1, priority: 0 },
  },
  {
    state: 'pending',
    facets: [
      taskFacet.pending,
      { kind: 'priority', value: 'H', label: 'priority H' },
      taskFacet.noProject,
      taskFacet.noIssue,
      taskFacet.noTag,
    ],
    searchable: ['renew the certificate', '', '', '#2'],
  },
)
const cache = describedTask(
  {
    ...unlinked,
    uuid: 'c',
    id: 3,
    description: 'Tune the cache',
    priority: 'L',
    tags: ['perf'],
    urgency: 3.2,
    ranks: { urgency: 2, state: 2, id: 2, tag: 0, issue: 2, priority: 1 },
  },
  {
    state: 'pending',
    facets: [
      taskFacet.pending,
      { kind: 'priority', value: 'L', label: 'priority L' },
      taskFacet.noProject,
      taskFacet.noIssue,
      { kind: 'tag', value: 'perf', label: '+perf' },
    ],
    searchable: ['tune the cache', '', '', '+perf', '#3'],
  },
)

// rows is the description each listed row leads with, in order.
function rows(): string[] {
  return within(screen.getByRole('list', { name: 'Tasks' }))
    .getAllByRole('listitem')
    .map((item) => item.textContent)
}

test('the list opens most urgent first and says so', async () => {
  // Arrange
  fakeApi({ '/api/tasks': makeTaskList([cache, urgent, certificate]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(await screen.findByRole('combobox', { name: 'Sort' })).toHaveProperty('value', 'urgency')
  expect(screen.getByText('3 tasks, most urgent first.')).toBeTruthy()
})

test('sorting by priority lists the tasks by it, shows each one, and says so', async () => {
  // Arrange
  fakeApi({ '/api/tasks': makeTaskList([cache, urgent, certificate]) })
  renderWithClient(<TasksPanel />)
  const sort = await screen.findByRole('combobox', { name: 'Sort' })

  // Act
  await userEvent.selectOptions(sort, 'By priority')

  // Assert
  expect(rows()).toEqual([
    expect.stringMatching(/Renew the certificate.*priority H/),
    expect.stringMatching(/Tune the cache.*priority L/),
    expect.stringMatching(/Fix the token leak.*no priority/),
  ])
  expect(screen.getByText('3 tasks, by priority.')).toBeTruthy()
})

test('sorting by tag shows each task its tags', async () => {
  // Arrange
  // Without the certificate the server ranks the two afresh: the cache first
  // by tag and by priority, the token leak in every other order.
  fakeApi({
    '/api/tasks': makeTaskList([
      ranked(urgent, { urgency: 0, state: 0, id: 0, tag: 1, issue: 0, priority: 1 }),
      ranked(cache, { urgency: 1, state: 1, id: 1, tag: 0, issue: 1, priority: 0 }),
    ]),
  })
  renderWithClient(<TasksPanel />)
  const sort = await screen.findByRole('combobox', { name: 'Sort' })

  // Act
  await userEvent.selectOptions(sort, 'By tag')

  // Assert
  expect(rows()).toEqual([
    expect.stringMatching(/Tune the cache.*\+perf/),
    expect.stringMatching(/Fix the token leak.*no tag(?!s)/),
  ])
})

test('sorting lists the tasks where the server ranks them, whatever their fields say', async () => {
  // Arrange
  // By id the server puts task 3 first and task 1 last, as no field would.
  const ranked = (task: typeof urgent, id: number) => ({ ...task, ranks: { ...task.ranks, id } })
  fakeApi({
    '/api/tasks': makeTaskList([ranked(urgent, 2), ranked(certificate, 1), ranked(cache, 0)]),
  })
  renderWithClient(<TasksPanel />)
  const sort = await screen.findByRole('combobox', { name: 'Sort' })

  // Act
  await userEvent.selectOptions(sort, 'By ID')

  // Assert
  expect(rows()).toEqual([
    expect.stringMatching(/Tune the cache/),
    expect.stringMatching(/Renew the certificate/),
    expect.stringMatching(/Fix the token leak/),
  ])
})

test('sorting by priority shows each task its priority as the server labels it', async () => {
  // Arrange
  // A label the page could not spell from the value, so the row is seen to
  // read the server's.
  const labeled = describedTask(certificate, {
    ...certificate,
    facets: certificate.facets.map((facet) =>
      facet.kind === 'priority' ? { ...facet, label: 'priority High' } : facet,
    ),
  })
  fakeApi({ '/api/tasks': makeTaskList([ranked(labeled, firstInEveryOrder)]) })
  renderWithClient(<TasksPanel />)
  const sort = await screen.findByRole('combobox', { name: 'Sort' })

  // Act
  await userEvent.selectOptions(sort, 'By priority')

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Renew the certificate.*priority High/)])
})
