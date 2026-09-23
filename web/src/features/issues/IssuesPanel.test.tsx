import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { checkoutBranch } from './checkoutApi.ts'
import { IssuesPanel } from './IssuesPanel.tsx'

vi.mock('./checkoutApi.ts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./checkoutApi.ts')>()),
  checkoutBranch: vi.fn(() => Promise.resolve()),
}))
const mockCheckout = vi.mocked(checkoutBranch)

const tokenLeak: Issue = {
  key: 'PROJ-1',
  summary: 'Fix the token leak',
  status: 'In Progress',
  status_category: 'indeterminate',
  type: 'Bug',
  priority: 'High',
}

const setupDocs: Issue = {
  key: 'PROJ-2',
  summary: 'Write the setup docs',
  status: 'To Do',
  status_category: 'new',
  type: 'Task',
}

// streamIssues puts a stream frame listing the issues on screen.
function streamIssues(issues: Issue[]) {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: issues.length, start_at: 0, issues } }),
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

test('reads an issue again when it is reopened after a minute', async () => {
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
  vi.setSystemTime(new Date('2026-09-23T12:01:01Z'))

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  await waitFor(() => {
    expect(readsOf(paths)).toBe(2)
  })
})

test('does not read an issue again when it is reopened within a minute', async () => {
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
  await user.click(screen.getByRole('button', { name: /check out PROJ-1/i }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('fix/PROJ-1-leak')
})

test('does not offer to check out the branch already on HEAD', () => {
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
  expect(screen.queryByRole('button', { name: /check out PROJ-1/i })).toBeNull()
})

test('shows the reason when a list checkout is refused', async () => {
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
  await user.click(screen.getByRole('button', { name: /check out PROJ-1/i }))

  // Assert
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toMatch(/uncommitted changes/i)
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
  expect(screen.queryByRole('button', { name: /check out PROJ-1/i })).toBeNull()
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
  await user.click(screen.getByRole('button', { name: /check out PROJ-1/i }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('feat/PROJ-1-redo')
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})

test('says when no issues match the view', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({ issues: { total: 0, start_at: 0, issues: [] } }),
  })

  // Act
  renderWithClient(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/no issues match/i)).toBeTruthy()
})
