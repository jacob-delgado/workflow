import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import type { Issue } from '@/api/generated/types.gen.ts'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeBranch, makeSnapshot, makeTaskBranch } from '@/test/fixtures.ts'
import { renderWithClient } from '@/test/renderWithClient.tsx'
import { checkoutBranch } from './checkoutApi.ts'
import { IssuesPanel } from './IssuesPanel.tsx'
import { startWork } from './startWorkApi.ts'

// How an issue's row offers to switch to its branch, and what the switch says
// and holds while it runs.

vi.mock('./checkoutApi.ts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./checkoutApi.ts')>()),
  checkoutBranch: vi.fn(() => Promise.resolve(makeBranch())),
}))
const mockCheckout = vi.mocked(checkoutBranch)

vi.mock('./startWorkApi.ts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./startWorkApi.ts')>()),
  startWork: vi.fn(() => Promise.resolve(makeBranch())),
}))
const mockStartWork = vi.mocked(startWork)

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

// withIssues puts a stream frame listing PROJ-1 and PROJ-2 on screen.
function withIssues() {
  const issues = [tokenLeak, setupDocs]
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: { total: issues.length, next: '', unavailable: [], issues },
    }),
  })
}

test('checks out an in-flight branch from the list without opening the detail', async () => {
  // Arrange
  // PROJ-1's branch exists but is not the checked-out one, so the row offers to
  // switch to it directly.
  const user = userEvent.setup()
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false })],
    },
  }))
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('fix/PROJ-1-leak')
})

test('the row names its switch by what it is doing while it runs', async () => {
  // Arrange
  const user = userEvent.setup()
  mockCheckout.mockReturnValueOnce(new Promise(() => {}))
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false })],
    },
  }))
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  expect(screen.getByRole('button', { name: /switching/i }).textContent).toBe('Switching…')
})

test('one switch goes at a time: while a row switches, no other row sends its own', async () => {
  // Arrange
  const user = userEvent.setup()
  mockCheckout.mockReturnValueOnce(new Promise(() => {}))
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [
        makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false }),
        makeTaskBranch({ name: 'docs/PROJ-2-setup', issue_key: 'PROJ-2', current: false }),
      ],
    },
  }))
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-2' }))

  // Assert
  expect(mockCheckout.mock.calls).toEqual([['fix/PROJ-1-leak']])
  expect(
    screen.getByRole('button', { name: 'Switch branch for PROJ-2' }).getAttribute('aria-disabled'),
  ).toBe('true')
})

test('does not offer to switch to the branch already on HEAD', () => {
  // Arrange
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: true })],
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
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false })],
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
        makeTaskBranch({ name: 'feat/PROJ-1-redo', issue_key: 'PROJ-1', current: true }),
        makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false }),
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
        makeTaskBranch({ name: 'feat/PROJ-1-redo', issue_key: 'PROJ-1', current: false }),
        makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false }),
      ],
    },
  }))
  renderWithClient(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  expect(mockCheckout).toHaveBeenCalledWith('feat/PROJ-1-redo')
})

// withSetupDocsOpen lists PROJ-1, whose branch is not on HEAD, beside PROJ-2,
// not started, and opens PROJ-2's story, which offers to start work on it.
async function withSetupDocsOpen(user: ReturnType<typeof userEvent.setup>) {
  withIssues()
  useSnapshotStore.setState((state) => ({
    snapshot: state.snapshot && {
      ...state.snapshot,
      branches: [makeTaskBranch({ name: 'fix/PROJ-1-leak', issue_key: 'PROJ-1', current: false })],
    },
  }))
  renderWithClient(<IssuesPanel />)
  await user.click(screen.getByRole('button', { name: /write the setup docs/i }))
}

test("while a row switches, the story's Start work moves nothing", async () => {
  // Arrange
  // Starting work here makes a branch and switches to it: two moves of the
  // one working tree would race, and the last to land, not the last said,
  // would stand.
  const user = userEvent.setup()
  mockCheckout.mockReturnValueOnce(new Promise(() => {}))
  await withSetupDocsOpen(user)
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Assert
  expect(mockStartWork).not.toHaveBeenCalled()
  expect(screen.getByRole('button', { name: 'Start work' }).getAttribute('aria-disabled')).toBe(
    'true',
  )
})

test('while the story starts work, no row sends its switch', async () => {
  // Arrange
  const user = userEvent.setup()
  mockStartWork.mockReturnValueOnce(new Promise(() => {}))
  await withSetupDocsOpen(user)
  await user.click(screen.getByRole('button', { name: 'Start work' }))

  // Act
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Assert
  expect(mockCheckout).not.toHaveBeenCalled()
  expect(
    screen.getByRole('button', { name: 'Switch branch for PROJ-1' }).getAttribute('aria-disabled'),
  ).toBe('true')
})

test('while a row switches, branching from what you have moves nothing', async () => {
  // Arrange
  const user = userEvent.setup()
  mockStartWork.mockRejectedValueOnce({
    code: 'fetch_failed',
    detail: 'the base remote could not be fetched, so nothing was made for PROJ-2',
  })
  mockCheckout.mockReturnValueOnce(new Promise(() => {}))
  await withSetupDocsOpen(user)
  await user.click(screen.getByRole('button', { name: 'Start work' }))
  const branchFromHere = await screen.findByRole('button', { name: 'Branch from what you have' })
  await user.click(screen.getByRole('button', { name: 'Switch branch for PROJ-1' }))

  // Act
  await user.click(branchFromHere)

  // Assert
  expect(mockStartWork.mock.calls).toEqual([['PROJ-2', true]])
})
