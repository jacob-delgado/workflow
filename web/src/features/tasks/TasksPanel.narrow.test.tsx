import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

const leak = makeTask({
  uuid: 'a',
  id: 1,
  description: 'Fix the token leak',
  urgency: 9.5,
  issue_key: '',
})
const cert = makeTask({
  uuid: 'b',
  id: 2,
  description: 'Renew the cert',
  priority: 'H',
  urgency: 5,
  issue_key: '',
})
const room = makeTask({
  uuid: 'c',
  id: 3,
  description: 'Book the room',
  status: 'waiting',
  wait: '2099-01-02T00:00:00Z',
  urgency: 1,
  issue_key: '',
})

function rows(): string[] {
  return within(screen.getByRole('list', { name: 'Tasks' }))
    .getAllByRole('listitem')
    .map((item) => item.textContent)
}

function renderPanel() {
  fakeApi({ '/api/tasks': makeTaskList([leak, cert, room]) })
  renderWithClient(<TasksPanel />)
}

test('typing a filter narrows the list and says how many match', async () => {
  // Arrange
  renderPanel()
  const filter = await screen.findByRole('searchbox', { name: 'Filter' })

  // Act
  await userEvent.type(filter, 'CERT')

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Renew the cert/)])
  expect(screen.getByText('1 of 3 tasks match, most urgent first.')).toBeTruthy()
})

test('a narrow chip narrows the list to the value it names', async () => {
  // Arrange
  renderPanel()
  const narrow = await screen.findByRole('group', { name: 'Narrow' })

  // Act
  await userEvent.click(within(narrow).getByRole('button', { name: 'priority H 1' }))

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Renew the cert/)])
  expect(within(narrow).getByRole('button', { name: 'priority H 1' })).toHaveProperty(
    'ariaPressed',
    'true',
  )
})

test('picking waiting lists the waiting tasks, each saying until when', async () => {
  // Arrange
  renderPanel()
  const narrow = await screen.findByRole('group', { name: 'Narrow' })

  // Act
  await userEvent.click(within(narrow).getByRole('button', { name: 'waiting 1' }))

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Book the room.*waits until 2099-01-02/)])
})

test('a filter matching nothing says so', async () => {
  // Arrange
  renderPanel()
  const filter = await screen.findByRole('searchbox', { name: 'Filter' })

  // Act
  await userEvent.type(filter, 'zzz')

  // Assert
  // Said on screen in the list's place, and to a screen reader in the status.
  expect(screen.getAllByText('No task matches the filters.')).toHaveLength(2)
})
