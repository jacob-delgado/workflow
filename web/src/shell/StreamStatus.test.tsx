import { render, screen } from '@testing-library/react'
import { useSnapshotStore } from '@/api/snapshot.ts'
import { StreamStatus } from './StreamStatus.tsx'

test('reflects the live stream state', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'live' })

  // Act
  render(<StreamStatus />)

  // Assert
  expect(screen.getByRole('status').textContent).toContain('Live')
})
