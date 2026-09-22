import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { checkoutBranch } from './checkoutApi.ts'
import { IssuesPanel } from './IssuesPanel.tsx'

vi.mock('./checkoutApi.ts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./checkoutApi.ts')>()),
  checkoutBranch: vi.fn(() => Promise.resolve()),
}))
const mockCheckout = vi.mocked(checkoutBranch)

function withIssues() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      issues: {
        total: 2,
        start_at: 0,
        issues: [
          {
            key: 'PROJ-1',
            summary: 'Fix the token leak',
            status: 'In Progress',
            status_category: 'indeterminate',
            type: 'Bug',
            priority: 'High',
          },
          {
            key: 'PROJ-2',
            summary: 'Write the setup docs',
            status: 'To Do',
            status_category: 'new',
            type: 'Task',
          },
        ],
      },
    }),
  })
}

test('lists the issues in the view', () => {
  // Arrange
  withIssues()

  // Act
  render(<IssuesPanel />)

  // Assert
  expect(screen.getByText('Fix the token leak')).toBeTruthy()
  expect(screen.getByText('Write the setup docs')).toBeTruthy()
})

test('shows an issue detail when it is selected', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  render(<IssuesPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: /fix the token leak/i }))

  // Assert
  expect(screen.getByRole('heading', { level: 2, name: /fix the token leak/i })).toBeTruthy()
})

test('shows the work story for the selected issue', async () => {
  // Arrange
  const user = userEvent.setup()
  withIssues()
  render(<IssuesPanel />)

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
  render(<IssuesPanel />)

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
  render(<IssuesPanel />)

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
  render(<IssuesPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: /check out PROJ-1/i })).toBeNull()
})

test('prompts to connect before any snapshot arrives', () => {
  // Act
  render(<IssuesPanel />)

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
  render(<IssuesPanel />)

  // Assert
  expect(screen.getByText(/no issues match/i)).toBeTruthy()
})
