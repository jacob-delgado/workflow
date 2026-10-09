import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { TaskFacet } from '@/api/generated/types.gen.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { describedTask, makeTaskList, taskFacet } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { firstInEveryOrder, ranked, secondInEveryOrder } from '@/test/tasks.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { TasksPanel } from './TasksPanel.tsx'

// The values the tasks below hold, each as the server labels it.
const { pending, waiting, noPriority, noProject, noTag, noIssue } = taskFacet
const priorityH: TaskFacet = { kind: 'priority', value: 'H', label: 'priority H' }

// unlinked are the fields of a task with no project, tag or issue.
const unlinked = { project: '', tags: [], issue_key: '', issue_url: '' }

const leak = describedTask(
  {
    ...unlinked,
    uuid: 'a',
    id: 1,
    description: 'Fix the token leak',
    status: 'pending',
    priority: '',
    urgency: 9.5,
  },
  {
    state: 'pending',
    facets: [pending, noPriority, noProject, noIssue, noTag],
    searchable: ['fix the token leak', '', '', '#1'],
  },
)
const cert = describedTask(
  {
    ...unlinked,
    uuid: 'b',
    id: 2,
    description: 'Renew the cert',
    status: 'pending',
    priority: 'H',
    urgency: 5,
  },
  {
    state: 'pending',
    facets: [pending, priorityH, noProject, noIssue, noTag],
    searchable: ['renew the cert', '', '', '#2'],
  },
)
const room = describedTask(
  {
    ...unlinked,
    uuid: 'c',
    id: 3,
    description: 'Book the room',
    status: 'waiting',
    wait: '2099-01-02T00:00:00Z',
    priority: '',
    urgency: 1,
  },
  {
    state: 'waiting',
    facets: [waiting, noPriority, noProject, noIssue, noTag],
    searchable: ['book the room', '', '', '#3'],
  },
)
// held is pending to Taskwarrior, with a wait still ahead, so the server words
// its state as waiting.
const held = describedTask(
  {
    ...unlinked,
    uuid: 'd',
    id: 4,
    description: 'Call the vendor',
    status: 'pending',
    wait: '2099-01-03T00:00:00Z',
    priority: '',
    urgency: 0.5,
  },
  {
    state: 'waiting',
    facets: [waiting, noPriority, noProject, noIssue, noTag],
    searchable: ['call the vendor', '', '', '#4'],
  },
)

// The places the server gives the token leak and the certificate in a list of
// them, with or without the room after: the token leak first but by priority,
// where the certificate's H comes before none.
const leakRanks = { urgency: 0, state: 0, id: 0, tag: 0, issue: 0, priority: 1 }
const certRanks = { urgency: 1, state: 1, id: 1, tag: 1, issue: 1, priority: 0 }

// offered is the values the tasks above hold, in the order the server offers
// them.
const offered: TaskFacet[] = [pending, waiting, priorityH, noPriority, noProject, noTag, noIssue]

// chips is each chip the filter offers, as it names it, in order.
function chips(): string[] {
  return within(screen.getByRole('group', { name: 'Filter' }))
    .getAllByRole('button')
    .map((chip) => chip.textContent)
}

// factOf is what a task's detail says of it under a term, if it says anything.
function factOf(detail: HTMLElement, term: string): string | undefined {
  const terms = within(detail)
    .getAllByRole('term')
    .map((shown) => shown.textContent)

  return within(detail).getAllByRole('definition')[terms.indexOf(term)]?.textContent
}

function rows(): string[] {
  return within(screen.getByRole('list', { name: 'Tasks' }))
    .getAllByRole('listitem')
    .map((item) => item.textContent)
}

function renderPanel() {
  const listed = [
    ranked(leak, leakRanks),
    ranked(cert, certRanks),
    ranked(room, { urgency: 2, state: 2, id: 2, tag: 2, issue: 2, priority: 2 }),
  ]
  fakeApi({ '/api/tasks': makeTaskList(listed, { facet_order: offered }) })
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

test('a pending task picked as waiting reads waiting in its row, as the server words it', async () => {
  // Arrange
  fakeApi({
    '/api/tasks': makeTaskList(
      [ranked(leak, firstInEveryOrder), ranked(held, secondInEveryOrder)],
      { facet_order: offered },
    ),
  })
  renderWithClient(<TasksPanel />)
  const narrow = await screen.findByRole('group', { name: 'Filter' })

  // Act
  await userEvent.click(within(narrow).getByRole('button', { name: 'waiting 1' }))

  // Assert
  expect(rows()).toEqual([expect.stringMatching(/^waiting#4Call the vendor/)])
})

test("a pending task picked as waiting has the server's state in its detail", async () => {
  // Arrange
  fakeApi({
    '/api/tasks': makeTaskList(
      [ranked(leak, firstInEveryOrder), ranked(held, secondInEveryOrder)],
      { facet_order: offered },
    ),
  })
  renderWithClient(<TasksPanel />)
  const narrow = await screen.findByRole('group', { name: 'Filter' })

  // Act
  await userEvent.click(within(narrow).getByRole('button', { name: 'waiting 1' }))

  // Assert
  expect(factOf(screen.getByRole('article'), 'State')).toBe('waiting')
})

test('the filter offers its chips in the order the server offers them, as it labels them', async () => {
  // Arrange
  const order: TaskFacet[] = [
    { kind: 'priority', value: 'H', label: 'priority High' },
    { kind: 'state', value: 'pending', label: 'pending' },
  ]
  fakeApi({
    '/api/tasks': makeTaskList([ranked(leak, leakRanks), ranked(cert, certRanks)], {
      facet_order: order,
    }),
  })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  await screen.findByRole('group', { name: 'Filter' })
  expect(chips()).toEqual(['priority High 1', 'pending 2'])
})

test('typed text matches the fields the server says it matches', async () => {
  // Arrange
  // The second field is one the page could not find in the task's own, so the
  // match is seen to read the server's.
  const known = describedTask(leak, { ...leak, searchable: ['fix the token leak', 'p-7 ops'] })
  fakeApi({ '/api/tasks': makeTaskList([ranked(known, leakRanks), ranked(cert, certRanks)]) })
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
  const later = describedTask(cert, {
    state: 'waiting',
    facets: [waiting, priorityH, noProject, noIssue, noTag],
    searchable: cert.searchable,
  })
  fakeApi({ '/api/tasks': makeTaskList([ranked(leak, leakRanks), ranked(later, certRanks)]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(await screen.findByText('1 waiting')).toBeTruthy()
  expect(rows()).toEqual([expect.stringMatching(/Fix the token leak/)])
})

test('a picked value the server no longer offers comes last, at zero', async () => {
  // Arrange
  useUiStore.setState({ taskFilter: [{ kind: 'project', value: 'gone', label: 'project gone' }] })
  fakeApi({ '/api/tasks': makeTaskList([leak], { facet_order: offered }) })

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
