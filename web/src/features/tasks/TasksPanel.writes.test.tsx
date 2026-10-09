import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Task, TaskList } from '@/api/generated/types.gen.ts'
import { useHealthStore } from '@/api/health.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { describedTask, makeHealth, makeTaskList, taskFacet } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import {
  certificate,
  firstInEveryOrder,
  ranked,
  secondInEveryOrder,
  startedTokenLeak,
  tokenLeak,
} from '@/test/tasks.ts'
import { TasksPanel } from './TasksPanel.tsx'

type User = ReturnType<typeof userEvent.setup>

const tasksPath = '/api/tasks'

const started = startedTokenLeak(new Date(Date.now() - 600_000).toISOString())
const taskPath = `${tasksPath}/${tokenLeak.uuid}`

// leakThenCertificate are the token leak and the certificate as the server
// ranks the two: the token leak, more urgent and tracking an issue, first in
// every order.
const leakThenCertificate = [
  ranked(tokenLeak, firstInEveryOrder),
  ranked(certificate, secondInEveryOrder),
]

// refused is a problem answer, as the server gives a write it refuses.
function refused(detail: string, status: number): Response {
  return Response.json(
    { title: 'Conflict', status, detail, code: 'conflict' },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

// serveTasks answers the list's read with shown, and each write — a POST to
// any path under /api/tasks — with the answer the path is given.
function serveTasks(shown: TaskList, writes: Record<string, unknown>): Request[] {
  const routes: Record<string, unknown> = { [tasksPath]: shown }
  for (const [path, answer] of Object.entries(writes)) {
    routes[path] = answer
  }

  return fakeApi(routes)
}

// postsOf are the writes the page sent: each one's path and body.
async function postsOf(requests: Request[]): Promise<[string, unknown][]> {
  const posts = requests.filter((request) => request.method === 'POST')

  return Promise.all(
    posts.map(async (post): Promise<[string, unknown]> => {
      const text = await post.text()

      return [new URL(post.url).pathname, text === '' ? undefined : JSON.parse(text)]
    }),
  )
}

// factOf is what the detail says of a task under a term, if it says anything.
function factOf(term: string): string | undefined {
  const terms = screen.getAllByRole('term').map((shown) => shown.textContent)

  return screen.getAllByRole('definition')[terms.indexOf(term)]?.textContent
}

// statusSaying is the live status line that says text, if one does.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((line) => line.textContent === text)
}

interface Verb {
  name: string
  shown: Task[]
  act: (user: User) => Promise<void>
  path: string
  body?: unknown
  answered: Task[]
  said: string
  // after reports whether the page shows the answer in place of what it showed.
  after: () => boolean
}

const verbs: Verb[] = [
  {
    name: 'Start',
    shown: [tokenLeak],
    act: (user) => user.click(screen.getByRole('button', { name: 'Start' })),
    path: `${taskPath}/start`,
    answered: [started],
    said: 'Started task 1.',
    after: () => screen.queryByRole('button', { name: 'Stop' }) !== null,
  },
  {
    name: 'Stop',
    shown: [started],
    act: (user) => user.click(screen.getByRole('button', { name: 'Stop' })),
    path: `${taskPath}/stop`,
    answered: [tokenLeak],
    said: 'Stopped task 1.',
    after: () => screen.queryByRole('button', { name: 'Start' }) !== null,
  },
  {
    name: 'Mark done',
    shown: [tokenLeak, certificate],
    act: async (user) => {
      await user.click(screen.getByRole('button', { name: 'Mark done…' }))
      await user.click(screen.getByRole('button', { name: 'Mark done' }))
    },
    path: `${taskPath}/done`,
    answered: [certificate],
    said: 'Marked task 1 done.',
    after: () =>
      screen.queryByRole('heading', { level: 2, name: 'Renew the certificate' }) !== null,
  },
  {
    name: 'Annotate',
    shown: [tokenLeak],
    act: async (user) => {
      await user.type(screen.getByRole('textbox', { name: 'task 1 annotate' }), 'Ana reviews it')
      await user.click(screen.getByRole('button', { name: 'Annotate' }))
    },
    path: `${taskPath}/annotations`,
    body: { text: 'Ana reviews it' },
    answered: [
      { ...tokenLeak, annotations: [{ entry: tokenLeak.entry, description: 'Ana reviews it' }] },
    ],
    said: 'Annotated task 1.',
    after: () => screen.queryByRole('list', { name: 'Annotations' }) !== null,
  },
  {
    name: 'Modify',
    shown: [tokenLeak],
    act: async (user) => {
      await user.type(screen.getByRole('textbox', { name: 'task 1 modify' }), 'priority:H')
      await user.click(screen.getByRole('button', { name: 'Modify' }))
    },
    path: `${taskPath}/modify`,
    body: { line: 'priority:H' },
    answered: [
      describedTask(
        { ...tokenLeak, priority: 'H' },
        {
          state: 'pending',
          facets: [
            taskFacet.pending,
            { kind: 'priority', value: 'H', label: 'priority H' },
            taskFacet.noProject,
            taskFacet.withIssue,
            taskFacet.noTag,
          ],
          searchable: tokenLeak.searchable,
        },
      ),
    ],
    said: 'Modified task 1.',
    after: () => factOf('Priority') === 'H',
  },
]

test.each(verbs)(
  '$name posts to its path, lists what Taskwarrior answers and says what it did',
  async ({ shown, act, path, body, answered, said, after }) => {
    // Arrange
    const user = userEvent.setup()
    const requests = serveTasks(makeTaskList(shown), { [path]: makeTaskList(answered) })
    renderWithClient(<TasksPanel />)
    await screen.findByRole('article')

    // Act
    await act(user)

    // Assert
    expect(await screen.findByText(said)).toBeTruthy()
    expect(statusSaying(said)).toBeDefined()
    expect(after()).toBe(true)
    expect(await postsOf(requests)).toEqual([[path, body]])
  },
)

test('a write keeps its answer over a read that was in flight when it landed', async () => {
  // Arrange
  // Refresh's read is answered only after Start has answered, with the list
  // from before the start: the page must keep what the write answered.
  const user = userEvent.setup()
  let answerRefresh = () => {}
  const reads = [
    Promise.resolve(Response.json(makeTaskList([tokenLeak]))),
    new Promise<Response>((resolve) => {
      answerRefresh = () => {
        resolve(Response.json(makeTaskList([tokenLeak])))
      }
    }),
  ]
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) =>
      request.method === 'GET'
        ? reads.shift()
        : Promise.resolve(Response.json(makeTaskList([started]))),
    ),
  )
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: 'Refresh' }))
  await user.click(screen.getByRole('button', { name: 'Start' }))
  await screen.findByRole('button', { name: 'Stop' })

  // Act
  await act(async () => {
    answerRefresh()
    await new Promise((resolve) => setTimeout(resolve, 50))
  })

  // Assert
  expect(screen.getByRole('button', { name: 'Stop' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Start' })).toBeNull()
})

test('a write whose list could not be read again reads it once more', async () => {
  // Arrange
  // The start landed, but the server could not read the list after it, so it
  // answers the list unavailable with why; the page reads the list itself.
  const user = userEvent.setup()
  const notReadAgain =
    'The change was made, but your tasks could not be read again: Taskwarrior did not answer in time'
  const lists = [makeTaskList([tokenLeak]), makeTaskList([started])]
  fakeApi({
    [tasksPath]: () => (lists.length > 1 ? lists.shift() : lists[0]),
    [`${taskPath}/start`]: makeTaskList([], {
      available: false,
      reason: notReadAgain,
      reason_code: 'unavailable',
    }),
  })
  renderWithClient(<TasksPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Start' }))

  // Assert
  expect(await screen.findByRole('button', { name: 'Stop' })).toBeTruthy()
  expect(screen.queryByText(notReadAgain)).toBeNull()
})

test('a write that sorts the list anew keeps the detail on the task it showed', async () => {
  // Arrange
  // Started, the token leak is the most urgent and listed first, so its detail
  // is shown; stopped, it loses the urgency Taskwarrior gives a started task
  // and falls below the certificate, which the server then ranks first in
  // every order but by ID and by issue.
  const user = userEvent.setup()
  const startedLeak = ranked({ ...started, urgency: 8.9 }, firstInEveryOrder)
  const stoppedLeak = ranked(
    { ...tokenLeak, urgency: 4.9 },
    { urgency: 1, state: 1, id: 0, tag: 1, issue: 0, priority: 1 },
  )
  serveTasks(makeTaskList([startedLeak, ranked(certificate, secondInEveryOrder)]), {
    [`${taskPath}/stop`]: makeTaskList([
      ranked(certificate, { urgency: 0, state: 0, id: 1, tag: 0, issue: 1, priority: 0 }),
      stoppedLeak,
    ]),
  })
  renderWithClient(<TasksPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Stop' }))

  // Assert
  await screen.findByText('Stopped task 1.')
  expect(within(screen.getByRole('article')).getByRole('heading', { level: 2 }).textContent).toBe(
    tokenLeak.description,
  )
  expect(screen.getByRole('button', { name: 'Start' })).toBeTruthy()
})

test("Done takes the done task's controls with it, so focus goes to what it said", async () => {
  // Arrange
  // A press of Done must never land on the next task's Done under the pointer.
  const user = userEvent.setup()
  serveTasks(makeTaskList(leakThenCertificate), {
    [`${taskPath}/done`]: makeTaskList([certificate]),
  })
  renderWithClient(<TasksPanel />)

  await user.click(await screen.findByRole('button', { name: 'Mark done…' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Mark done' }))

  // Assert
  await screen.findByRole('heading', { level: 2, name: 'Renew the certificate' })
  expect(document.activeElement).toBe(statusSaying('Marked task 1 done.'))
})

test('a change Taskwarrior makes nothing of says why beside its button, and keeps the list', async () => {
  // Arrange
  const user = userEvent.setup()
  const detail = 'the task is already in that state, or is no longer pending'
  serveTasks(makeTaskList(leakThenCertificate), {
    [`${taskPath}/start`]: () => refused(detail, 409),
  })
  renderWithClient(<TasksPanel />)
  const start = await screen.findByRole('button', { name: 'Start' })

  // Act
  await user.click(start)

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe(detail)
  expect(alert.parentElement?.contains(start)).toBe(true)
  expect(document.activeElement).toBe(start)
  expect(within(screen.getByRole('list', { name: 'Tasks' })).getAllByRole('listitem')).toHaveLength(
    2,
  )
})

test.each([
  ['a backend is set', true, 1],
  ['none is set', false, 0],
])('Sync is offered only when %s', async (_, syncAvailable, offered) => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([tokenLeak], { sync_available: syncAvailable }) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  await screen.findByRole('article')
  expect(screen.queryAllByRole('button', { name: 'Sync…' })).toHaveLength(offered)
})

test('Undo is offered even when no task is pending', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(await screen.findByRole('button', { name: 'Undo…' })).toBeTruthy()
})

test.each([
  ['Undo', 'undo', 'reverted 1 operation', 'Undone: reverted 1 operation'],
  ['Undo', 'undo', '', 'Undone.'],
  ['Sync', 'sync', 'Sync successful.', 'Synced: Sync successful.'],
  ['Sync', 'sync', '', 'Synced.'],
])('%s, posted to /api/tasks/%s, says %j as %j', async (button, path, said, words) => {
  // Arrange
  const user = userEvent.setup()
  const requests = serveTasks(makeTaskList([tokenLeak], { sync_available: true }), {
    [`${tasksPath}/${path}`]: makeTaskList([certificate], { sync_available: true, said }),
  })
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: `${button}…` }))

  // Act
  await user.click(screen.getByRole('button', { name: button }))

  // Assert
  expect(await screen.findByText(words)).toBeTruthy()
  expect(screen.getByRole('heading', { level: 2, name: 'Renew the certificate' })).toBeTruthy()
  expect(await postsOf(requests)).toEqual([[`${tasksPath}/${path}`, undefined]])
})

test('Undo with nothing to undo says so beside it', async () => {
  // Arrange
  const user = userEvent.setup()
  serveTasks(makeTaskList([tokenLeak]), {
    [`${tasksPath}/undo`]: () => refused('Taskwarrior has nothing to undo', 409),
  })
  renderWithClient(<TasksPanel />)
  await user.click(await screen.findByRole('button', { name: 'Undo…' }))
  const undo = screen.getByRole('button', { name: 'Undo' })

  // Act
  await user.click(undo)

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toBe('Taskwarrior has nothing to undo')
  expect(alert.parentElement?.contains(undo)).toBe(true)
})

test('a task added outside the list is named by the start of its uuid', async () => {
  // Arrange
  // The active context leaves the new task out of the list.
  const user = userEvent.setup()
  fakeApi({
    [tasksPath]: (_: URL, asked: Request) =>
      makeTaskList(
        [certificate],
        asked.method === 'POST' ? { added: '66666666-6666-4666-8666-666666666666' } : {},
      ),
  })
  renderWithClient(<TasksPanel />)
  await user.type(await screen.findByRole('textbox', { name: 'task add' }), 'Pay the invoice')

  // Act
  await user.click(screen.getByRole('button', { name: 'Add' }))

  // Assert
  expect(await screen.findByText('Added task 66666666.')).toBeTruthy()
})

test('the add line sends what was typed without the spaces around it', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = fakeApi({ [tasksPath]: makeTaskList([certificate]) })
  renderWithClient(<TasksPanel />)
  await user.type(await screen.findByRole('textbox', { name: 'task add' }), '  Write the docs  ')

  // Act
  await user.click(screen.getByRole('button', { name: 'Add' }))

  // Assert
  await screen.findByText('Added the task.')
  expect(await postsOf(requests)).toEqual([[tasksPath, { line: 'Write the docs' }]])
})

