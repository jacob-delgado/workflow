import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { WorkStory } from './WorkStory.tsx'

// makeSnapshot's branch is fix/PROJ-1, so PROJ-1 owns the current work.
test('lays out the in-flight stages for the issue that owns the branch', () => {
  // Arrange
  useSnapshotStore.setState({
    status: 'live',
    snapshot: makeSnapshot({
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

test('shows a not-started story for an issue that does not own the branch', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })

  // Act
  render(<WorkStory issueKey="PROJ-999" />)

  // Assert
  expect(screen.getByText(/not in progress/i)).toBeTruthy()
  expect(screen.getByText(/no branch for this issue yet/i)).toBeTruthy()
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
