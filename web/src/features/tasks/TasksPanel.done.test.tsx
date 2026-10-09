import { act, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Task, TaskList } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi, held, type Held } from '@/test/fakeApi.ts'
import { makeSnapshot, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { certificate, tokenLeak, tracking, trackingDone } from '@/test/tasks.ts'
import { TasksPanel } from './TasksPanel.tsx'

// A task marked done from the Tasks section's detail: the list a done answers
// no longer holds it, and until a list is answered after the done, the one from
// before it offers nothing that would start it or mark it done again — whatever
// the stream's frames, which this section does not draw from, say meanwhile.

const tasksPath = '/api/tasks'

type User = ReturnType<typeof userEvent.setup>

// markDoneUnread shows task as the only one listed and marks it done; the done
// lands, but the server could not read the list after it, so the page reads
// the list again itself, and that read is held for the test to answer. The
// clock moves on between the first read and the done, as Taskwarrior takes its
// time over a done.
async function markDoneUnread(user: User, task: Task): Promise<Held<TaskList>> {
  vi.useFakeTimers({ toFake: ['Date'] })
  const reread = held<TaskList>()
  const lists = [makeTaskList([task])]
  fakeApi({
    [tasksPath]: () => lists.shift() ?? reread.promise,
    [`${tasksPath}/${task.uuid}/done`]: makeTaskList([], {
      available: false,
      reason:
        'The change was made, but your tasks could not be read again: Taskwarrior did not answer in time',
      reason_code: 'unavailable',
    }),
  })
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: 'Mark done…' }))
  vi.setSystemTime(Date.now() + 1_000)
  await user.click(screen.getByRole('button', { name: 'Mark done' }))
  await screen.findByText(/^Marked task \d+ done\.$/)

  return reread
}

// streamLinked lands a stream frame linking the tasks to issues, received a
// moment after the one before it.
function streamLinked(linked: Task[]) {
  act(() => {
    useSnapshotStore.setState((state) => ({
      status: 'live',
      snapshot: makeSnapshot({ tasks: { available: true, reason: '', linked } }),
      receivedAt: Math.max(Date.now(), state.receivedAt) + 1,
    }))
  })
}

test('a done whose list could not be read again offers no Start or Mark done on the task while it is read once more', async () => {
  // Arrange
  const user = userEvent.setup()

  // Act
  await markDoneUnread(user, tokenLeak)

  // Assert
  expect(screen.queryByRole('button', { name: 'Start' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Mark done…' })).toBeNull()
})

test.each([
  { frames: 'a frame linking no task', task: certificate, linked: [[]] },
  {
    frames: 'a frame with the task done',
    task: tracking,
    linked: [[trackingDone(new Date().toISOString())]],
  },
  {
    frames: 'two frames with the task still to do',
    task: tokenLeak,
    linked: [[tokenLeak], [tokenLeak]],
  },
])(
  'after $frames, a task marked done offers no Start or Mark done while the list is read once more',
  async ({ task, linked }) => {
    // Arrange
    const user = userEvent.setup()
    await markDoneUnread(user, task)

    // Act
    for (const frame of linked) {
      streamLinked(frame)
    }

    // Assert
    expect(screen.getByRole('heading', { name: task.description })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Start' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Mark done…' })).toBeNull()
  },
)

test('a list answered after the done offers the writes on a task it still holds', async () => {
  // Arrange
  // The list read again still holds the task, as it does once a terminal has
  // undone the done meanwhile.
  const user = userEvent.setup()
  const reread = await markDoneUnread(user, tokenLeak)

  // Act
  reread.answer(makeTaskList([tokenLeak]))

  // Assert
  expect(await screen.findByRole('button', { name: 'Start' })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Mark done…' })).toBeTruthy()
})
