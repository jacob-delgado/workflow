import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// Each task's place in every order, as the server ranks them.
const urgent = makeTask({
  uuid: 'a',
  id: 1,
  description: 'Fix the token leak',
  urgency: 9.5,
  issue_key: '',
  ranks: { urgency: 0, state: 0, id: 0, tag: 1, issue: 0, priority: 2 },
})
const certificate = makeTask({
  uuid: 'b',
  id: 2,
  description: 'Renew the certificate',
  urgency: 5.1,
  priority: 'H',
  issue_key: '',
  ranks: { urgency: 1, state: 1, id: 1, tag: 2, issue: 1, priority: 0 },
})
const cache = makeTask({
  uuid: 'c',
  id: 3,
  description: 'Tune the cache',
  urgency: 3.2,
  priority: 'L',
  tags: ['perf'],
  issue_key: '',
  ranks: { urgency: 2, state: 2, id: 2, tag: 0, issue: 2, priority: 1 },
})

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
  fakeApi({ '/api/tasks': makeTaskList([urgent, cache]) })
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
  const labeled = makeTask({
    ...certificate,
    facets: certificate.facets.map((facet) =>
      facet.kind === 'priority' ? { ...facet, label: 'priority High' } : facet,
    ),
  })
  fakeApi({ '/api/tasks': makeTaskList([labeled]) })
  renderWithClient(<TasksPanel />)
  const sort = await screen.findByRole('combobox', { name: 'Sort' })

  // Act
  await userEvent.selectOptions(sort, 'By priority')

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Renew the certificate.*priority High/)])
})
