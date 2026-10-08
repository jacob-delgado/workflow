import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fakeApi, held } from '@/test/fakeApi.ts'
import { makeTask, makeTaskList } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

// Marking done, undoing and syncing each wait on a last look: a sync leaves
// the machine, Taskwarrior has no redo, and done runs the task's hooks.

const tasksPath = '/api/tasks'
const tokenLeak = makeTask({
  uuid: '11111111-1111-4111-8111-111111111111',
  id: 1,
  description: 'PROJ-1: Fix the token leak',
  issue_key: 'PROJ-1',
})

const looks = [
  {
    opener: 'Mark done…',
    question: 'Mark task 1 done?',
    confirm: 'Mark done',
    path: `${tasksPath}/${tokenLeak.uuid}/done`,
  },
  {
    opener: 'Undo…',
    question: "Undo Taskwarrior's last change?",
    confirm: 'Undo',
    path: `${tasksPath}/undo`,
  },
  {
    opener: 'Sync…',
    question: 'Sync Taskwarrior with its server?',
    confirm: 'Sync',
    path: `${tasksPath}/sync`,
  },
]

// servingTasks answers the list and every write with the token leak listed,
// a sync backend set; it returns every request made.
function servingTasks(): Request[] {
  const listed = makeTaskList([tokenLeak], { sync_available: true })

  return fakeApi(
    Object.fromEntries(
      [tasksPath, ...looks.map((look) => look.path)].map((path) => [path, listed]),
    ),
  )
}

// postedPaths are the paths of the writes the page sent.
function postedPaths(requests: Request[]): string[] {
  return requests
    .filter((request) => request.method === 'POST')
    .map((request) => new URL(request.url).pathname)
}

test.each(looks)('$opener asks first and sends nothing until $confirm', async (look) => {
  // Arrange
  const user = userEvent.setup()
  const requests = servingTasks()
  renderWithClient(<TasksPanel />)

  // Act: open the look
  await user.click(await screen.findByRole('button', { name: look.opener }))

  // Assert: it asks, and nothing is sent
  const asked = screen.getByRole('group', { name: look.question })
  expect(postedPaths(requests)).toEqual([])

  // Act: confirm
  await user.click(within(asked).getByRole('button', { name: look.confirm }))

  // Assert: the write is sent once
  await screen.findByRole('button', { name: look.opener })
  expect(postedPaths(requests)).toEqual([look.path])
})

test.each(looks)('a refused $confirm says why, with focus still on it', async (look) => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({
    [tasksPath]: makeTaskList([tokenLeak], { sync_available: true }),
    [look.path]: () =>
      Response.json({ code: 'unprocessable', detail: 'Taskwarrior refused' }, { status: 422 }),
  })
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: look.opener }))
  const confirm = within(screen.getByRole('group', { name: look.question })).getByRole('button', {
    name: look.confirm,
  })

  // Act
  await user.click(confirm)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('Taskwarrior refused')
  expect(document.activeElement).toBe(confirm)
})

test.each(looks)('Cancel on the look $question sends nothing', async (look) => {
  // Arrange
  const user = userEvent.setup()
  const requests = servingTasks()
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: look.opener }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.queryByRole('group', { name: look.question })).toBeNull()
  expect(document.activeElement).toBe(screen.getByRole('button', { name: look.opener }))
  expect(postedPaths(requests)).toEqual([])
})

test.each(looks)('Cancel on the look $question is held while $confirm goes', async (look) => {
  // Arrange
  const user = userEvent.setup()
  const answer = held<Response>()
  fakeApi({
    [tasksPath]: makeTaskList([tokenLeak], { sync_available: true }),
    [look.path]: () => answer.promise,
  })
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: look.opener }))
  const asked = screen.getByRole('group', { name: look.question })
  await user.click(within(asked).getByRole('button', { name: look.confirm }))

  // Act
  await user.click(within(asked).getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(screen.getByRole('group', { name: look.question })).toBe(asked)
  expect(within(asked).getByRole('button', { name: 'Cancel' }).getAttribute('aria-disabled')).toBe(
    'true',
  )
})
