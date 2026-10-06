import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Task, TasksSummary } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { fakeApi } from '@/test/fakeApi.ts'
import { makeSnapshot, makeTask, makeTaskList } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { IssueTasks } from './IssueTasks.tsx'
import { TasksPanel } from './TasksPanel.tsx'

const issueKey = 'PROJ-412'
const hour = 3_600_000

const tracking = makeTask({
  description: 'PROJ-412: Redact tokens before they reach the request log',
  issue_key: issueKey,
  issue_url: 'https://jira.example.com/browse/PROJ-412',
})
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

// statusSaying is the live status line that says text, if one does.
function statusSaying(text: string): HTMLElement | undefined {
  return screen.getAllByRole('status').find((line) => line.textContent === text)
}

test("lists the issue's task with its mark, and offers to start it and mark it done", () => {
  // Arrange
  streamLinked(tracking, {
    ...tracking,
    uuid: '99999999-9999-4999-8999-999999999999',
    issue_key: 'PROJ-1',
  })

  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  const rows = within(card).getAllByRole('listitem')
  expect(rows).toHaveLength(1)
  expect(rows[0]?.textContent).toMatch(/#12.*Redact tokens.*pending/)
  expect(markShape(rows[0] ?? card)).toBe(drawnMark('not-started'))
  expect(within(card).getByRole('button', { name: 'Start task 12' })).toBeTruthy()
  expect(within(card).getByRole('button', { name: 'Mark done… task 12' })).toBeTruthy()
  expect(within(card).queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
})

test('a started task says how long ago it was started, and offers to stop it', () => {
  // Arrange
  streamLinked({ ...tracking, start: new Date(Date.now() - 1.2 * hour).toISOString() })

  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  expect(card.textContent).toContain('started 1h12m ago')
  expect(markShape(within(card).getByRole('listitem'))).toBe(drawnMark('in-flight'))
  expect(within(card).getByRole('button', { name: 'Stop task 12' })).toBeTruthy()
})

test('a task still to do says when it is due', () => {
  // Arrange
  streamLinked({ ...tracking, due: new Date(Date.now() + 27.5 * hour).toISOString() })

  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  expect(screen.getByRole('region', { name: 'Tasks' }).textContent).toContain('due in 1d 3h')
})

test('an issue no task tracks offers to track it, which says which task now does', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  const requests = fakeApi({
    '/api/tasks/track': makeTaskList([tracking], { added: tracking.uuid }),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))

  // Assert
  expect(await screen.findByText('Task 12 tracks PROJ-412.')).toBeTruthy()
  expect(statusSaying('Task 12 tracks PROJ-412.')).toBeDefined()
  const [post] = requests
  expect(post?.method).toBe('POST')
  expect(post && new URL(post.url).pathname).toBe('/api/tasks/track')
  expect(await post?.json()).toEqual({ issue_key: issueKey })
  expect(screen.queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
})

test('a task tracked outside the list is named by the start of its uuid', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([], { added: tracking.uuid }) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))

  // Assert
  expect(await screen.findByText('Task 5f1d7a3c tracks PROJ-412.')).toBeTruthy()
  expect(screen.queryByText('No task tracks PROJ-412.')).toBeNull()
})

test.each([
  [
    'created but not annotated: Taskwarrior did not answer in time',
    'Task 12 tracks PROJ-412; created but not annotated: Taskwarrior did not answer in time.',
  ],
  [
    'created but not annotated: Taskwarrior refused the command: The annotation is empty.',
    'Task 12 tracks PROJ-412; created but not annotated: Taskwarrior refused the command: The annotation is empty.',
  ],
])('a task tracked but not annotated says so: %j', async (said, words) => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([tracking], { added: tracking.uuid, said }) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))

  // Assert
  expect(await screen.findByText(words)).toBeTruthy()
})

test('a track the tracker refuses says why, and can be tried again', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({
    '/api/tasks/track': () =>
      Response.json(
        {
          title: 'Not found',
          status: 404,
          detail: 'issue PROJ-412 was not found',
          code: 'not_found',
        },
        { status: 404, headers: { 'Content-Type': 'application/problem+json' } },
      ),
  })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))

  // Assert
  expect((await screen.findByRole('alert')).textContent).toBe('issue PROJ-412 was not found')
  expect(screen.getByRole('button', { name: 'Track in Taskwarrior' })).toBeTruthy()
  expect(screen.getByText('No task tracks PROJ-412.')).toBeTruthy()
})

