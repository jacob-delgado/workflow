import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'
import { commitChanges } from './commitApi.ts'
import { pushBranch } from './pushApi.ts'

vi.mock('./commitApi.ts', () => ({ commitChanges: vi.fn(() => Promise.resolve()) }))
const mockCommit = vi.mocked(commitChanges)

vi.mock('./pushApi.ts', () => ({ pushBranch: vi.fn(() => Promise.resolve()) }))
const mockPush = vi.mocked(pushBranch)

// The commit form reads the configuration for the team's types; with none, it
// offers the built-in set and defaults to fix, as these tests expect.
vi.mock('@/features/settings/configApi.ts', () => ({ useConfig: () => ({ data: undefined }) }))

// pushable is a snapshot whose branch has commits the remote does not have.
function pushable() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'fix/PROJ-1',
        detached: false,
        head: 'abc1234',
        upstream: '',
        ahead: 0,
        behind: 0,
        base: 'origin/main',
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens' }],
      },
    }),
  })
}

// staged is a snapshot whose working tree has one staged and one unstaged change.
function staged() {
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: true, has_unstaged: false, conflicted: false },
          { path: 'b.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })
}

test('shows the current branch, its commits, and its changes', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'feat/web',
        detached: false,
        head: 'deadbee',
        upstream: 'origin/feat/web',
        ahead: 3,
        behind: 1,
        base: 'origin/main',
        commits: [{ hash: 'deadbeef1', subject: 'feat: build the shell' }],
      },
      changes: {
        changes: [
          {
            path: 'web/src/App.tsx',
            kind: 'modified',
            staged: true,
            has_unstaged: false,
            conflicted: false,
          },
        ],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('heading', { name: 'feat/web' })).toBeTruthy()
  expect(screen.getByText(/build the shell/)).toBeTruthy()
  expect(screen.getByText('web/src/App.tsx')).toBeTruthy()
})

test('offers a commit form when a change is staged', () => {
  // Arrange
  staged()

  // Act
  render(<BranchPanel />)

  // Assert
  const commit = screen.getByRole('button', { name: /commit staged changes/i })
  expect(commit.hasAttribute('disabled')).toBe(false)
  expect(screen.queryByText('Nothing staged yet — stage a file above.')).toBeNull()
})

test('keeps the commit form when nothing is staged, saying what it waits for', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      changes: {
        changes: [
          { path: 'b.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  const commit = screen.getByRole('button', { name: /commit staged changes/i })
  expect(commit.hasAttribute('disabled')).toBe(true)
  expect(screen.getByText('Nothing staged yet — stage a file above.')).toBeTruthy()
})

test('commits the staged changes when the form is submitted', async () => {
  // Arrange
  mockCommit.mockResolvedValueOnce()
  const user = userEvent.setup()
  staged()
  render(<BranchPanel />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(mockCommit).toHaveBeenCalledWith(
    expect.objectContaining({ type: 'fix', subject: 'redact tokens' }),
  )
})

test('shows the reason when a commit is refused', async () => {
  // Arrange
  mockCommit.mockRejectedValueOnce({
    code: 'unprocessable',
    detail: 'nothing is staged to commit',
  })
  const user = userEvent.setup()
  staged()
  render(<BranchPanel />)
  await user.type(screen.getByLabelText('Subject'), 'redact tokens')

  // Act
  await user.click(screen.getByRole('button', { name: /commit staged changes/i }))

  // Assert
  expect(await screen.findByText(/nothing is staged/i)).toBeTruthy()
})

test('offers to push a branch with unpushed commits', () => {
  // Arrange
  pushable()

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('button', { name: 'Push branch' })).toBeTruthy()
})

test('offers no push for a fully published branch', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'fix/PROJ-1',
        detached: false,
        head: 'abc1234',
        upstream: 'origin/fix/PROJ-1',
        ahead: 0,
        behind: 0,
        base: 'origin/main',
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens' }],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Push branch' })).toBeNull()
})

test('offers no push in a detached HEAD', () => {
  // Arrange
  // Commits present, but not on a branch — so there is nothing to push.
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: '',
        detached: true,
        head: 'abcdef1234',
        upstream: '',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [{ hash: 'c0ffee1', subject: 'fix: redact tokens' }],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.queryByRole('button', { name: 'Push branch' })).toBeNull()
})

test('pushes only after the confirm step', async () => {
  // Arrange
  mockPush.mockResolvedValueOnce()
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act: ask, then confirm
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  expect(mockPush).toHaveBeenCalledTimes(1)
})

test('does not push if the confirm is canceled', async () => {
  // Arrange
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Cancel' }))

  // Assert
  expect(mockPush).not.toHaveBeenCalled()
})

test('shows the reason when a push fails', async () => {
  // Arrange
  mockPush.mockRejectedValueOnce({ code: 'unprocessable', detail: 'the push failed: rejected' })
  const user = userEvent.setup()
  pushable()
  render(<BranchPanel />)

  // Act
  await user.click(screen.getByRole('button', { name: 'Push branch' }))
  await user.click(screen.getByRole('button', { name: 'Push' }))

  // Assert
  expect(await screen.findByText(/the push failed/i)).toBeTruthy()
})

test('invites connecting before the first snapshot arrives', () => {
  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText(/connecting/i)).toBeTruthy()
})

test('shows placeholders for an unpublished branch with a clean tree', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: 'wip',
        detached: false,
        head: 'aaa',
        upstream: '',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [],
      },
      changes: { changes: [] },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText('none')).toBeTruthy()
  expect(screen.getByText(/no commits yet/i)).toBeTruthy()
  expect(screen.getByText(/clean/i)).toBeTruthy()
})

test('shows a detached HEAD rather than calling it not a repository', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: '',
        detached: true,
        head: 'abcdef1234',
        upstream: '',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByRole('heading', { name: /detached head at abcdef1/i })).toBeTruthy()
})

test('says so when the workspace is not a Git repository', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branch: {
        name: '',
        detached: false,
        head: '',
        upstream: '',
        ahead: 0,
        behind: 0,
        base: '',
        commits: [],
      },
    }),
  })

  // Act
  render(<BranchPanel />)

  // Assert
  expect(screen.getByText(/not a git repository/i)).toBeTruthy()
})
