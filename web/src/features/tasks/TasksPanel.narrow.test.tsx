import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { TaskFacet } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { useUiStore } from '@/shell/uiStore.ts'
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

// chips is each chip the filter offers, as it names it, in order.
function chips(): string[] {
  return within(screen.getByRole('group', { name: 'Filter' }))
    .getAllByRole('button')
    .map((chip) => chip.textContent)
}

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
  const filter = await screen.findByRole('searchbox', { name: 'Search' })

  // Act
  await userEvent.type(filter, 'CERT')

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Renew the cert/)])
  // The count is of the tasks the list shows unnarrowed, so the waiting one is
  // not among them.
  expect(screen.getByText('1 of 2 tasks matches, most urgent first.')).toBeTruthy()
})

test('a narrow chip narrows the list to the value it names', async () => {
  // Arrange
  renderPanel()
  const narrow = await screen.findByRole('group', { name: 'Filter' })

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
  const narrow = await screen.findByRole('group', { name: 'Filter' })

  // Act
  await userEvent.click(within(narrow).getByRole('button', { name: 'waiting 1' }))

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Book the room.*waits until 2099-01-02/)])
})

test('the filter offers its chips in the order the server offers them, as it labels them', async () => {
  // Arrange
  const order: TaskFacet[] = [
    { kind: 'priority', value: 'H', label: 'priority High' },
    { kind: 'state', value: 'pending', label: 'pending' },
  ]
  fakeApi({ '/api/tasks': makeTaskList([leak, cert], { facet_order: order }) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  await screen.findByRole('group', { name: 'Filter' })
  expect(chips()).toEqual(['priority High 1', 'pending 2'])
})

test('typed text matches the fields the server says it matches', async () => {
  // Arrange
  const known = makeTask({ ...leak, searchable: ['fix the token leak', 'p-7 ops'] })
  fakeApi({ '/api/tasks': makeTaskList([known, cert]) })
  renderWithClient(<TasksPanel />)
  const filter = await screen.findByRole('searchbox', { name: 'Search' })

  // Act
  await userEvent.type(filter, 'P-7 OPS')

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/Fix the token leak/)])
})

test('a task the server reads as waiting is counted, not listed', async () => {
  // Arrange
  // Pending, with no wait of its own the page could read, but the server says
  // it waits.
  const later = makeTask({ ...cert, state: 'waiting' })
  fakeApi({ '/api/tasks': makeTaskList([leak, later]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(await screen.findByText('1 waiting')).toBeTruthy()
  expect(rows()).toEqual([expect.stringMatching(/Fix the token leak/)])
})

test('a picked value the server no longer offers comes last, at zero', async () => {
  // Arrange
  useUiStore.setState({ taskFilter: [{ kind: 'project', value: 'gone', label: 'project gone' }] })
  fakeApi({ '/api/tasks': makeTaskList([leak]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  await screen.findByRole('group', { name: 'Filter' })
  expect(chips().at(-1)).toBe('project gone 0')
})

test('a filter matching nothing says so', async () => {
  // Arrange
  renderPanel()
  const filter = await screen.findByRole('searchbox', { name: 'Search' })

  // Act
  await userEvent.type(filter, 'zzz')

  // Assert
  // Said on screen in the list's place, and to a screen reader in the status.
  expect(screen.getAllByText('No task matches the filters.')).toHaveLength(2)
})

test('unpicking the last chip, which no task holds, leaves focus on the filter', async () => {
  // Arrange
  // A pick kept from earlier, with no task holding it any more: its chip is
  // the group's last, and goes when it is unpicked.
  useUiStore.setState({ taskFilter: [{ kind: 'project', value: 'gone', label: 'project gone' }] })
  fakeApi({ '/api/tasks': makeTaskList([]) })
  renderWithClient(<TasksPanel />)
  const chip = await screen.findByRole('button', { name: 'project gone 0' })

  // Act
  await userEvent.click(chip)

  // Assert
  expect(document.activeElement).toBe(screen.getByRole('searchbox', { name: 'Search' }))
})
