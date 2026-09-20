import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { WorkStory } from './WorkStory.tsx'

// makeSnapshot's branch is fix/PROJ-1; a branches entry marks PROJ-1 as the one
// checked out, so PROJ-1 owns the current work.
const onHead = [{ name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true }]

test('lays out the in-flight stages for the issue that owns the branch', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      changes: {
        changes: [
          { path: 'a.go', kind: 'modified', staged: false, has_unstaged: true, conflicted: false },
        ],
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText('Branch')).toBeTruthy()
  expect(screen.getByText('Pull request')).toBeTruthy()
  expect(screen.getByText(/1 file\(s\) to commit/)).toBeTruthy()
})

test('reads a committed clean tree and a green pull request', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      branch: {
        name: 'fix/PROJ-1',
        detached: false,
        head: 'h1h2h3h',
        upstream: 'origin/fix/PROJ-1',
        ahead: 1,
        behind: 0,
        base: 'origin/main',
        commits: [{ hash: 'h1h2h3h4', subject: 'do the work' }],
      },
      changes: { changes: [] },
      review: {
        found: true,
        pull: {
          number: 128,
          url: 'https://x/128',
          title: 'the change',
          draft: false,
          approvals: 1,
          changes_requested: false,
          mergeable: 'clean',
        },
        ci: { state: 'passed', total: 1, done: 1, failed: 0, checks: [] },
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText(/working tree clean/i)).toBeTruthy()
  expect(screen.getByText(/#128 · CI passed/i)).toBeTruthy()
})

test('reads a fresh branch as nothing-committed and a pull request with no CI', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: onHead,
      review: {
        found: true,
        pull: {
          number: 42,
          url: 'https://x/42',
          title: 'the change',
          draft: false,
          approvals: 0,
          changes_requested: false,
          mergeable: 'unknown',
        },
      },
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.getByText(/nothing committed yet/i)).toBeTruthy()
  expect(screen.getByText('#42')).toBeTruthy()
})

test('shows a not-started story for an issue that does not own the branch', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  render(<WorkStory issueKey="PROJ-999" />)

  // Assert
  expect(screen.getByText(/not in progress/i)).toBeTruthy()
  expect(screen.getByText(/no branch for this issue yet/i)).toBeTruthy()
})

test('shows an in-progress-elsewhere story for an issue on a branch not checked out', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
      branches: [
        { name: 'fix/PROJ-1', issue_key: 'PROJ-1', current: true },
        { name: 'feat/PROJ-2-metrics', issue_key: 'PROJ-2', current: false },
      ],
    }),
  })

  // Act
  render(<WorkStory issueKey="PROJ-2" />)

  // Assert
  expect(screen.getByText(/in progress on feat\/PROJ-2-metrics/i)).toBeTruthy()
  // Both the changes and pull-request stages defer to the checked-out branch.
  expect(screen.getAllByText(/shown for the checked-out branch/i)).toHaveLength(2)
})

test('jumps to a stage section when it is clicked', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory issueKey="PROJ-1" />)

  // Act
  await user.click(screen.getByRole('button', { name: /pull request/i }))

  // Assert
  expect(useUiStore.getState().section).toBe('review')
})

test('renders nothing before a snapshot arrives', () => {
  // Act
  render(<WorkStory issueKey="PROJ-1" />)

  // Assert
  expect(screen.queryByText('Branch')).toBeNull()
})
