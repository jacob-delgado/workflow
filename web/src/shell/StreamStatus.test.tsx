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

test('tells a dropped stream apart from one whose frames this page cannot read', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'stale', reason: 'a frame did not match; reload the page' })

  // Act
  render(<StreamStatus />)

  // Assert
  const region = screen.getByRole('status')
  expect(region.textContent).toContain('Out of date')
  expect(region.textContent).toContain('a frame did not match; reload the page')
})

test('says a dropped stream is reconnecting', () => {
  // Arrange
  useSnapshotStore.setState({ status: 'reconnecting' })

  // Act
  render(<StreamStatus />)

  // Assert
  expect(screen.getByRole('status').textContent).toContain('Reconnecting')
})
