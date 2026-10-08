import { QueryClient } from '@tanstack/react-query'
import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import App from '@/App.tsx'
import { fakeApi } from '@/test/fakeApi.ts'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { makeSnapshot, makeTask, makeTaskList } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { appQueryClient, renderWithClient } from '@/test/renderWithClient.tsx'
import { TasksPanel } from './TasksPanel.tsx'

const tasksPath = '/api/tasks'
const hour = 3_600_000

// hoursFromNow is the RFC 3339 time some hours after now, or before it when
// negative.
function hoursFromNow(hours: number): string {
  return new Date(Date.now() + hours * hour).toISOString()
}

const tokenLeakIssue: Issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
}

const tokenLeak = makeTask({
  uuid: '11111111-1111-4111-8111-111111111111',
  id: 1,
  description: 'PROJ-1: Fix the token leak',
  issue_key: 'PROJ-1',
  issue_url: 'https://jira.example.com/browse/PROJ-1',
  urgency: 9.5,
})
const certificate = makeTask({
  uuid: '22222222-2222-4222-8222-222222222222',
  id: 2,
  description: 'Renew the certificate',
  issue_key: '',
  issue_url: '',
  urgency: 5.1,
  due: hoursFromNow(27.5),
})
const elsewhere = makeTask({
  uuid: '33333333-3333-4333-8333-333333333333',
  id: 3,
  description: 'PROJ-9: Tune the cache',
  issue_key: 'PROJ-9',
  issue_url: 'https://jira.example.com/browse/PROJ-9',
  urgency: 3.2,
})
const waiting = makeTask({
  uuid: '44444444-4444-4444-8444-444444444444',
  id: 4,
  description: 'Book the retro room',
  status: 'waiting',
  wait: hoursFromNow(48),
  issue_key: '',
  issue_url: '',
  urgency: 1.1,
})

// streamIssues puts a stream frame on screen whose Issues list holds the issues.
function streamIssues(...issues: Issue[]) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: { total: issues.length, start_at: 0, unavailable: [], issues },
    }),
  })
}

// refused is a problem answer, as the server gives a failed read or write.
function refused(body: object, status: number): Response {
  return Response.json(body, { status, headers: { 'Content-Type': 'application/problem+json' } })
}

// rowsOf is the text of each row a list holds.
function rowsOf(list: HTMLElement): string[] {
  return within(list)
    .getAllByRole('listitem')
    .map((row) => row.textContent)
}

// readsOf counts the reads of the task list among the requests the page made.
function readsOf(requests: Request[]): number {
  return requests.filter(
    (request) => request.method === 'GET' && new URL(request.url).pathname === tasksPath,
  ).length
}

// statusSaying is the live status line that says text, if one does.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((line) => line.textContent === text)
}

