import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot, makeTask } from '@/test/fixtures.ts'
import { drawnMark, markShape } from '@/test/marks.tsx'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { checkoutBranch } from './checkoutApi.ts'
import { IssuesPanel } from './IssuesPanel.tsx'

vi.mock('./checkoutApi.ts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./checkoutApi.ts')>()),
  checkoutBranch: vi.fn(() => Promise.resolve(makeBranch())),
}))
const mockCheckout = vi.mocked(checkoutBranch)

const tokenLeak: Issue = {
  key: 'PROJ-1',
  tracker: 'jira',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
}

const setupDocs: Issue = {
  key: 'PROJ-2',
  tracker: 'jira',
  summary: 'Write the setup docs',
  status: 'To Do',
  status_category: 'new',
  type: 'Task',
}

// streamIssues puts a stream frame listing the issues on screen, with the
// trackers it says could not be read.
function streamIssues(issues: Issue[], unavailable: string[] = []) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: issues.length, start_at: 0, unavailable, issues } }),
  })
}

function withIssues() {
  streamIssues([tokenLeak, setupDocs])
}

// serveTokenLeak answers getIssue for PROJ-1 with its full detail, and any
// other issue with a 404. It returns the paths read, in order, so a test can
// count the reads.
function serveTokenLeak(): string[] {
  const paths: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn((request: Request) => {
      const path = new URL(request.url).pathname
      paths.push(path)
      if (path !== '/api/issues/PROJ-1') {
        return Promise.resolve(Response.json({ detail: 'no such issue' }, { status: 404 }))
      }

      return Promise.resolve(
        Response.json({
          ...tokenLeak,
          reporter: 'Ana Lopez',
          description: 'Tokens reach the request log.',
          comments: [{ author: 'Ana Lopez', body: "Repro'd.", created: '2026-09-20T10:00:00Z' }],
          comment_total: 1,
          url: '',
        }),
      )
    }),
  )

  return paths
}

// readsOf is how many of the paths read are PROJ-1's detail.
function readsOf(paths: string[]): number {
  return paths.filter((path) => path === '/api/issues/PROJ-1').length
}

test('lists the issues in the view', () => {
  // Arrange
  withIssues()

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText('Fix the token leak')).toBeTruthy()
  expect(screen.getByText('Write the setup docs')).toBeTruthy()
})

test('shows an issue detail when it is selected', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: /fix the token leak/i })).toBeTruthy()
})

test('reads the selected issue in full', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  serveTokenLeak()
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  expect(await screen.findByText('Tokens reach the request log.')).toBeTruthy()
  expect(screen.getByRole('list', { name: /comments/i }).textContent).toMatch(/repro'd/i)
})

test('keeps the detail heading current with the stream after the full read lands', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  serveTokenLeak()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))
  await screen.findByText('Tokens reach the request log.')

  // Act
  // PROJ-1 is moved to Done elsewhere, and the stream's next frame carries it.
  act(() => {
    streamIssues([{ ...tokenLeak, status: 'Done', status_category: 'done' }, setupDocs])
  })

  // Assert
  expect(within(screen.getByRole('article')).getByText('Done')).toBeTruthy()
})

test('keeps the selected issue open when the stream drops it', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Act
  // PROJ-1 leaves the view elsewhere; the next frame lists only PROJ-2.
  act(() => {
    streamIssues([setupDocs])
  })

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: 'PROJ-1' })).toBeTruthy()
})

test('reads an issue again when it is reopened after 30 seconds', async () => {
  // Arrange
  // Only the clock is faked: staleness is judged from Date.now().
  vi.useFakeTimers({ toFake: ['Date'] })
  vi.setSystemTime(new Date('2026-09-23T12:00:00Z'))
  const user = userEvent.setup()
  withIssues()
  const paths = serveTokenLeak()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))
  await screen.findByText('Tokens reach the request log.')
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))
  vi.setSystemTime(new Date('2026-09-23T12:00:31Z'))

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  await waitFor(() => {
    expect(readsOf(paths)).toBe(2)
  })
})

test('does not read an issue again when it is reopened within 30 seconds', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  const paths = serveTokenLeak()
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))
  await screen.findByText('Tokens reach the request log.')
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  // Let a read the reopening might start reach fetch before counting.
  await act(
    () =>
      new Promise((resolve) => {
        setTimeout(resolve, 0)
      }),
  )
  expect(readsOf(paths)).toBe(1)
})

test('shows the work story for the selected issue', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  renderWithClient(<IssuesPanel />)

  // Act
  // PROJ-2 has no priority, so its meta line omits the priority clause.
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))

  // Assert
  expect(screen.getByRole('heading', { name: /work story/i })).toBeTruthy()
  expect(screen.getByText('Announce')).toBeTruthy()
})

test("the issue's detail shows its tasks between its work story and its description", async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  serveTokenLeak()
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  await screen.findByText('Tokens reach the request log.')
  expect(
    screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent),
  ).toEqual(['Work story', 'Tasks', 'Description', 'Comments'])
})

test('marks only the issues a local branch names as in flight', () => {
  // Arrange
  // Two issues, but only PROJ-1 has a local branch, so only it is in flight.
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [{ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: true }],
    },
  }))

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getAllByText('in flight')).toHaveLength(1)
})

test("draws each issue's status category as its mark, beside the status", () => {
  // Arrange
  const shipped: Issue = { ...setupDocs, key: 'PROJ-3', status: 'Done', status_category: 'done' }
  streamIssues([tokenLeak, setupDocs, shipped])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(
    ['To Do', 'In Progress', 'Done'].map((status) => markShape(screen.getByText(status))),
  ).toEqual([drawnMark('not-started'), drawnMark('in-flight'), drawnMark('done')])
})

