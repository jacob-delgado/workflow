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
import { tracking, trackingDone } from '@/test/tasks.ts'
import { IssueTasks } from './IssueTasks.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// How the issue's Tasks card keeps up with a write before the stream does:
// the list the write answered laid over the stream's frame, whichever is newer
// standing, and what the page has just done remembered until a frame catches
// up with it.

const issueKey = 'PROJ-412'

const trackingPath = `/api/tasks/${tracking.uuid}`

// doneAnswer is what a done of the tracking task answers: the pending list,
// which no longer holds it, and the task as it stands done.
const doneAnswer = makeTaskList([], { done: trackingDone(new Date().toISOString()) })

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
    [`${trackingPath}/start`]: makeTaskList([
      taskStanding(tracking, { start: new Date().toISOString() }, standing.started),
      rotating,
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

test('a task marked done from the card shows done at once, with nothing left to do on it', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: doneAnswer })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))

  // Assert
  await screen.findByText('Marked task 12 done.')
  const row = screen.getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('done'))
  expect(row.textContent).toMatch(/completed$/)
  expect(within(row).queryAllByRole('button')).toEqual([])
})

test("a task marked done from the card shows as the done's answer describes it", async () => {
  // Arrange
  // A hook ran as the task was marked done and noted where the fix shipped;
  // the answer describes the task as Taskwarrior holds it after.
  const user = userEvent.setup()
  const finished = trackingDone(new Date().toISOString())
  const shipped = describedTask(
    { ...finished, description: `${finished.description} (shipped in 2.3)` },
    {
      state: finished.state,
      facets: finished.facets,
      searchable: [
        'proj-412: redact tokens before they reach the request log (shipped in 2.3)',
        '',
        'proj-412',
      ],
    },
  )
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: makeTaskList([], { done: shipped }) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))

  // Assert
  await screen.findByText('Marked task 12 done.')
  const row = screen.getByRole('listitem')
  expect(row.textContent).toContain('(shipped in 2.3)')
  expect(markShape(row)).toBe(drawnMark('done'))
})

test('a done whose answer describes no task shows it as the stream has it until a frame does', async () => {
  // Arrange
  // The done landed, but the server could not read the task again after it.
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: makeTaskList([]) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))

  // Assert
  await screen.findByText('Marked task 12 done.')
  const row = screen.getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('not-started'))
  expect(within(row).getByRole('button', { name: 'Start task 12' })).toBeTruthy()
})

test('a card opened again before the stream has the done still shows the task done', async () => {
  // Arrange
  const user = userEvent.setup()
  const renderShared = oneClient()
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: doneAnswer })
  const { unmount } = renderShared(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))
  await screen.findByText('Marked task 12 done.')
  unmount()

  // Act
  renderShared(<IssueTasks issueKey={issueKey} />)

  // Assert
  const row = screen.getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('done'))
  expect(within(row).queryAllByRole('button')).toEqual([])
})

test.each([
  ['has the task done', [trackingDone(new Date().toISOString())]],
  ['no longer holds the task', []],
])(
  'once a frame %s, a later frame that brings the task back offers it again',
  async (_, caughtUp) => {
    // Arrange
    // Undone in a terminal after the stream caught up with the done: the later
    // frame has the task still to do again.
    const user = userEvent.setup()
    streamLinked(tracking)
    fakeApi({ [`${trackingPath}/done`]: doneAnswer })
    renderWithClient(<IssueTasks issueKey={issueKey} />)
    await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
    await user.click(screen.getByRole('button', { name: 'Mark done' }))
    await screen.findByText('Marked task 12 done.')
    act(() => {
      streamTasks({ linked: caughtUp }, Date.now() + 1)
    })

    // Act
    act(() => {
      streamTasks({ linked: [tracking] }, Date.now() + 2)
    })

    // Assert
    expect(screen.getByRole('button', { name: 'Start task 12' })).toBeTruthy()
  },
)

test('an undo that brings back a task done from the card offers it again, frame or no', async () => {
  // Arrange
  // The undo lands before the stream has caught up with the done, so no frame
  // ever has the task done.
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({
    '/api/tasks': makeTaskList([]),
    [`${trackingPath}/done`]: doneAnswer,
    '/api/tasks/undo': makeTaskList([tracking], { said: 'reverted 1 operation' }),
  })
  renderWithClient(
    <>
      <IssueTasks issueKey={issueKey} />
      <TasksPanel />
    </>,
  )
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))
  await screen.findByText('Marked task 12 done.')
  await user.click(screen.getByRole('button', { name: 'Undo…' }))
  await user.click(screen.getByRole('button', { name: 'Undo' }))
  await screen.findByText('Undone: reverted 1 operation')

  // Act
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Start task 12' })).toBeTruthy()
})

test('the first frame after a done that still has the task to do leaves it done', async () => {
  // Arrange
  // A frame read before the done can land after its answer.
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: doneAnswer })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))
  await screen.findByText('Marked task 12 done.')

  // Act
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Assert
  const row = screen.getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('done'))
  expect(within(row).queryAllByRole('button')).toEqual([])
})

test('a frame received in the millisecond of a done does not count toward letting it go', async () => {
  // Arrange
  // The frame that lands in the same millisecond as the done may have been
  // read before it, so the frame after it is only the first to count.
  vi.useFakeTimers({ toFake: ['Date'] })
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: doneAnswer })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))
  await screen.findByText('Marked task 12 done.')
  act(() => {
    streamTasks({ linked: [tracking] })
  })

  // Act
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Assert
  const row = screen.getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('done'))
  expect(within(row).queryAllByRole('button')).toEqual([])
})

test('a done undone in a terminal before the next frame shows the task to do again two frames on', async () => {
  // Arrange
  // No frame ever has the task done: the first after the done may have been
  // read before it, and the second was read after it, and holds it pending.
  const user = userEvent.setup()
  streamLinked(tracking)
  fakeApi({ [`${trackingPath}/done`]: doneAnswer })
  renderWithClient(<IssueTasks issueKey={issueKey} />)
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))
  await screen.findByText('Marked task 12 done.')
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 1)
  })

  // Act
  act(() => {
    streamTasks({ linked: [tracking] }, Date.now() + 2)
  })

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  const row = within(card).getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('not-started'))
  expect(within(row).getByRole('button', { name: 'Start task 12' })).toBeTruthy()
  expect(within(row).getByRole('button', { name: 'Mark done… task 12' })).toBeTruthy()
  expect(within(card).queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
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
