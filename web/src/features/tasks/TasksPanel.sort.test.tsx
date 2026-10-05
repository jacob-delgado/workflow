import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

const urgent = makeTask({
  uuid: 'a',
  id: 1,
  description: 'Fix the token leak',
  urgency: 9.5,
  issue_key: '',
})
const certificate = makeTask({
  uuid: 'b',
  id: 2,
  description: 'Renew the certificate',
  urgency: 5.1,
  priority: 'H',
  issue_key: '',
})
const cache = makeTask({
  uuid: 'c',
  id: 3,
  description: 'Tune the cache',
  urgency: 3.2,
  priority: 'L',
  tags: ['perf'],
  issue_key: '',
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
    expect.stringMatching(/no tags/),
  ])
})
