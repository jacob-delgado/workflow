import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { BranchPanel } from './BranchPanel.tsx'

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