test('marks an issue in flight with the in-flight mark, beside the words', () => {
  // Arrange
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [{ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: true }],
    },
  }))

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const words = screen.getByText('in flight')
  expect(markShape(words.parentElement ?? words)).toBe(drawnMark('in-flight'))
})

// taskMarkRows lists four issues — one whose task is started, one whose task is
// still to do, one whose every task is completed, and one no task tracks —
// under a stream frame whose task summary is available as given.
function taskMarkRows(available: boolean) {
  const shipped: Issue = { ...setupDocs, key: 'PROJ-3', summary: 'Ship the release' }
  const untracked: Issue = { ...setupDocs, key: 'PROJ-4', summary: 'Plan the next one' }
  const linked = [
    makeTask({
      uuid: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1',
      id: 1,
      issue_key: 'PROJ-1',
      start: '2026-09-28T09:00:00Z',
    }),
    makeTask({ uuid: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2', id: 2, issue_key: 'PROJ-2' }),
    makeTask({
      uuid: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3',
      id: 0,
      issue_key: 'PROJ-3',
      status: 'completed',
    }),
  ]
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: {
        total: 4,
        start_at: 0,
        unavailable: [],
        issues: [tokenLeak, setupDocs, shipped, untracked],
      },
      tasks: { available, reason: available ? '' : 'Turned off by taskwarrior.disabled.', linked },
    }),
  })
}

test("marks each issue's task by shape, beside the words for it", () => {
  // Arrange
  taskMarkRows(true)

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const rows = within(screen.getByRole('list', { name: 'Issues' })).getAllByRole('listitem')
  const marked = ['task active', 'tracked', 'task done'].map((words) => {
    const said = screen.getByText(words)

    return [rows.findIndex((row) => row.contains(said)), markShape(said.parentElement ?? said)]
  })
  expect(marked).toEqual([
    [0, drawnMark('in-flight')],
    [1, drawnMark('not-started')],
    [2, drawnMark('done')],
  ])
  expect(rows[3]?.textContent).not.toMatch(/tracked|task active|task done/)
})

test.each([
  ['waits until a later date', { status: 'waiting', wait: '2099-01-01T00:00:00Z' }],
  ['recurs', { status: 'recurring' }],
] as const)('an issue whose only task %s is tracked, never done', (_, shape) => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: { total: 1, start_at: 0, unavailable: [], issues: [tokenLeak] },
      tasks: { available: true, reason: '', linked: [makeTask({ issue_key: 'PROJ-1', ...shape })] },
    }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  const said = screen.getByText('tracked')
  expect(markShape(said.parentElement ?? said)).toBe(drawnMark('not-started'))
  expect(screen.queryByText('task done')).toBeNull()
})

test('draws no task marks when Taskwarrior is unavailable', () => {
  // Arrange
  taskMarkRows(false)

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByRole('list', { name: 'Issues' }).textContent).not.toMatch(
    /tracked|task active|task done/,
  )
})

test('checks out an in-flight branch from the list without opening the detail', async () => {
  // Arrange
  // PROJ-1's branch exists but is not the checked-out one, so the row offers to
  // switch to it directly.
  const user = userEvent.setup()
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [{ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false }],
    },
  }))
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('fix/PROJ-1-leak')
})

test('does not offer to switch to the branch already on HEAD', () => {
  // Arrange
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [{ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: true }],
    },
  }))

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Switch branch for PROJ-1' })).toBeNull()
})

test('shows the reason when a list checkout is refused, with focus still on it', async () => {
  // Arrange
  mockCheckout.mockRejectedValueOnce({
    code: 'conflict',
    detail: 'the working tree has uncommitted changes',
  })
  const user = userEvent.setup()
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [{ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false }],
    },
  }))
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toMatch(/uncommitted changes/i)
  expect(document.activeElement).toBe(
    screen.getByRole('button', { name: 'Switch branch for PROJ-1' }),
  )
})

test('is not offered when a branch on HEAD exists among several for one issue', () => {
  // Arrange
  // PROJ-1 has two branches and the newest is on HEAD, so it is not offered for
  // checkout even though an older branch is not current.
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [
        { name: 'feat/PROJ-1-redo', issue_key: 'PROJ-1', current: true },
        { name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false },
      ],
    },
  }))

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Switch branch for PROJ-1' })).toBeNull()
})

test('checks out the most recent branch when several name one issue', async () => {
  // Arrange
  // Branches arrive most-recently-committed first; neither is on HEAD, so the row
  // offers the most recent one.
  const user = userEvent.setup()
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [
        { name: 'feat/PROJ-1-redo', issue_key: 'PROJ-1', current: false },
        { name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false },
      ],
    },
  }))
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('feat/PROJ-1-redo')
})

test('says when no issues match the view', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: 0, start_at: 0, unavailable: [], issues: [] } }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/no issues match/i)).toBeTruthy()
})

// Twin of TestAForgeIssueIsListedByItsNumber.
test('a forge issue is listed by its number, as the forge writes it', () => {
  // Arrange
  streamIssues([
    { ...setupDocs, key: '57', tracker: 'forge', summary: 'Typo in the README' },
    tokenLeak,
  ])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByRole('button', { name: /typo in the readme/i }).textContent).toContain('#57')
})

// Twin of TestTheIssuesListSaysWhenTheForgeCouldNotBeRead.
test("the list says which tracker's issues could not be read", () => {
  // Arrange
  streamIssues([tokenLeak], ["the forge's issues"])

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(
    screen
      .getAllByRole('status')
      .some((line) => line.textContent === "Not read: the forge's issues."),
  ).toBe(true)
})