test('an empty add line is refused without a request', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = fakeApi({ [tasksPath]: makeTaskList([certificate]) })
  renderWithClient(<TasksPanel />)
  await user.type(await screen.findByRole('textbox', { name: 'task add' }), '   ')

  // Act
  await user.click(screen.getByRole('button', { name: 'Add' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('Type a line for task add first.')
  expect(requests.filter((request) => request.method === 'POST')).toEqual([])
})

test('an empty note is refused without a request', async () => {
  // Arrange
  const user = userEvent.setup()
  const requests = fakeApi({ [tasksPath]: makeTaskList([tokenLeak]) })
  renderWithClient(<TasksPanel />)
  await screen.findByRole('textbox', { name: 'task 1 annotate' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Annotate' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'Type a line for task 1 annotate first.',
  )
  expect(requests.filter((request) => request.method === 'POST')).toEqual([])
})

test('a refused line says why and keeps what was typed', async () => {
  // Arrange
  const user = userEvent.setup()
  const detail = "Taskwarrior refused the command: The 'due' attribute does not allow 'soon'."
  serveTasks(makeTaskList([tokenLeak]), {
    [`${taskPath}/modify`]: () =>
      Response.json(
        { title: 'Unprocessable content', status: 422, detail, code: 'unprocessable' },
        { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
      ),
  })
  renderWithClient(<TasksPanel />)
  const line = await screen.findByRole('textbox', { name: 'task 1 modify' })
  await user.type(line, 'due:soon')

  // Act
  await user.click(screen.getByRole('button', { name: 'Modify' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(detail)
  expect((line as HTMLInputElement).value).toBe('due:soon')
  expect(document.activeElement).toBe(screen.getByRole('button', { name: 'Modify' }))
})

test('a write the dry run holds back says so, and sends nothing', async () => {
  // Arrange
  useHealthStore.setState({ health: makeHealth({ dry_run: true }) })
  const user = userEvent.setup()
  const requests = fakeApi({ [tasksPath]: makeTaskList([tokenLeak]) })
  renderWithClient(<TasksPanel />)

  // Act
  await user.click(await screen.findByRole('button', { name: 'Start' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe(
    'Held back by --dry-run: nothing was sent.',
  )
  expect(requests.filter((request) => request.method === 'POST')).toEqual([])
})
