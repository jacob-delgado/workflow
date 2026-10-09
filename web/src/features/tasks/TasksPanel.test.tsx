import { act, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import App from '@/App.tsx'
import { fakeApi } from '@/test/fakeApi.ts'
import { FakeEventSource } from '@/test/fakeEventSource.ts'
import { describedTask, makeSnapshot, makeTaskList, taskFacet } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import {
  cacheTuning,
  certificate,
  firstInEveryOrder,
  ranked,
  retroRoom,
  secondInEveryOrder,
  startedTokenLeak,
  tokenLeak,
} from '@/test/tasks.ts'
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

// certificateDue is the certificate, due in a day and more.
const certificateDue = { ...certificate, due: hoursFromNow(27.5) }

// certificateAndCache are the certificate and the cache tuning as the server
// ranks the two: the certificate first by urgency, and so wherever the two
// tie, the cache tuning first by issue, since only it tracks one.
const certificateAndCache = [
  ranked(certificate, { urgency: 0, state: 0, id: 0, tag: 0, issue: 1, priority: 0 }),
  ranked(cacheTuning, { urgency: 1, state: 1, id: 1, tag: 1, issue: 0, priority: 1 }),
]

// streamIssues puts a stream frame on screen whose Issues list holds the issues.
function streamIssues(...issues: Issue[]) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: { total: issues.length, start_at: 0, unavailable: [], issues },
    }),
  })
}

// rowsOf is the text of each row a list holds.
function rowsOf(list: HTMLElement): string[] {
  return within(list)
    .getAllByRole('listitem')
    .map((row) => row.textContent)
}

// statusSaying is the live status line that says text, if one does.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((line) => line.textContent === text)
}

test('lists the tasks for your issues apart from the others, and counts those waiting', async () => {
  // Arrange
  // The server hands the tasks over as Taskwarrior lists them, each ranked
  // among the four; the page lists them by urgency, the waiting one apart.
  streamIssues(tokenLeakIssue)
  fakeApi({
    [tasksPath]: makeTaskList([
      ranked(retroRoom, { urgency: 3, state: 3, id: 3, tag: 3, issue: 3, priority: 3 }),
      ranked(cacheTuning, { urgency: 2, state: 2, id: 2, tag: 2, issue: 1, priority: 2 }),
      ranked(certificateDue, { urgency: 1, state: 1, id: 1, tag: 1, issue: 2, priority: 1 }),
      ranked(tokenLeak, firstInEveryOrder),
    ]),
  })

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
  fakeApi({ [tasksPath]: makeTaskList([cacheTuning]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const row = within((await screen.findAllByRole('listitem'))[0] ?? document.body)
  expect(row.getByText('PROJ-9').textContent).toBe('PROJ-9')
  expect(row.getByText('urgency 3.2').textContent).toBe('urgency 3.2')
})

test('tasks all of one kind are one list, with no headings over it', async () => {
  // Arrange
  fakeApi({ [tasksPath]: makeTaskList(certificateAndCache) })

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
  fakeApi({
    [tasksPath]: makeTaskList([
      ranked(startedTokenLeak(hoursFromNow(-1)), firstInEveryOrder),
      ranked(certificate, secondInEveryOrder),
    ]),
  })

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
  // Taskwarrior numbers only the tasks in its working set, so the server
  // matches no #id for this one.
  const unnumbered = describedTask(
    { ...certificate, id: 0 },
    { state: 'pending', facets: certificate.facets, searchable: ['renew the certificate', '', ''] },
  )
  fakeApi({ [tasksPath]: makeTaskList([unnumbered]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  const list = await screen.findByRole('list', { name: 'Tasks' })
  expect(rowsOf(list)).toEqual([expect.not.stringContaining('#')])
})

test('the first task is selected and its detail shown; choosing another shows its', async () => {
  // Arrange
  const user = userEvent.setup()
  fakeApi({ [tasksPath]: makeTaskList(certificateAndCache) })

  // Act: list the tasks
  renderWithClient(<TasksPanel />)

  // Assert: the first is selected
  const list = await screen.findByRole('list', { name: 'Tasks' })
  const first = within(list).getByRole('button', { name: /renew the certificate/i })
  const second = within(list).getByRole('button', { name: /tune the cache/i })
  expect(first.getAttribute('aria-current')).toBe('true')

  // Act: choose the second
  await user.click(second)

  // Assert: the second is selected instead, and its detail shown
  expect(first.getAttribute('aria-current')).toBeNull()
  expect(second.getAttribute('aria-current')).toBe('true')
  expect(screen.getByRole('heading', { level: 2, name: 'PROJ-9: Tune the cache' })).toBeTruthy()
})

test('the detail gives the task its facts and its notes', async () => {
  // Arrange
  const noted = describedTask(
    {
      ...tokenLeak,
      project: 'api',
      priority: 'H',
      tags: ['jira', 'security'],
      annotations: [{ entry: '2026-09-21T09:00:00Z', description: 'Ana can review it' }],
    },
    {
      state: 'pending',
      facets: [
        taskFacet.pending,
        { kind: 'priority', value: 'H', label: 'priority H' },
        { kind: 'project', value: 'api', label: 'project api' },
        taskFacet.withIssue,
        { kind: 'tag', value: 'jira', label: '+jira' },
        { kind: 'tag', value: 'security', label: '+security' },
      ],
      searchable: ['proj-1: fix the token leak', 'api', 'proj-1', '+jira', '+security', '#1'],
    },
  )
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
  const started = { ...startedTokenLeak(hoursFromNow(-1.2)), due: hoursFromNow(-2) }
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
  fakeApi({ [tasksPath]: makeTaskList([cacheTuning]) })

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
  fakeApi({ [tasksPath]: makeTaskList([retroRoom]) })

  // Act
  renderWithClient(<TasksPanel />)

  // Assert
  expect(
    await screen.findByText('No pending tasks. Add one above, or track an issue from Issues.'),
  ).toBeTruthy()
  expect(screen.getByText('1 waiting')).toBeTruthy()
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
  const added = describedTask(
    {
      uuid: '55555555-5555-4555-8555-555555555555',
      id: 5,
      description: 'Write the setup docs',
      status: 'pending',
      project: 'docs',
      priority: '',
      tags: [],
      issue_key: '',
      issue_url: '',
    },
    {
      state: 'pending',
      facets: [
        taskFacet.pending,
        taskFacet.noPriority,
        { kind: 'project', value: 'docs', label: 'project docs' },
        taskFacet.noIssue,
        taskFacet.noTag,
      ],
      searchable: ['write the setup docs', 'docs', '', '#5'],
    },
  )
  const requests = fakeApi({
    [tasksPath]: (_: URL, asked: Request) =>
      asked.method === 'POST'
        ? makeTaskList(
            [ranked(certificate, firstInEveryOrder), ranked(added, secondInEveryOrder)],
            { added: added.uuid },
          )
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