test("the track's answer takes the place of the task list the page holds", async () => {
  // Arrange
  const user = userEvent.setup()
  const certificate = makeTask({
    uuid: '22222222-2222-4222-8222-222222222222',
    id: 2,
    description: 'Renew the certificate',
    issue_key: '',
    issue_url: '',
  })
  streamLinked()
  const requests = fakeApi({
    '/api/tasks': makeTaskList([certificate]),
    '/api/tasks/track': makeTaskList([certificate, tracking], { added: tracking.uuid }),
  })
  renderWithClient(
    <>
      <IssueTasks issueKey={issueKey} />
      <TasksPanel />
    </>,
  )
  await screen.findByRole('button', { name: /renew the certificate/i })

  // Act
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))

  // Assert
  expect(await screen.findByRole('button', { name: /redact tokens before/i })).toBeTruthy()
  const reads = requests.filter((request) => request.method === 'GET')
  expect(reads).toHaveLength(1)
})

test.each([
  [['Start task 12'], 'start', tracking, 'Started task 12.'],
  [['Stop task 12'], 'stop', { ...tracking, start: new Date().toISOString() }, 'Stopped task 12.'],
  [['Mark done… task 12', 'Mark done'], 'done', tracking, 'Marked task 12 done.'],
])('%j posts to its path and says what it did', async (buttons, verb, task, said) => {
  // Arrange
  const user = userEvent.setup()
  streamLinked(task)
  const requests = fakeApi({ [`${trackingPath}/${verb}`]: makeTaskList([]) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  for (const button of buttons) {
    await user.click(screen.getByRole('button', { name: button }))
  }

  // Assert
  expect(await screen.findByText(said)).toBeTruthy()
  expect(requests.map((request) => new URL(request.url).pathname)).toEqual([
    `${trackingPath}/${verb}`,
  ])
})

test('a track whose answer names no task still takes Track away', async () => {
  // Arrange
  const user = userEvent.setup()
  streamLinked()
  fakeApi({ '/api/tasks/track': makeTaskList([]) })
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Track in Taskwarrior' }))

  // Assert
  expect(await screen.findByText('The task tracks PROJ-412.')).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
  expect(screen.queryByText('No task tracks PROJ-412.')).toBeNull()
})

test('a completed task is listed done, with nothing to do, and the issue can be tracked again', () => {
  // Arrange
  streamLinked({ ...tracking, status: 'completed', end: new Date().toISOString() })

  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  const card = screen.getByRole('region', { name: 'Tasks' })
  const row = within(card).getByRole('listitem')
  expect(markShape(row)).toBe(drawnMark('done'))
  expect(row.textContent).not.toContain('#12')
  expect(within(row).queryAllByRole('button')).toEqual([])
  expect(within(card).getByRole('button', { name: 'Track in Taskwarrior' })).toBeTruthy()
})

test('a waiting task still tracks the issue', () => {
  // Arrange
  streamLinked({ ...tracking, status: 'waiting', wait: new Date(Date.now() + hour).toISOString() })

  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  expect(screen.getByRole('button', { name: 'Start task 12' })).toBeTruthy()
  expect(screen.queryByRole('button', { name: 'Track in Taskwarrior' })).toBeNull()
})

test('nothing is drawn when the snapshot says Taskwarrior is unavailable', () => {
  // Arrange
  streamTasks({
    available: false,
    reason: 'Turned off by taskwarrior.disabled.',
    linked: [tracking],
  })

  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  expect(screen.queryByRole('region', { name: 'Tasks' })).toBeNull()
  expect(screen.queryByRole('button')).toBeNull()
})

test('nothing is drawn before the first snapshot lands', () => {
  // Act
  renderWithClient(<IssueTasks issueKey={issueKey} />)

  // Assert
  expect(screen.queryByRole('region', { name: 'Tasks' })).toBeNull()
})
