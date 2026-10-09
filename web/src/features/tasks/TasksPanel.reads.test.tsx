import { QueryClient } from '@tanstack/react-query'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeTaskList } from '@/test/fixtures.ts'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
import { cacheTuning, certificate } from '@/test/tasks.ts'
import { TasksPanel } from './TasksPanel.tsx'

// How the Tasks section reads your tasks: why none can be listed, a read that
// fails and Try again, Refresh, and reading again each time it opens.

const tasksPath = '/api/tasks'

// refused is a problem answer, as the server gives a failed read.
function refused(body: object, status: number): Response {
  return Response.json(body, { status, headers: { 'Content-Type': 'application/problem+json' } })
}

// readsOf counts the reads of the task list among the requests the page made.
function readsOf(requests: Request[]): number {
  return requests.filter(
    (request) => request.method === 'GET' && new URL(request.url).pathname === tasksPath,
  ).length
}

test.each([
  [
    'go-task on PATH, and names the setting that finds Taskwarrior',
    'not_taskwarrior',
    "The task on PATH is another program (go-task, most likely), not Taskwarrior. Set taskwarrior.program to Taskwarrior's path, then restart workflow; workflow doctor names what it found.",
    "The task on PATH is another program (go-task, most likely), not Taskwarrior. Set taskwarrior.program to Taskwarrior's path, then restart workflow; workflow doctor names what it found.",
  ],
  [
    'no Taskwarrior at all',
    'not_installed',
    'Taskwarrior is not installed, or no task program is on PATH. Install Taskwarrior 3.5.0 or newer, or set taskwarrior.program.',
    'Taskwarrior is not installed, or no task program is on PATH. Install Taskwarrior 3.5.0 or newer, or set taskwarrior.program.',
  ],
  [
    'Taskwarrior settings saved since the start',
    'unavailable',
    'Taskwarrior settings changed; restart workflow to apply.',
    'Taskwarrior settings changed; restart workflow to apply.',
  ],
  [
    'a Taskwarrior that could not start',
    'unavailable',
    'Taskwarrior could not start; workflow doctor says why.',
    'Taskwarrior could not start; workflow doctor says why.',
  ],
] as const)('with %s, says why no task can be listed', async (_, code, reason, said) => {
  // Arrange
  fakeApi({
    [tasksPath]: makeTaskList([], { available: false, reason, reason_code: code }),
  })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const why = await screen.findByText(reason, { exact: false })
  expect(why.textContent).toBe(said)
  expect(screen.queryByRole('textbox', { name: 'task add' })).toBeNull()
})

test('a failed read says why, and Try again reads again, one request each', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [
    refused({ detail: 'Taskwarrior did not answer in time' }, 502),
    Response.json(makeTaskList([certificate])),
  ]
  const requests = fakeApi({ [tasksPath]: () => answers.shift() })

  // Act: read the tasks, refused
  renderWithClient(<TasksPanel />)

  // Assert: it says why
  expect((await screen.findByRole('alert')).textContent).toBe('Taskwarrior did not answer in time')

  // Act: try again
  await user.click(screen.getByRole('button', { name: 'Try again' }))

  // Assert: the list is read, one request each
  expect(await screen.findByRole('list', { name: 'Tasks' })).toBeTruthy()
  expect(screen.queryByRole('alert')).toBeNull()
  expect(readsOf(requests)).toBe(2)
})

test('a failed Refresh keeps the list last read in view, beside why, and offers Try again', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [
    Response.json(makeTaskList([certificate])),
    refused({ detail: 'Taskwarrior did not answer in time' }, 502),
  ]
  fakeApi({ [tasksPath]: () => answers.shift() })
  renderWithClient(<TasksPanel />)
  await screen.findByRole('list', { name: 'Tasks' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Refresh' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('Taskwarrior did not answer in time')
  const list = screen.getByRole('list', { name: 'Tasks' })
  expect(within(list).getByRole('button', { name: /renew the certificate/i })).toBeTruthy()
  expect(screen.getByRole('button', { name: 'Try again' })).toBeTruthy()
})

test("a failed read is said at once, never retried behind the user's back", async () => {
  // Arrange
  // The app's own client, whose queries retry by default.
  const requests = fakeApi({ [tasksPath]: () => refused({}, 502) })
  const client = new QueryClient()

  // Act
  renderWithClient(<TasksPanel />, client)

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'Your tasks could not be read. Press Try again.',
  )
  expect(readsOf(requests)).toBe(1)
})

test('Refresh reads the tasks again, keeping the list while it does', async () => {
  // Arrange
  const user = userEvent.setup()
  const answers = [makeTaskList([certificate]), makeTaskList([certificate, cacheTuning])]
  const requests = fakeApi({ [tasksPath]: () => answers.shift() })
  renderWithClient(<TasksPanel />)
  await screen.findByRole('list', { name: 'Tasks' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Refresh' }))

  // Assert
  expect(await screen.findByText('PROJ-9: Tune the cache')).toBeTruthy()
  expect(readsOf(requests)).toBe(2)
})

test('reads the tasks again each time the section opens', async () => {
  // Arrange
  // The app's own defaults, under which a query reads again only when its own
  // staleTime says so.
  const requests = fakeApi({ [tasksPath]: makeTaskList([certificate]) })
  const client = appQueryClient()
  const view = renderWithClient(<TasksPanel />, client)
  await screen.findByRole('list', { name: 'Tasks' })
  view.unmount()

  // Act
  renderWithClient(<TasksPanel />, client)

  // Assert
  await screen.findByRole('list', { name: 'Tasks' })
  await screen.findByRole('button', { name: 'Refresh' })
  expect(readsOf(requests)).toBe(2)
})
