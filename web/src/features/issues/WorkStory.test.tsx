import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { makeSnapshot } from '@/test/fixtures.ts'
import { useUiStore } from '@/shell/uiStore.ts'
import { WorkStory } from './WorkStory.tsx'

test('lays out the stages of the loop from the streamed state', () => {
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
  render(<WorkStory />)

  // Assert
  expect(screen.getByText('Branch')).toBeTruthy()
  expect(screen.getByText('Changes')).toBeTruthy()
  expect(screen.getByText('Pull request')).toBeTruthy()
  expect(screen.getByText('Announce')).toBeTruthy()
  expect(screen.getByText(/1 file\(s\) to commit/)).toBeTruthy()
})

test('jumps to a stage section when it is clicked', async () => {
  // Arrange
  const user = userEvent.setup()
  useSnapshotStore.setState({ status: 'live', snapshot: makeSnapshot() })
  render(<WorkStory />)

  // Act
  await user.click(screen.getByRole('button', { name: /pull request/i }))

  // Assert
  expect(useUiStore.getState().section).toBe('review')
})

test('renders nothing before a snapshot arrives', () => {
  // Act
  render(<WorkStory />)

  // Assert
  expect(screen.queryByText('Branch')).toBeNull()
})
