import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, within, type RenderResult } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import type { Task, TasksSummary } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import {
  describedTask,
  makeSnapshot,
  makeTaskList,
  standing,
  taskStanding,
} from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import {
  firstInEveryOrder,
  ranked,
  secondInEveryOrder,
  tracking,
  trackingDone,
} from '@/test/tasks.ts'
import { IssueTasks } from './IssueTasks.tsx'

// How the issue's Tasks card keeps up with a write before the stream does:
// the list the write answered laid over the stream's frame, whichever is newer
// standing, and a track remembered until a frame catches up with it. A done
// is IssueTasks.done.test.tsx's.

const issueKey = 'PROJ-412'

const trackingPath = `/api/tasks/${tracking.uuid}`

// streamTasks puts a stream frame on screen whose task summary is summary,
// landed at receivedAt — now, unless a test says otherwise.
function streamTasks(summary: Partial<TasksSummary>, receivedAt = Date.now()) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ tasks: { available: true, reason: '', linked: [], ...summary } }),
    receivedAt,
  })
}

// streamLinked puts a stream frame on screen linking the tasks to issues.
function streamLinked(...linked: Task[]) {
  streamTasks({ linked })
}

// oneClient renders each of its calls' trees under one query client, as the
// page does, so what one tree's write caches another reads.
function oneClient(): (ui: ReactElement) => RenderResult {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })

  return (ui) => render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

test('a task started from the card shows as started at once, ahead of the stream', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({
    [`${trackingPath}/start`]: makeTaskList([
      taskStanding(tracking, { start: new Date().toISOString() }, standing.started),
    ]),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Start task 12' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Stop task 12' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Start task 12' })).toBeNull()
})

test('a card opened again before the stream has the tracked task never offers Track', async () => {
  // Arrange
  const user = userEvent.setup()
  const renderShared = oneClient()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([tracking], { added: tracking.uuid }) })
  const { unmount } = renderShared(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))
  await screen.findByText('Task 12 tracks PROJ-412.')
  unmount()

  // Act
  renderShared(<IssueTasks issueKey={issueKey} />)

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  expect(within(card).queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
  expect(within(card).queryByText('No task tracks PROJ-412.')).toBeNull()
  expect(within(card).getByRole('listitem').textContent).toMatch(/#12.*Redact tokens/)
})

test('a frame that lands after a write shows the task as the stream has it', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({
    [`${trackingPath}/start`]: makeTaskList([
      taskStanding(tracking, { start: new Date().toISOString() }, standing.started),
    ]),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Start task 12' }))
  await screen.findByRole('button', { name: 'Stop task 12' })

  // Act
  // Stopped again in a terminal: the stream's next frame, which lands after
  // the write's answer, has the task pending.
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Start task 12' })).toBeTruthy()
})

test('a frame newer than the list that has the task done offers Track', async () => {
  // Arrange
  // The list the start answered holds the task still to do; the frame that
  // lands after it has the task done, marked done in a terminal since.
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({
    [`${trackingPath}/start`]: makeTaskList([
      taskStanding(tracking, { start: new Date().toISOString() }, standing.started),
    ]),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Start task 12' }))
  await screen.findByRole('button', { name: 'Stop task 12' })

  // Act
  act(() => {
    streamTasks({ linked: [trackingDone(new Date().toISOString())] }, Date.now() + 1)
  })

  // Assert
  expect(markShape(screen.getByRole('listitem'))).toBe(drawnMark('done'))
  expect(screen.getByRole('button', { name: 'Track in Taskwarrior' })).toBeTruthy()
})

test('a frame newer than the list no longer lists a task only the list held', async () => {
  // Arrange
  // The start's answer holds a second task for the issue, added in a terminal;
  // the frame that lands after it holds no such task, deleted there since.
  const user = userEvent.setup()
  const rotating = describedTask(
    {
      ...tracking,
      uuid: '33333333-3333-4333-8333-333333333333',
      id: 13,
      description: 'PROJ-412: Rotate the leaked token',
    },
    {
      state: 'pending',
      facets: tracking.facets,
      searchable: ['proj-412: rotate the leaked token', '', 'proj-412', '#13'],
    },
  )
  streamLinked(tracking)
  fakeApi({
    // Started, task 12 is first among the two in every order; task 13 is as
    // urgent, but numbered after it.
    [`${trackingPath}/start`]: makeTaskList([
      ranked(
        taskStanding(tracking, { start: new Date().toISOString() }, standing.started),
        firstInEveryOrder,
      ),
      ranked(rotating, secondInEveryOrder),
    ]),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Start task 12' }))
  await screen.findByRole('button', { name: 'Start task 13' })

  // Act
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  expect(within(card).getAllByRole('listitem')).toHaveLength(1)
  expect(within(card).queryByRole('button', { name: 'Start task 13' })).toBeNull()
})

test("a frame that lands in the same millisecond as a write's answer leaves the answer's version", async () => {
  // Arrange
  vi.useFakeTimers({ toFake: ['Date'] })
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({
    [`${trackingPath}/start`]: makeTaskList([
      taskStanding(tracking, { start: new Date().toISOString() }, standing.started),
    ]),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Start task 12' }))
  await screen.findByRole('button', { name: 'Stop task 12' })

  // Act
  act(() => {
    streamTasks({ linked: [tracking] })
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Stop task 12' })).toBeTruthy()
})

test('the first frame after a track that does not hold its task still offers no Track', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([], { added: tracking.uuid }) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))
  await screen.findByText('Task 5f1d7a3c tracks PROJ-412.')

  // Act
  act(() => {
    streamTasks({ linked: [] }, Date.now() + 1)
  })

  // Assert
  expect(screen.queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
})

test('a track whose task the context hides, deleted before a frame holds it, offers Track again two frames on', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([], { added: tracking.uuid }) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))
  await screen.findByText('Task 5f1d7a3c tracks PROJ-412.')
  act(() => {
    streamTasks({ linked: [] }, Date.now() + 1)
  })

  // Act
  act(() => {
    streamTasks({ linked: [] }, Date.now() + 2)
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Track in Taskwarrior' })).toBeTruthy()
})

test('a card opened again before the stream holds a task tracked outside the context offers no Track', async () => {
  // Arrange
  const user = userEvent.setup()
  const renderShared = oneClient()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([], { added: tracking.uuid }) })
  const { unmount } = renderShared(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))
  await screen.findByText('Task 5f1d7a3c tracks PROJ-412.')
  unmount()

  // Act
  renderShared(<IssueTasks issueKey={issueKey} />)

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  expect(within(card).queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
  expect(within(card).queryByText('No task tracks PROJ-412.')).toBeNull()
})

test('once a frame holds the tracked task, the issue can be tracked again when it is done', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([], { added: tracking.uuid }) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))
  await screen.findByText('Task 5f1d7a3c tracks PROJ-412.')
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Act
  act(() => {
    streamTasks({ linked: [trackingDone(new Date().toISOString())] }, Date.now() + 2)
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Track in Taskwarrior' })).toBeTruthy()
})