test('lists the tasks for your issues apart from the others, and counts those waiting', async () => {
  // Arrange
  streamIssues(tokenLeakIssue)
  fakeApi({ [tasksPath]: makeTaskList([tokenLeak, certificate, elsewhere, waiting]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const mine = await screen.findByRole('list', { name: 'For my issues' })
  const others = screen.getByRole('list', { name: 'Other tasks' })
  expect(rowsOf(mine)).toEqual([expect.stringMatching(/#1.*PROJ-1: Fix the token leak/)])
  expect(rowsOf(others)).toEqual([
    expect.stringMatching(/#2.*Renew the certificate.*due in 1d 3h.*urgency 5\.1/),
    expect.stringMatching(/#3.*PROJ-9: Tune the cache.*PROJ-9.*urgency 3\.2/),
  ])
  expect(screen.getByRole('heading', { level: 2, name: 'For my issues' })).toBeTruthy()
  expect(screen.getByRole('heading', { level: 2, name: 'Other tasks' })).toBeTruthy()
  expect(screen.getByText('1 waiting')).toBeTruthy()
  expect(screen.queryByText(/book the retro room/i)).toBeNull()
})

test('a row gives each fact after the task an element of its own', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([elsewhere]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const row = within((await screen.findAllByRole('listitem'))[0] ?? document.body)
  expect(row.getByText('PROJ-9').textContent).toBe('PROJ-9')
  expect(row.getByText('urgency 3.2').textContent).toBe('urgency 3.2')
})

test('tasks all of one kind are one list, with no headings over it', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([certificate, elsewhere]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Tasks' })
  expect(rowsOf(list)).toHaveLength(2)
  expect(screen.queryByRole('heading', { name: 'For my issues' })).toBeNull()
  expect(screen.queryByRole('heading', { name: 'Other tasks' })).toBeNull()
})

test('the mark beside each task is how far it has got, by shape, beside its words', async () => {
  // Arrange
  const started = { ...tokenLeak, start: hoursFromNow(-1) }
  fakeApi({ [tasksPath]: makeTaskList([started, certificate]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const rows = within(await screen.findByRole('list', { name: 'Tasks' })).getAllByRole('button')
  expect(rows.map((row) => markShape(row))).toEqual([
    drawnMark('in-flight'),
    drawnMark('not-started'),
  ])
  expect(rows.map((row) => row.textContent)).toEqual([
    expect.stringContaining('started'),
    expect.stringContaining('pending'),
  ])
})

test('a task outside the working set shows no number', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([{ ...certificate, id: 0 }]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Tasks' })
  expect(rowsOf(list)).toEqual([expect.not.stringContaining('#')])
})

test('the first task is selected and its detail shown; choosing another shows its', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [tasksPath]: makeTaskList([certificate, elsewhere]) })
  renderWithClient(<TasksPanel />)
  const list = await screen.findByRole('list', { name: 'Tasks' })
  const first = within(list).getByRole('button', { name: /renew the certificate/i })
  const second = within(list).getByRole('button', { name: /tune the cache/i })
  expect(first.getAttribute('aria-current')).toBe('true')

  // Act
  await user.click(second)

  // Assert
  expect(first.getAttribute('aria-current')).toBeNull()
  expect(second.getAttribute('aria-current')).toBe('true')
  expect(screen.getByRole('heading', { level: 2, name: 'PROJ-9: Tune the cache' })).toBeTruthy()
})

test('the detail gives the task its facts and its notes', async () => {
  // Arrange
  const noted = {
    ...tokenLeak,
    project: 'api',
    priority: 'H',
    tags: ['jira', 'security'],
    annotations: [{ entry: '2026-09-21T09:00:00Z', description: 'Ana can review it' }],
  }
  fakeApi({ [tasksPath]: makeTaskList([noted]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const detail = await screen.findByRole('article')
  expect(within(detail).getByRole('heading', { level: 2 }).textContent).toBe(noted.description)
  const terms = within(detail)
    .getAllByRole('term')
    .map((term) => term.textContent)
  const values = within(detail)
    .getAllByRole('definition')
    .map((value) => value.textContent)
  expect(Object.fromEntries(terms.map((term, index) => [term, values[index]]))).toMatchObject({
    State: 'pending',
    Project: 'api',
    Priority: 'H',
    Tags: '+jira +security',
    Urgency: '9.5',
    ID: '#1',
    Issue: 'PROJ-1',
  })
  expect(within(detail).getByRole('list', { name: 'Annotations' }).textContent).toContain(
    'Ana can review it',
  )
})

test('the detail of a started task says how long ago it was started, and when it is due', async () => {
  // Arrange
  const started = { ...tokenLeak, start: hoursFromNow(-1.2), due: hoursFromNow(-2) }
  fakeApi({ [tasksPath]: makeTaskList([started]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const detail = await screen.findByRole('article')
  expect(detail.textContent).toContain('started 1h12m ago')
  expect(detail.textContent).toContain('overdue')
})

test('Open in Issues is offered only for a task whose issue the Issues list holds', async () => {
  // Arrange
  streamIssues(tokenLeakIssue)
  fakeApi({ [tasksPath]: makeTaskList([elsewhere]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  await screen.findByRole('heading', { level: 2, name: 'PROJ-9: Tune the cache' })
  expect(screen.queryByRole('button', { name: 'Open in Issues' })).toBeNull()
})

test("Open in Issues opens the Issues section on the task's issue", async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [tasksPath]: makeTaskList([tokenLeak]) })
  renderWithClient(<App />)
  act(() => {
    FakeEventSource.latest().emit(
      'snapshot',
      JSON.stringify(
        makeSnapshot({
          issues: { total: 1, start_at: 0, unavailable: [], issues: [tokenLeakIssue] },
        }),
      ),
    )
  })
  await user.click(screen.getByRole('button', { name: 'Tasks' }))

  // Act
  await user.click(await screen.findByRole('button', { name: 'Open in Issues' }))

  // Assert
  expect(screen.getByRole('heading', { level: 1, name: 'Issues' })).toBeTruthy()
  expect(screen.getByRole('heading', { level: 2, name: 'Fix the token leak' })).toBeTruthy()
})

test('no pending tasks says how to get one, and still counts those waiting', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([waiting]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(
    await screen.findByText('No pending tasks. Add one above, or track an issue from Issues.'),
  ).toBeTruthy()
  expect(screen.getByText('1 waiting')).toBeTruthy()
})

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
  renderWithClient(<TasksPanel />)
  expect((await screen.findByRole('alert')).textContent).toBe('Taskwarrior did not answer in time')

  // Act
  await user.click(screen.getByRole('button', { name: 'Try again' }))

  // Assert
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
  const answers = [makeTaskList([certificate]), makeTaskList([certificate, elsewhere])]
  const requests = fakeApi({ [tasksPath]: () => answers.shift() })
  renderWithClient(<TasksPanel />)
  await screen.findByRole('list', { name: 'Tasks' })

  // Act
  await user.click(screen.getByRole('button', { name: 'Refresh' }))

  // Assert
  expect(await screen.findByText('PROJ-9: Tune the cache')).toBeTruthy()
  expect(readsOf(requests)).toBe(2)
})

test('says which context narrows the list', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList([certificate], { context: 'work' }) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(await screen.findByText(/context work narrows the list/)).toBeTruthy()
})

test('the add line posts the typed line and lists what Taskwarrior answers', async () => {
  // Arrange
  const user = userEvent.setup()
  const added = makeTask({
    uuid: '55555555-5555-4555-8555-555555555555',
    id: 5,
    description: 'Write the setup docs',
    project: 'docs',
    issue_key: '',
    issue_url: '',
  })
  const requests = fakeApi({
    [tasksPath]: (_: URL, asked: Request) =>
      asked.method === 'POST'
        ? makeTaskList([certificate, added], { added: added.uuid })
        : makeTaskList([certificate]),
  })
  renderWithClient(<TasksPanel />)
  await user.type(
    await screen.findByRole('textbox', { name: 'task add' }),
    'Write the setup docs project:docs',
  )

  // Act
  await user.click(screen.getByRole('button', { name: 'Add' }))

  // Assert
  expect(await screen.findByText('Write the setup docs')).toBeTruthy()
  expect(statusSaying('Added task 5: Write the setup docs')).toBeDefined()
  const post = requests.find((request) => request.method === 'POST')
  expect(post && new URL(post.url).pathname).toBe(tasksPath)
  expect(await post?.json()).toEqual({ line: 'Write the setup docs project:docs' })
  expect(screen.getByRole<HTMLInputElement>('textbox', { name: 'task add' }).value).toBe('')
})

test('the mockup lists its own tasks without a server', async () => {
  // Arrange
  vi.stubEnv('VITE_MOCK', 'true')

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const others = await screen.findByRole('list', { name: /tasks/i })
  expect(within(others).getAllByRole('listitem').length).toBeGreaterThan(0)
  expect(globalThis.fetch).not.toHaveBeenCalled()
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
