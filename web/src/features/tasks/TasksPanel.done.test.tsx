import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { TaskList } from '@/api/generated/types.gen.ts'
import { fakeApi, held } from '@/test/fakeApi.ts'
import { makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { tokenLeak } from '@/test/tasks.ts'
import { TasksPanel } from './TasksPanel.tsx'

// A task marked done from the Tasks section's detail: the list a done answers
// no longer holds it, and until a list is read after the done, the one from
// before it offers nothing that would start it or mark it done again.

const tasksPath = '/api/tasks'

const donePath = `${tasksPath}/${tokenLeak.uuid}/done`

test('a done whose list could not be read again offers no Start or Mark done on the task while it is read once more', async () => {
  // Arrange
  // The done landed, but the server could not read the list after it; the
  // page reads the list again itself, and that read has not answered yet. The
  // clock moves on between the first read and the done, as Taskwarrior takes
  // its time over a done.
  vi.useFakeTimers({ toFake: ['Date'] })
  const user = userEvent.setup()
  const reread = held<TaskList>()
  const lists = [makeTaskList([tokenLeak])]
  fakeApi({
    [tasksPath]: () => lists.shift() ?? reread.promise,
    [donePath]: makeTaskList([], {
      available: false,
      reason:
        'The change was made, but your tasks could not be read again: Taskwarrior did not answer in time',
      reason_code: 'unavailable',
    }),
  })
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: 'Mark done…' }))
  vi.setSystemTime(Date.now() + 1_000)

  // Act
  await user.click(screen.getByRole('button', { name: 'Mark done' }))

  // Assert
  await screen.findByText('Marked task 1 done.')
  expect(screen.queryByRole('button', { name: 'Start' })).toBeNull()
  expect(screen.queryByRole('button', { name: 'Mark done…' })).toBeNull()
})
