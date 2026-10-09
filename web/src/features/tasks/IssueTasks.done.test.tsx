import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, within, type RenderResult } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import type { Task, TasksSummary } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { describedTask, makeSnapshot, makeTaskList } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { tracking, trackingDone } from '@/test/tasks.ts'
import { IssueTasks } from './IssueTasks.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// A task marked done from the issue's Tasks card: shown as the done's answer
// describes it, since the list it answers no longer holds it, until the
// stream's frames catch up with the done, or an undo brings the task back.

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

  // Act: mark the task done
  await user.click(screen.getByRole('button', { name: 'Mark done… task 12' }))
  await user.click(screen.getByRole('button', { name: 'Mark done' }))

  // Assert: the task stands as the stream has it
  await screen.findByText('Marked task 12 done.')
  expect(markShape(screen.getByRole('listitem'))).toBe(drawnMark('not-started'))
  expect(
    within(screen.getByRole('listitem')).getByRole('button', { name: 'Start task 12' }),
  ).toBeTruthy()

  // Act: a frame that has the task done lands
  act(() => {
    streamTasks({ linked: [trackingDone(new Date().toISOString())] }, Date.now() + 1)
  })

  // Assert: the task shows done, with nothing left to do on it
  const row = screen.getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('done'))
  expect(within(row).queryAllByRole('button')).toEqual([])
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
